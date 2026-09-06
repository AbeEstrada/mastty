package tui

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

const (
	streamInitialBackoff = time.Second
	streamMaxBackoff     = time.Minute
	reloadLimit          = 40
	pageLimit            = 20
)

type HomeView struct {
	app              *App
	timeline         *TimelineView
	statusView       *StatusView
	accountView      *AccountView
	notificationView *NotificationView
	linksView        *LinksView
	picker           *PickerView
	vote             *pollVote
	relationships    map[mastodon.ID]*mastodon.Relationship
	focusedView      int
	showingLinks     bool
	lastSelectedID   mastodon.ID

	// roots holds every root timeline fetched so far, by title, so switching
	// between them is instant. Streaming keeps the home and notification roots
	// fresh even while another root is shown.
	roots                  map[string]*Timeline
	lastMarkedNotification mastodon.ID

	// Layout of the last Draw in screen coordinates, for mouse hit testing.
	bodyLeft, bodyTop, bodyWidth, bodyHeight, split int

	streamCancel context.CancelFunc
	// streamBroken is set when the stream dropped; the next event triggers a
	// backfill of whatever was missed.
	streamBroken bool
}

func CreateHomeView() *HomeView {
	statusView := CreateStatusView()
	accountView := CreateAccountView()
	v := &HomeView{
		statusView:       statusView,
		accountView:      accountView,
		notificationView: CreateNotificationView(statusView, accountView),
		linksView:        CreateLinksView(),
		roots:            make(map[string]*Timeline),
		relationships:    make(map[mastodon.ID]*mastodon.Relationship),
	}
	accountView.Relationships = v.relationships
	timelineView := CreateTimelineView()
	timelineView.onLoadMore = v.loadMore
	v.timeline = timelineView

	return v
}

func (v *HomeView) SetApp(app *App) {
	v.app = app
	v.timeline.SetApp(app)
	v.statusView.SetApp(app)
	v.accountView.SetApp(app)
}

func (v *HomeView) OnActivate() {
	v.showRoot(homeSource())
	v.startStreaming()
}

// Close stops the streaming connection.
func (v *HomeView) Close() {
	if v.streamCancel != nil {
		v.streamCancel()
		v.streamCancel = nil
	}
}

// FocusLabel names the pane that receives navigation keys.
func (v *HomeView) FocusLabel() string {
	switch {
	case v.focusedView == 0:
		return "timeline"
	case v.showingLinks:
		return "links"
	default:
		return "detail"
	}
}

// selectedStatus returns the status behind the selection: the status itself,
// or the one a notification refers to.
func (v *HomeView) selectedStatus() *mastodon.Status {
	switch item := v.timeline.SelectedItem().(type) {
	case StatusItem:
		return item.Status
	case NotificationItem:
		if item.Notification != nil {
			return item.Status
		}
	}
	return nil
}

// showRoot replaces the navigation stack with a root timeline, reusing an
// already fetched one when available.
func (v *HomeView) showRoot(source Source) {
	title := source.Title()
	if root := v.timeline.Root(); root != nil && v.timeline.Depth() == 1 && root.Title() == title {
		return
	}
	t, cached := v.roots[title]
	if !cached {
		t = v.timeline.NewTimeline(source)
		v.roots[title] = t
	}
	v.showingLinks = false
	v.focusedView = 0
	v.timeline.SetRoot(t)
	switch {
	case !cached:
		v.fetch(t, nil)
	case title == titleNotifications:
		v.markNotificationsRead(t)
	}
}

// push shows a new timeline on top of the current one and loads it.
func (v *HomeView) push(source Source, selected TimelineItem) {
	t := v.timeline.NewTimeline(source)
	v.showingLinks = false
	v.timeline.Push(t)
	v.fetch(t, selected)
}

func (v *HomeView) pop() bool {
	if !v.timeline.Pop() {
		return false
	}
	v.showingLinks = false
	return true
}

// fetch loads the first page of a timeline and installs it.
func (v *HomeView) fetch(t *Timeline, selected TimelineItem) {
	client := v.app.client
	load(v.app, func(ctx context.Context) ([]TimelineItem, error) {
		return t.Source.Fetch(ctx, client)
	}, func(items []TimelineItem) {
		t.SetItems(items, selected)
		if v.timeline.Current() == t {
			v.timeline.afterChange(t)
		}
		if t.Title() == titleNotifications {
			v.markNotificationsRead(t)
		}
	})
}

// reload fetches newer items for the current timeline, or refetches it from
// the top when the source cannot page forward.
func (v *HomeView) reload() {
	v.reloadTimeline(v.timeline.Current())
}

func (v *HomeView) reloadTimeline(t *Timeline) {
	if t == nil {
		return
	}
	client := v.app.client
	sinceID, hasSince := t.NewestID()

	type result struct {
		items []TimelineItem
		newer bool
	}
	load(v.app, func(ctx context.Context) (result, error) {
		if hasSince {
			items, ok, err := t.Source.Newer(ctx, client, sinceID)
			if ok {
				return result{items: items, newer: true}, err
			}
		}
		items, err := t.Source.Fetch(ctx, client)
		return result{items: items}, err
	}, func(r result) {
		if r.newer {
			t.Prepend(r.items)
		} else {
			t.SetItems(r.items, nil)
			if v.timeline.Current() == t {
				v.timeline.afterChange(t)
			}
		}
		if t.Title() == titleNotifications && len(r.items) > 0 {
			v.markNotificationsRead(t)
		}
	})
}

// loadMore fetches the next older page of the current timeline.
func (v *HomeView) loadMore() {
	t := v.timeline.Current()
	if t == nil || t.exhausted {
		return
	}
	client := v.app.client
	load(v.app, func(ctx context.Context) ([]TimelineItem, error) {
		return t.Source.LoadMore(ctx, client)
	}, func(items []TimelineItem) {
		if len(items) == 0 {
			t.exhausted = true
			return
		}
		t.Append(items)
	})
}

func (v *HomeView) openThread() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	if original.ID == "" {
		return
	}
	v.push(newThreadSource(original), StatusItem{Status: original})
}

// openProfile pushes the timeline of the selected item's account, or the
// user's own when own is set.
func (v *HomeView) openProfile(own bool) {
	account := v.selectedAccount()
	if own {
		account = v.app.currentAccount
	}
	if account == nil {
		return
	}
	if current := v.timeline.Current(); current != nil && current.Title() == "@"+account.Acct {
		return
	}
	v.push(accountSource(account), nil)
	v.fetchRelationship(account.ID)
}

// markNotificationsRead tells the server the newest notification has been
// seen and clears the badge.
func (v *HomeView) markNotificationsRead(t *Timeline) {
	newest, ok := t.NewestID()
	if !ok || newest == v.lastMarkedNotification {
		return
	}
	v.lastMarkedNotification = newest
	v.app.header.SetBadge(0)

	client := v.app.customClient
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		if err := client.SetNotificationsMarker(ctx, newest); err != nil {
			log.Printf("Failed to set notifications marker: %v", err)
			return
		}
		v.app.fetchUnreadNotifications(client)
	}()
}

func (v *HomeView) promptHashtag() {
	v.app.PromptInput("Hashtag: #", "", func(text string) {
		tag := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "#"))
		if tag != "" {
			v.push(hashtagSource(tag), nil)
		}
	})
}

func (v *HomeView) promptSearch() {
	v.app.PromptInput("Search: ", "", func(text string) {
		query := strings.TrimSpace(text)
		if query == "" {
			return
		}
		if strings.HasPrefix(query, "#") && !strings.ContainsAny(query, " \t") {
			v.push(hashtagSource(strings.TrimPrefix(query, "#")), nil)
			return
		}
		v.push(&searchSource{query: query}, nil)
	})
}

func (v *HomeView) pickList() {
	client := v.app.client
	load(v.app, func(ctx context.Context) ([]*mastodon.List, error) {
		return client.GetLists(ctx)
	}, func(lists []*mastodon.List) {
		if len(lists) == 0 {
			v.app.Flash("You have no lists")
			return
		}
		titles := make([]string, len(lists))
		for i, list := range lists {
			titles[i] = list.Title
		}
		v.picker = NewPicker("Lists", titles, func(index int) {
			v.push(listSource(lists[index]), nil)
		})
	})
}

func (v *HomeView) startStreaming() {
	if v.streamCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.streamCancel = cancel
	go v.runStream(ctx, v.app.client)
}

// runStream keeps a user stream open, backing off exponentially between
// failed connections. go-mastodon reconnects on its own without any delay, so
// each connection is cancelled on error and reopened here instead.
func (v *HomeView) runStream(ctx context.Context, client *mastodon.Client) {
	backoff := streamInitialBackoff
	for ctx.Err() == nil {
		received := v.streamOnce(ctx, client)
		if ctx.Err() != nil {
			return
		}
		if received {
			backoff = streamInitialBackoff
		}
		v.app.Sync(func() { v.streamBroken = true })

		log.Printf("streaming: reconnecting in %s", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < streamMaxBackoff {
			backoff *= 2
		}
	}
}

// streamOnce consumes one streaming connection until it fails and reports
// whether any event arrived on it.
func (v *HomeView) streamOnce(ctx context.Context, client *mastodon.Client) bool {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, err := client.StreamingUser(connCtx)
	if err != nil {
		log.Printf("streaming: %v", err)
		return false
	}

	received := false
	for event := range events {
		if connCtx.Err() != nil {
			// Drain until go-mastodon notices the cancellation and closes the channel.
			continue
		}
		if e, ok := event.(*mastodon.ErrorEvent); ok {
			log.Printf("streaming error: %v", e.Error())
			cancel()
			continue
		}
		received = true
		v.app.Sync(func() { v.handleStreamingEvent(event) })
	}
	return received
}

// forEachTimeline visits every root and every timeline on the stack once.
func (v *HomeView) forEachTimeline(fn func(*Timeline)) {
	seen := make(map[*Timeline]bool, len(v.roots))
	for _, t := range v.roots {
		seen[t] = true
		fn(t)
	}
	for _, t := range v.timeline.Stack() {
		if !seen[t] {
			fn(t)
		}
	}
}

// replaceStatusEverywhere applies an updated status to every timeline.
func (v *HomeView) replaceStatusEverywhere(status *mastodon.Status) {
	if status == nil {
		return
	}
	v.forEachTimeline(func(t *Timeline) { t.ReplaceStatus(status) })
}

func (v *HomeView) handleStreamingEvent(event mastodon.Event) {
	home := v.roots[titleHome]
	if v.streamBroken {
		v.streamBroken = false
		if home != nil && !v.app.IsLoading() {
			v.reloadTimeline(home)
		}
		// The backfill fetches everything newer than the top of the timeline,
		// including this status, so skip the direct insert to keep ordering.
		if _, ok := event.(*mastodon.UpdateEvent); ok {
			return
		}
	}

	switch e := event.(type) {
	case *mastodon.UpdateEvent:
		if home != nil {
			home.Prepend([]TimelineItem{StatusItem{Status: e.Status}})
		}

	case *mastodon.UpdateEditEvent:
		v.replaceStatusEverywhere(e.Status)

	case *mastodon.NotificationEvent:
		v.onNotification(e.Notification)

	case *mastodon.DeleteEvent:
		v.forEachTimeline(func(t *Timeline) { t.Delete(e.ID) })

	default:
		log.Printf("streaming: unhandled event %T", event)
	}
}

func (v *HomeView) onNotification(n *mastodon.Notification) {
	if n == nil {
		return
	}
	log.Printf("New Notification [%s] from @%s", n.Type, n.Account.Acct)
	if notifications := v.roots[titleNotifications]; notifications != nil {
		notifications.Prepend([]TimelineItem{NotificationItem{Notification: n}})
		if v.timeline.Root() == notifications {
			v.markNotificationsRead(notifications)
			return
		}
	}
	v.app.header.IncrementBadge()
	if v.app.config.Preferences.DesktopNotifications && n.Type == "mention" {
		v.app.vx.Notify("Tuit", fmt.Sprintf("@%s mentioned you", n.Account.Acct))
	}
}

func (v *HomeView) selectedStatusLinks() []LinkItem {
	status := v.selectedStatus()
	if status == nil {
		return nil
	}
	status = originalStatus(status)

	var items []LinkItem
	seen := map[string]bool{}
	for _, link := range utils.ExtractLinks(status.Content) {
		if seen[link.URL] {
			continue
		}
		seen[link.URL] = true
		label := link.URL
		if (link.Kind == utils.LinkKindMention || link.Kind == utils.LinkKindHashtag) && link.Text != "" {
			label = link.Text
		}
		items = append(items, LinkItem{Label: label, URL: link.URL})
	}
	if status.Card != nil && status.Card.URL != "" && !seen[status.Card.URL] {
		seen[status.Card.URL] = true
		label := status.Card.Title
		if label == "" {
			label = status.Card.URL
		}
		items = append(items, LinkItem{Label: label, URL: status.Card.URL})
	}
	for _, att := range status.MediaAttachments {
		if att.URL == "" || seen[att.URL] {
			continue
		}
		seen[att.URL] = true
		label := "[" + mediaLabel(att) + "]"
		if att.Description != "" {
			label += " " + att.Description
		}
		items = append(items, LinkItem{Label: label, URL: att.URL})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return utils.IsTagLink(items[j].URL) && !utils.IsTagLink(items[i].URL)
	})
	return items
}

// windowOrigin returns the absolute screen position of a window.
func windowOrigin(win vaxis.Window) (col, row int) {
	for w := &win; w != nil; w = w.Parent {
		col += w.Column
		row += w.Row
	}
	return col, row
}

func (v *HomeView) Draw(win vaxis.Window) {
	width, height := win.Size()
	v.bodyLeft, v.bodyTop = windowOrigin(win)
	v.bodyWidth, v.bodyHeight = width, height

	left, right := v.app.config.Preferences.Split()
	split := width * left / (left + right)
	if width >= 30 {
		split = max(12, min(split, width-14))
	}
	v.split = split
	v.app.split = split
	v.app.header.SetFocus(v.FocusLabel())

	timelineWin := win.New(0, 0, split, height)
	detailWidth := max(0, width-split-2)
	detailWin := win.New(split+2, 0, detailWidth, height)

	v.timeline.Draw(timelineWin, v.focusedView == 0)

	selectedItem := v.timeline.SelectedItem()
	isDetailFocused := v.focusedView == 1

	switch {
	case v.showingLinks:
		v.linksView.Draw(detailWin, isDetailFocused)
	case selectedItem != nil:
		if currentID := selectedItem.ID(); currentID != v.lastSelectedID {
			v.statusView.ResetScroll()
			v.lastSelectedID = currentID
		}
		switch item := selectedItem.(type) {
		case StatusItem:
			v.statusView.Draw(detailWin, isDetailFocused, item.Status)
		case AccountItem:
			v.accountView.Draw(detailWin, isDetailFocused, item.Account)
		case NotificationItem:
			v.notificationView.Draw(detailWin, isDetailFocused, item.Notification)
		}
	}

	separatorStyle := vaxis.Style{Foreground: vaxis.IndexColor(0)}
	for row := 0; row < height; row++ {
		win.SetCell(split, row, vaxis.Cell{
			Character: vaxis.Character{Grapheme: "│"},
			Style:     separatorStyle,
		})
	}

	if v.picker != nil {
		v.picker.Draw(win)
	}
}

// closeLinksIfMoved hides the links view when the timeline selection moved
// away from the status it was listing.
func (v *HomeView) closeLinksIfMoved(prev TimelineItem) {
	if !v.showingLinks {
		return
	}
	if cur := v.timeline.SelectedItem(); cur != nil && (prev == nil || cur.ID() != prev.ID()) {
		v.showingLinks = false
	}
}

func (v *HomeView) HandleMouse(m vaxis.Mouse) {
	if v.picker != nil || m.EventType == vaxis.EventRelease || m.EventType == vaxis.EventMotion {
		return
	}
	col, row := m.Col-v.bodyLeft, m.Row-v.bodyTop
	if row < 0 || row >= v.bodyHeight || col < 0 || col >= v.bodyWidth {
		return
	}
	inTimeline := col < v.split
	inDetail := col > v.split+1

	switch m.Button {
	case vaxis.MouseWheelUp, vaxis.MouseWheelDown:
		delta := 1
		if m.Button == vaxis.MouseWheelUp {
			delta = -1
		}
		switch {
		case inTimeline:
			prev := v.timeline.SelectedItem()
			v.timeline.Move(delta)
			v.closeLinksIfMoved(prev)
		case inDetail && v.showingLinks:
			v.linksView.Move(delta)
		case inDetail:
			v.statusView.Scroll(delta * 3)
		}
	case vaxis.MouseLeftButton:
		switch {
		case inTimeline:
			v.focusedView = 0
			prev := v.timeline.SelectedItem()
			if v.timeline.SelectRow(row) {
				v.closeLinksIfMoved(prev)
			}
		case inDetail:
			v.focusedView = 1
		}
	}
}

func (v *HomeView) toggleContentWarning() {
	if status := v.selectedStatus(); status != nil {
		v.statusView.ToggleExpanded(originalStatus(status).ID)
	}
}

// yank copies the selected link, or the selected status or profile URL, to
// the clipboard.
func (v *HomeView) yank() {
	var url string
	if v.showingLinks && v.focusedView == 1 {
		if link, ok := v.linksView.Selected(); ok {
			url = link.URL
		}
	} else if status := v.selectedStatus(); status != nil {
		url = originalStatus(status).URL
	} else if item, ok := v.timeline.SelectedItem().(AccountItem); ok {
		url = item.URL
	}
	if url == "" {
		return
	}
	v.app.vx.ClipboardPush(url)
	v.app.Flash("Copied " + url)
}

func (v *HomeView) openOriginal() {
	if status := v.selectedStatus(); status != nil {
		v.app.OpenURL(originalStatus(status).URL)
		return
	}
	if item, ok := v.timeline.SelectedItem().(AccountItem); ok {
		v.app.OpenURL(item.URL)
	}
}

func (v *HomeView) openLocal() {
	server := v.app.config.Auth.Server
	if status := v.selectedStatus(); status != nil {
		original := originalStatus(status)
		if original.URL != "" {
			v.app.OpenURL(fmt.Sprintf("%s/@%s/%s", server, original.Account.Acct, original.ID))
		}
		return
	}
	if item, ok := v.timeline.SelectedItem().(AccountItem); ok {
		v.app.OpenURL(fmt.Sprintf("%s/@%s", server, item.Acct))
	}
}

func (v *HomeView) openLink() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	url := ""
	if original.Card != nil {
		url = original.Card.URL
	}
	if url == "" {
		url = utils.ExtractFirstExternalURL(original.Content)
	}
	if url == "" {
		v.app.Flash("No link in this status")
		return
	}
	v.app.OpenURL(url)
}

func (v *HomeView) HandleKey(key vaxis.Key) {
	km := v.app.keys
	if v.picker != nil {
		if v.picker.HandleKey(key, km) {
			v.picker = nil
		}
		return
	}
	if v.vote != nil {
		v.handleVoteKey(key)
		return
	}
	if v.showingLinks {
		v.handleLinksKey(key)
		return
	}

	switch {
	case km.Is(key, ActFocusNext):
		v.focusedView = (v.focusedView + 1) % 2
	case km.Is(key, ActFocusTimeline):
		v.focusedView = 0
	case km.Is(key, ActFocusDetail):
		v.focusedView = 1
	case km.Is(key, ActReload):
		if !v.app.IsLoading() {
			v.reload()
		}
	case km.Is(key, ActThread):
		if !v.app.IsLoading() {
			v.openThread()
		}
	case km.Is(key, ActProfile):
		if !v.app.IsLoading() {
			v.openProfile(false)
		}
	case km.Is(key, ActOwnProfile):
		if !v.app.IsLoading() {
			v.openProfile(true)
		}
	case km.Is(key, ActHome):
		v.showRoot(homeSource())
	case km.Is(key, ActNotifications):
		v.showRoot(&notificationsSource{})
	case km.Is(key, ActLocal):
		v.showRoot(publicSource(true))
	case km.Is(key, ActFederated):
		v.showRoot(publicSource(false))
	case km.Is(key, ActBookmarks):
		v.showRoot(bookmarksSource())
	case km.Is(key, ActFavourites):
		v.showRoot(favouritesSource())
	case km.Is(key, ActLists):
		if !v.app.IsLoading() {
			v.pickList()
		}
	case km.Is(key, ActHashtag):
		v.promptHashtag()
	case km.Is(key, ActSearch):
		v.promptSearch()
	case km.Is(key, ActFavourite):
		v.toggleFavourite()
	case km.Is(key, ActBoost):
		v.toggleBoost()
	case km.Is(key, ActBookmark):
		v.toggleBookmark()
	case km.Is(key, ActFollow):
		v.toggleFollow()
	case km.Is(key, ActVote):
		v.startVote()
	case km.Is(key, ActCompose):
		if !v.app.IsLoading() {
			v.compose(false)
		}
	case km.Is(key, ActReply):
		if !v.app.IsLoading() {
			v.compose(true)
		}
	case km.Is(key, ActLinks):
		if links := v.selectedStatusLinks(); len(links) > 0 {
			v.linksView.SetLinks(links)
			v.showingLinks = true
			v.focusedView = 1
		} else {
			v.app.Flash("No links in this status")
		}
	case km.Is(key, ActToggleCW):
		v.toggleContentWarning()
	case km.Is(key, ActYank):
		v.yank()
	case km.Is(key, ActOpenOriginal):
		v.openOriginal()
	case km.Is(key, ActOpenLocal):
		v.openLocal()
	case km.Is(key, ActOpenLink):
		v.openLink()
	case km.Is(key, ActQuit):
		if !v.pop() {
			v.app.RequestQuit()
		}
	case km.Is(key, ActBack):
		v.pop()
	default:
		if v.focusedView == 0 {
			v.timeline.HandleKey(key)
			return
		}
		switch v.timeline.SelectedItem().(type) {
		case StatusItem:
			v.statusView.HandleKey(key)
		case AccountItem:
			v.accountView.HandleKey(key)
		case NotificationItem:
			v.notificationView.HandleKey(key)
		}
	}
}

func (v *HomeView) handleLinksKey(key vaxis.Key) {
	km := v.app.keys
	switch {
	case km.Is(key, ActFocusTimeline):
		v.focusedView = 0
		return
	case km.Is(key, ActFocusDetail):
		v.focusedView = 1
		return
	case km.Is(key, ActFocusNext):
		v.focusedView = (v.focusedView + 1) % 2
		return
	case km.Is(key, ActYank):
		v.yank()
		return
	case km.Is(key, ActToggleCW):
		v.toggleContentWarning()
		return
	case km.Is(key, ActQuit), km.Is(key, ActBack), km.Is(key, ActLinks):
		v.showingLinks = false
		v.focusedView = 0
		return
	}

	if v.focusedView == 0 {
		prev := v.timeline.SelectedItem()
		v.timeline.HandleKey(key)
		v.closeLinksIfMoved(prev)
		return
	}

	switch {
	case km.Is(key, ActDown):
		v.linksView.Move(1)
	case km.Is(key, ActUp):
		v.linksView.Move(-1)
	case km.Is(key, ActSelect):
		if link, ok := v.linksView.Selected(); ok {
			v.app.OpenURL(link.URL)
		}
	default:
		if key.Keycode >= '1' && key.Keycode <= '9' {
			if v.linksView.Select(int(key.Keycode - '1')) {
				if link, ok := v.linksView.Selected(); ok {
					v.app.OpenURL(link.URL)
				}
			}
		}
	}
}
