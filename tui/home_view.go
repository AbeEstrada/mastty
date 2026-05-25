package tui

import (
	"context"
	"log"
	"sort"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

type HomeView struct {
	app            *App
	timeline       *TimelineView
	statusView     *StatusView
	accountView    *AccountView
	linksView      *LinksView
	focusedView    int
	isStreaming    bool
	showingLinks   bool
	lastSelectedID mastodon.ID
}

func CreateHomeView() *HomeView {
	v := &HomeView{
		statusView:  CreateStatusView(),
		accountView: CreateAccountView(),
		linksView:   CreateLinksView(),
		focusedView: 0,
	}
	timelineView := CreateTimelineView()
	timelineView.onLoadMore = v.loadMoreTimeline
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
	go v.getHomeTimeline()
	go v.startStreaming()
	v.app.header.SetText("Home")
}

func (v *HomeView) getHomeTimeline() {
	v.app.SetLoading(true)

	statuses, err := v.app.client.GetTimelineHome(context.Background(), nil)
	if err == nil {
		items := make([]TimelineItem, len(statuses))
		for i, s := range statuses {
			items[i] = StatusItem{Status: s}
		}
		v.timeline.AddTimeline(items, nil, nil)
		v.app.vx.PostEvent(vaxis.Redraw{})
	}

	v.app.SetLoading(false)
}

func (v *HomeView) getStatusContext() {
	selectedItem := v.timeline.SelectedItem()

	item, ok := selectedItem.(StatusItem)
	if !ok || item.Status == nil {
		return
	}

	original := item.Status

	if original.Reblog != nil {
		original = original.Reblog
	}

	v.app.SetLoading(true)

	if original.ID != "" {
		ctx, err := v.app.client.GetStatusContext(context.Background(), original.ID)

		if err == nil {
			items := make([]TimelineItem, 0, len(ctx.Ancestors)+1+len(ctx.Descendants))

			for _, status := range ctx.Ancestors {
				items = append(items, StatusItem{Status: status})
			}

			items = append(items, StatusItem{Status: original})

			for _, status := range ctx.Descendants {
				items = append(items, StatusItem{Status: status})
			}

			v.timeline.AddTimeline(items, StatusItem{Status: original}, nil)
			v.app.vx.PostEvent(vaxis.Redraw{})
		}
	}

	v.app.SetLoading(false)
}

func (v *HomeView) reloadHomeTimeline() {
	v.app.SetLoading(true)

	index := 0 // Home
	if index >= len(v.timeline.timelines) {
		v.app.SetLoading(false)
		return
	}

	timeline := &v.timeline.timelines[index]
	items := timeline.Items

	if len(items) == 0 {
		v.app.SetLoading(false)
		return
	}

	var sinceID mastodon.ID
	if firstItem, ok := items[0].(StatusItem); ok && firstItem.Status != nil {
		sinceID = firstItem.Status.ID
	} else {
		v.app.SetLoading(false)
		return
	}

	newStatuses, err := v.app.client.GetTimelineHome(context.Background(), &mastodon.Pagination{
		SinceID: sinceID,
		Limit:   40,
	})

	if err == nil && len(newStatuses) > 0 {
		newItems := make([]TimelineItem, len(newStatuses))
		for i, s := range newStatuses {
			newItems[i] = StatusItem{Status: s}
		}
		v.timeline.PrependToTimeline(index, newItems)
		v.app.vx.PostEvent(vaxis.Redraw{})
	}
	v.app.SetLoading(false)
}

func (v *HomeView) loadMoreTimeline() {
	v.app.SetLoading(true)

	index := v.timeline.index
	if index >= len(v.timeline.timelines) {
		v.app.SetLoading(false)
		return
	}

	timeline := &v.timeline.timelines[index]
	items := timeline.Items

	if len(items) == 0 {
		v.app.SetLoading(false)
		return
	}

	var maxID mastodon.ID
	if lastItem, ok := items[len(items)-1].(StatusItem); ok && lastItem.Status != nil {
		maxID = lastItem.Status.ID
	} else {
		v.app.SetLoading(false)
		return
	}

	var newStatuses []*mastodon.Status
	var err error

	if index == 0 {
		newStatuses, err = v.app.client.GetTimelineHome(context.Background(), &mastodon.Pagination{
			MaxID: maxID,
			Limit: 20,
		})
	} else if timeline.Account != nil {
		newStatuses, err = v.app.client.GetAccountStatuses(context.Background(), timeline.Account.ID, &mastodon.Pagination{
			MaxID: maxID,
			Limit: 20,
		})
	}

	if err == nil && len(newStatuses) > 0 {
		newItems := make([]TimelineItem, len(newStatuses))
		for i, s := range newStatuses {
			newItems[i] = StatusItem{Status: s}
		}
		v.timeline.AppendToTimeline(index, newItems)
		v.app.vx.PostEvent(vaxis.Redraw{})
	}

	v.app.SetLoading(false)
}

func (v *HomeView) goToAccountTimeline(currentUser bool) {
	selectedItem := v.timeline.SelectedItem()

	item, ok := selectedItem.(StatusItem)
	if !ok || item.Status == nil {
		return
	}
	original := item.Status

	if original.Reblog != nil {
		original = original.Reblog
	}

	if original.ID == "" {
		return
	}

	v.app.SetLoading(true)

	var account *mastodon.Account
	var err error

	if currentUser {
		account, err = v.app.client.GetAccountCurrentUser(context.Background())
	} else {
		account, err = v.app.client.GetAccount(context.Background(), original.Account.ID)
	}

	if err == nil {
		statuses, err := v.app.client.GetAccountStatuses(context.Background(), account.ID, &mastodon.Pagination{})
		if err == nil {
			items := make([]TimelineItem, 0, len(statuses)+1)

			items = append(items, AccountItem{Account: account})

			for _, s := range statuses {
				items = append(items, StatusItem{Status: s})
			}

			v.timeline.AddTimeline(items, StatusItem{Status: original}, account)
		}
	}

	v.app.SetLoading(false)
}

func (v *HomeView) startStreaming() {
	ctx := context.Background()

	events, err := v.app.client.StreamingUser(ctx)
	if err != nil {
		log.Printf("Failed to start streaming: %v", err)
		return
	}

	v.isStreaming = true

	for {
		select {
		case event := <-events:
			v.handleStreamingEvent(event)
		case <-ctx.Done():
			v.isStreaming = false
			return
		}
	}
}

func (v *HomeView) handleStreamingEvent(event mastodon.Event) {
	switch e := event.(type) {
	case *mastodon.UpdateEvent:
		v.timeline.PrependToTimeline(0, []TimelineItem{StatusItem{Status: e.Status}})
		v.app.vx.PostEvent(vaxis.Redraw{})

	case *mastodon.UpdateEditEvent:
		v.timeline.UpdateEdit(0, StatusItem{Status: e.Status})
		v.app.vx.PostEvent(vaxis.Redraw{})

	case *mastodon.NotificationEvent:
		log.Printf("New Notification [%s] from @%s\n", e.Notification.Type, e.Notification.Account.Acct)
		v.app.header.badge++
		v.app.vx.PostEvent(vaxis.Redraw{})

	case *mastodon.DeleteEvent:
		v.timeline.DeleteFromTimeline(0, e.ID)
		v.app.vx.PostEvent(vaxis.Redraw{})

	case *mastodon.ErrorEvent:
		log.Printf("Error %v\n", e.Error())

	default:
		log.Printf("Streaming unhandled event type\n")
	}
}

func (v *HomeView) selectedStatusLinks() []LinkItem {
	item, ok := v.timeline.SelectedItem().(StatusItem)
	if !ok || item.Status == nil {
		return nil
	}
	status := item.Status
	if status.Reblog != nil {
		status = status.Reblog
	}
	var items []LinkItem
	seen := map[string]bool{}
	for _, u := range utils.ExtractAllURLs(status.Content) {
		if !seen[u] {
			seen[u] = true
			items = append(items, LinkItem{Label: u, URL: u})
		}
	}
	if status.Card != nil && status.Card.URL != "" && !seen[status.Card.URL] {
		seen[status.Card.URL] = true
		items = append(items, LinkItem{Label: status.Card.URL, URL: status.Card.URL})
	}
	for _, att := range status.MediaAttachments {
		if att.URL == "" || seen[att.URL] {
			continue
		}
		if att.Type != "image" && att.Type != "video" && att.Type != "gifv" {
			continue
		}
		seen[att.URL] = true
		label := att.Description
		if label == "" {
			if att.Type == "image" {
				label = "[image]"
			} else {
				label = "[video]"
			}
		}
		items = append(items, LinkItem{Label: label, URL: att.URL})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return utils.IsTagLink(items[j].URL) && !utils.IsTagLink(items[i].URL)
	})
	return items
}


func (v *HomeView) Draw(win vaxis.Window) {
	var (
		leftRatio  = 2
		rightRatio = 3
	)
	if leftRatio <= 0 {
		leftRatio = 1
	}
	if rightRatio <= 0 {
		leftRatio = 1
	}

	width, height := win.Size()
	separatorStyle := vaxis.Style{
		Foreground: vaxis.IndexColor(0),
	}

	total := leftRatio + rightRatio
	split := width * leftRatio / total

	timelineWin := win.New(0, 1, split, height)
	detailWidth := max(0, width-split-2)
	detailWin := win.New(split+2, 1, detailWidth, height)

	v.timeline.Draw(timelineWin, v.focusedView == 0)

	selectedItem := v.timeline.SelectedItem()
	isDetailFocused := v.focusedView == 1

	if v.showingLinks {
		v.linksView.Draw(detailWin, v.focusedView == 1)
	} else if selectedItem != nil {
		currentID := selectedItem.ID()
		if currentID != v.lastSelectedID {
			v.statusView.ResetScroll()
			v.lastSelectedID = currentID
		}
		switch item := selectedItem.(type) {
		case StatusItem:
			v.statusView.Draw(detailWin, isDetailFocused, item.Status)
		case AccountItem:
			v.accountView.Draw(detailWin, isDetailFocused, item.Account)
		default:
			v.statusView.Draw(detailWin, isDetailFocused, nil)
		}
	} else {
		v.statusView.Draw(detailWin, isDetailFocused, nil)
	}

	for row := 0; row < height-2; row++ {
		win.SetCell(split, row+1, vaxis.Cell{
			Character: vaxis.Character{
				Grapheme: "│",
			},
			Style: separatorStyle,
		})
	}
}

func (v *HomeView) HandleKey(key vaxis.Key) {
	if v.showingLinks {
		if key.Matches('h') {
			v.focusedView = 0
			return
		}
		if key.Matches('l') {
			v.focusedView = 1
			return
		}
		if v.focusedView == 0 {
			prevID := v.timeline.SelectedItem().ID()
			v.timeline.HandleKey(key)
			if item := v.timeline.SelectedItem(); item != nil && item.ID() != prevID {
				v.showingLinks = false
			}
			return
		}
		result := v.linksView.HandleKey(key)
		switch result {
		case "open":
			if err := utils.OpenBrowser(v.linksView.links[v.linksView.selected].URL); err != nil {
				log.Printf("Failed to open URL: %v", err)
			}
		case "close":
			v.showingLinks = false
			v.focusedView = 0
		}
		return
	}
	if key.Matches(vaxis.KeyTab) {
		v.focusedView = (v.focusedView + 1) % 2
	} else if key.Matches('h') {
		v.focusedView = 0
	} else if key.Matches('l') {
		v.focusedView = 1
	} else if key.Matches('r') && !v.app.loading && !v.isStreaming {
		go v.reloadHomeTimeline()
	} else if key.Matches('t') && !v.app.loading {
		go v.getStatusContext()
	} else if key.Matches('u') && !v.app.loading {
		go v.goToAccountTimeline(false)
	} else if key.Matches('U') && !v.app.loading {
		go v.goToAccountTimeline(true)
	} else if key.Matches('i') {
		links := v.selectedStatusLinks()
		if len(links) > 0 {
			v.linksView.SetLinks(links)
			v.showingLinks = true
			v.focusedView = 1
		}
	} else if key.Matches('q') {
		if len(v.timeline.timelines) <= 1 {
			v.app.RequestQuit()
		} else {
			v.timeline.RemoveLastTimeline()
		}
		return
	} else {
		if v.focusedView == 0 {
			v.timeline.HandleKey(key)
		} else {
			selectedItem := v.timeline.SelectedItem()
			if selectedItem != nil {
				switch selectedItem.(type) {
				case StatusItem:
					v.statusView.HandleKey(key)
				case AccountItem:
					v.accountView.HandleKey(key)
				}
			}
		}
	}
}
