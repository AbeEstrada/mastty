package tui

import (
	"strconv"

	"github.com/mattn/go-mastodon"
)

type TimelineItem interface {
	ID() mastodon.ID
}

type StatusItem struct {
	*mastodon.Status
}

func (s StatusItem) ID() mastodon.ID {
	return s.Status.ID
}

// Original returns the boosted status for a boost, or the status itself.
func (s StatusItem) Original() *mastodon.Status {
	return originalStatus(s.Status)
}

type AccountItem struct {
	*mastodon.Account
}

func (a AccountItem) ID() mastodon.ID {
	return a.Account.ID
}

type NotificationItem struct {
	*mastodon.Notification
}

func (n NotificationItem) ID() mastodon.ID {
	return n.Notification.ID
}

// originalStatus unwraps a boost so callers act on the status being shown.
func originalStatus(s *mastodon.Status) *mastodon.Status {
	if s != nil && s.Reblog != nil {
		return s.Reblog
	}
	return s
}

func statusItems(statuses []*mastodon.Status) []TimelineItem {
	items := make([]TimelineItem, 0, len(statuses))
	for _, s := range statuses {
		if s != nil {
			items = append(items, StatusItem{Status: s})
		}
	}
	return items
}

func accountItems(accounts []*mastodon.Account) []TimelineItem {
	items := make([]TimelineItem, 0, len(accounts))
	for _, a := range accounts {
		if a != nil {
			items = append(items, AccountItem{Account: a})
		}
	}
	return items
}

func notificationItems(notifications []*mastodon.Notification) []TimelineItem {
	items := make([]TimelineItem, 0, len(notifications))
	for _, n := range notifications {
		if n != nil {
			items = append(items, NotificationItem{Notification: n})
		}
	}
	return items
}

func notificationGlyph(kind string) string {
	switch kind {
	case "favourite":
		return "★"
	case "reblog":
		return "♺"
	case "mention":
		return "@"
	case "follow", "follow_request":
		return "+"
	case "poll":
		return "▤"
	case "update":
		return "✎"
	}
	return "•"
}

func notificationVerb(kind string) string {
	switch kind {
	case "favourite":
		return "favourited your post"
	case "reblog":
		return "boosted your post"
	case "mention":
		return "mentioned you"
	case "follow":
		return "followed you"
	case "follow_request":
		return "requested to follow you"
	case "poll":
		return "poll has ended"
	case "update":
		return "edited a post"
	case "status":
		return "posted"
	case "admin.sign_up":
		return "signed up"
	case "admin.report":
		return "filed a report"
	}
	return kind
}

// boolField reads the interface{} booleans go-mastodon uses for fields such as
// Favourited and Reblogged.
func boolField(v interface{}) bool {
	b, ok := v.(bool)
	return ok && b
}

// idField reads the interface{} IDs go-mastodon uses for InReplyToID, which
// servers send as strings or, rarely, numbers.
func idField(v interface{}) mastodon.ID {
	switch id := v.(type) {
	case string:
		return mastodon.ID(id)
	case float64:
		return mastodon.ID(strconv.FormatFloat(id, 'f', 0, 64))
	}
	return ""
}
