package tui

import (
	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

// NotificationView shows who did what, then the status or profile involved.
type NotificationView struct {
	statusView  *StatusView
	accountView *AccountView
}

func CreateNotificationView(statusView *StatusView, accountView *AccountView) *NotificationView {
	return &NotificationView{statusView: statusView, accountView: accountView}
}

func (v *NotificationView) Draw(win vaxis.Window, focused bool, n *mastodon.Notification) {
	if n == nil {
		return
	}
	width, height := win.Size()
	win.PrintTruncate(0,
		vaxis.Segment{Text: notificationGlyph(n.Type) + " "},
		vaxis.Segment{Text: accountName(&n.Account), Style: boldStyle},
		vaxis.Segment{Text: " @" + n.Account.Acct, Style: dimStyle},
		vaxis.Segment{Text: " " + notificationVerb(n.Type)},
		vaxis.Segment{Text: " · " + utils.FormatTimeSince(n.CreatedAt.Local()), Style: dimStyle},
	)
	if height <= 2 {
		return
	}
	body := win.New(0, 2, width, height-2)
	if n.Status != nil {
		v.statusView.Draw(body, focused, n.Status)
		return
	}
	account := n.Account
	v.accountView.Draw(body, focused, &account)
}

func (v *NotificationView) HandleKey(key vaxis.Key) {
	v.statusView.HandleKey(key)
}
