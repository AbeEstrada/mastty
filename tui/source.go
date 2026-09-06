package tui

import (
	"context"
	"fmt"

	"github.com/mattn/go-mastodon"
)

const (
	titleHome          = "Home"
	titleNotifications = "Notifications"
)

// Source knows how to fetch the items of one timeline. Sources own their
// pagination cursor: go-mastodon overwrites the Pagination it is given with the
// next page from the Link header, which is the only way to page through
// bookmarks and favourites.
type Source interface {
	Title() string
	// Fetch loads the first page and resets the cursor.
	Fetch(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error)
	// LoadMore loads the next older page, or nil when there is none.
	LoadMore(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error)
	// Newer loads items newer than sinceID. ok is false when the source cannot
	// do that, in which case callers refetch from the top instead.
	Newer(ctx context.Context, c *mastodon.Client, sinceID mastodon.ID) (items []TimelineItem, ok bool, err error)
}

// threadLayout is implemented by sources whose rows should be indented.
type threadLayout interface {
	Layout() (focal mastodon.ID, depths map[mastodon.ID]int)
}

type statusPager func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error)

// statusSource pages through any endpoint that returns statuses.
type statusSource struct {
	title  string
	page   statusPager
	prefix []TimelineItem // items shown before the first page, e.g. a profile
	newer  bool           // whether the endpoint supports since_id
	pg     mastodon.Pagination
	more   bool
}

func (s *statusSource) Title() string { return s.title }

func (s *statusSource) Fetch(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	s.pg = mastodon.Pagination{Limit: pageLimit}
	statuses, err := s.page(ctx, c, &s.pg)
	if err != nil {
		return nil, err
	}
	s.more = s.pg.MaxID != ""
	items := make([]TimelineItem, 0, len(s.prefix)+len(statuses))
	items = append(items, s.prefix...)
	return append(items, statusItems(statuses)...), nil
}

func (s *statusSource) LoadMore(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	if !s.more {
		return nil, nil
	}
	statuses, err := s.page(ctx, c, &s.pg)
	if err != nil {
		return nil, err
	}
	s.more = s.pg.MaxID != "" && len(statuses) > 0
	return statusItems(statuses), nil
}

func (s *statusSource) Newer(ctx context.Context, c *mastodon.Client, sinceID mastodon.ID) ([]TimelineItem, bool, error) {
	if !s.newer {
		return nil, false, nil
	}
	statuses, err := s.page(ctx, c, &mastodon.Pagination{SinceID: sinceID, Limit: reloadLimit})
	if err != nil {
		return nil, true, err
	}
	return statusItems(statuses), true, nil
}

func homeSource() Source {
	return &statusSource{title: titleHome, newer: true, page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetTimelineHome(ctx, pg)
	}}
}

func publicSource(local bool) Source {
	title := "Federated"
	if local {
		title = "Local"
	}
	return &statusSource{title: title, newer: true, page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetTimelinePublic(ctx, local, pg)
	}}
}

func hashtagSource(tag string) Source {
	return &statusSource{title: "#" + tag, newer: true, page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetTimelineHashtag(ctx, tag, false, pg)
	}}
}

func listSource(list *mastodon.List) Source {
	return &statusSource{title: "List: " + list.Title, newer: true, page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetTimelineList(ctx, list.ID, pg)
	}}
}

func bookmarksSource() Source {
	return &statusSource{title: "Bookmarks", page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetBookmarks(ctx, pg)
	}}
}

func favouritesSource() Source {
	return &statusSource{title: "Favourites", page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
		return c.GetFavourites(ctx, pg)
	}}
}

func accountSource(account *mastodon.Account) Source {
	return &statusSource{
		title:  "@" + account.Acct,
		newer:  true,
		prefix: []TimelineItem{AccountItem{Account: account}},
		page: func(ctx context.Context, c *mastodon.Client, pg *mastodon.Pagination) ([]*mastodon.Status, error) {
			return c.GetAccountStatuses(ctx, account.ID, pg)
		},
	}
}

// threadSource loads a whole conversation around one status.
type threadSource struct {
	status *mastodon.Status
	depths map[mastodon.ID]int
}

func newThreadSource(status *mastodon.Status) *threadSource {
	return &threadSource{status: status}
}

func (s *threadSource) Title() string { return "Thread" }

func (s *threadSource) Fetch(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	thread, err := c.GetStatusContext(ctx, s.status.ID)
	if err != nil {
		return nil, err
	}
	items := make([]TimelineItem, 0, len(thread.Ancestors)+1+len(thread.Descendants))
	items = append(items, statusItems(thread.Ancestors)...)
	items = append(items, StatusItem{Status: s.status})
	items = append(items, statusItems(thread.Descendants)...)
	s.depths = threadDepths(thread, s.status)
	return items, nil
}

func (s *threadSource) LoadMore(context.Context, *mastodon.Client) ([]TimelineItem, error) {
	return nil, nil
}

func (s *threadSource) Newer(context.Context, *mastodon.Client, mastodon.ID) ([]TimelineItem, bool, error) {
	return nil, false, nil
}

func (s *threadSource) Layout() (mastodon.ID, map[mastodon.ID]int) {
	return s.status.ID, s.depths
}

// threadDepths computes the reply depth of every status in a thread: the
// ancestor chain counts up from the root, and each descendant sits one level
// below its parent.
func threadDepths(thread *mastodon.Context, focal *mastodon.Status) map[mastodon.ID]int {
	depths := make(map[mastodon.ID]int, len(thread.Ancestors)+1+len(thread.Descendants))
	for i, s := range thread.Ancestors {
		depths[s.ID] = i
	}
	focalDepth := len(thread.Ancestors)
	depths[focal.ID] = focalDepth
	for _, s := range thread.Descendants {
		depth := focalDepth + 1
		if parent, ok := depths[idField(s.InReplyToID)]; ok {
			depth = parent + 1
		}
		depths[s.ID] = depth
	}
	return depths
}

// notificationsSource pages through the user's notifications.
type notificationsSource struct {
	pg   mastodon.Pagination
	more bool
}

func (s *notificationsSource) Title() string { return titleNotifications }

func (s *notificationsSource) Fetch(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	s.pg = mastodon.Pagination{Limit: pageLimit}
	notifications, err := c.GetNotifications(ctx, &s.pg)
	if err != nil {
		return nil, err
	}
	s.more = s.pg.MaxID != ""
	return notificationItems(notifications), nil
}

func (s *notificationsSource) LoadMore(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	if !s.more {
		return nil, nil
	}
	notifications, err := c.GetNotifications(ctx, &s.pg)
	if err != nil {
		return nil, err
	}
	s.more = s.pg.MaxID != "" && len(notifications) > 0
	return notificationItems(notifications), nil
}

func (s *notificationsSource) Newer(ctx context.Context, c *mastodon.Client, sinceID mastodon.ID) ([]TimelineItem, bool, error) {
	notifications, err := c.GetNotifications(ctx, &mastodon.Pagination{SinceID: sinceID, Limit: reloadLimit})
	if err != nil {
		return nil, true, err
	}
	return notificationItems(notifications), true, nil
}

// searchSource shows the accounts and statuses matching a query.
type searchSource struct {
	query string
}

func (s *searchSource) Title() string { return fmt.Sprintf("Search: %s", s.query) }

func (s *searchSource) Fetch(ctx context.Context, c *mastodon.Client) ([]TimelineItem, error) {
	results, err := c.Search(ctx, s.query, true)
	if err != nil {
		return nil, err
	}
	items := make([]TimelineItem, 0, len(results.Accounts)+len(results.Statuses))
	items = append(items, accountItems(results.Accounts)...)
	return append(items, statusItems(results.Statuses)...), nil
}

func (s *searchSource) LoadMore(context.Context, *mastodon.Client) ([]TimelineItem, error) {
	return nil, nil
}

func (s *searchSource) Newer(context.Context, *mastodon.Client, mastodon.ID) ([]TimelineItem, bool, error) {
	return nil, false, nil
}
