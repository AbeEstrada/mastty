package tui

import (
	"strings"

	"github.com/mattn/go-mastodon"
)

// Timeline is one entry in the navigation stack: a root timeline (home,
// notifications, local, ...) or a thread, profile, or search pushed on top.
type Timeline struct {
	id           int
	Source       Source
	Items        []TimelineItem
	Selected     TimelineItem
	scrollOffset int
	exhausted    bool // no older pages are available

	// Thread layout: the status the thread was opened from and the reply depth
	// of every status, used to indent rows.
	focalID mastodon.ID
	depths  map[mastodon.ID]int
}

func (t *Timeline) Title() string {
	if t.Source == nil {
		return ""
	}
	return t.Source.Title()
}

// NewestID returns the ID of the first status or notification, which is what
// since_id pagination needs.
func (t *Timeline) NewestID() (mastodon.ID, bool) {
	for _, item := range t.Items {
		switch item.(type) {
		case StatusItem, NotificationItem:
			return item.ID(), true
		}
	}
	return "", false
}

// SelectedIndex returns the position of the selected item, or -1.
func (t *Timeline) SelectedIndex() int {
	if t.Selected == nil {
		return -1
	}
	id := t.Selected.ID()
	for i, item := range t.Items {
		if item.ID() == id {
			return i
		}
	}
	return -1
}

// SetItems replaces the contents, keeping the selection (or the given one) when
// it is still present and otherwise selecting the first item.
func (t *Timeline) SetItems(items []TimelineItem, selected TimelineItem) {
	var target mastodon.ID
	switch {
	case selected != nil:
		target = selected.ID()
	case t.Selected != nil:
		target = t.Selected.ID()
	}

	t.Items = items
	t.exhausted = false
	t.scrollOffset = 0
	t.Selected = nil
	if len(items) > 0 {
		t.Selected = items[0]
	}
	for _, item := range items {
		if item.ID() == target {
			t.Selected = item
			break
		}
	}
	if layout, ok := t.Source.(threadLayout); ok {
		t.focalID, t.depths = layout.Layout()
	}
}

// insert adds the items not already present, at the front or the back, and
// returns how many were added.
func (t *Timeline) insert(newItems []TimelineItem, prepend bool) int {
	if len(newItems) == 0 {
		return 0
	}
	existing := make(map[mastodon.ID]struct{}, len(t.Items))
	for _, item := range t.Items {
		existing[item.ID()] = struct{}{}
	}
	fresh := make([]TimelineItem, 0, len(newItems))
	for _, item := range newItems {
		if _, dup := existing[item.ID()]; dup {
			continue
		}
		existing[item.ID()] = struct{}{}
		fresh = append(fresh, item)
	}
	if len(fresh) == 0 {
		return 0
	}

	if prepend {
		t.Items = append(fresh, t.Items...)
		// Keep the viewport anchored on what the user was looking at.
		if t.scrollOffset > 0 {
			t.scrollOffset += len(fresh)
		}
	} else {
		t.Items = append(t.Items, fresh...)
	}
	if t.Selected == nil {
		t.Selected = t.Items[0]
	}
	return len(fresh)
}

func (t *Timeline) Prepend(items []TimelineItem) int { return t.insert(items, true) }
func (t *Timeline) Append(items []TimelineItem) int  { return t.insert(items, false) }

// Replace swaps in an item with the same ID and reports whether it was found.
func (t *Timeline) Replace(item TimelineItem) bool {
	if item == nil {
		return false
	}
	for i, existing := range t.Items {
		if existing.ID() == item.ID() {
			t.Items[i] = item
			if t.Selected != nil && t.Selected.ID() == item.ID() {
				t.Selected = item
			}
			return true
		}
	}
	return false
}

// ReplaceStatus swaps in an updated status wherever it appears: directly,
// inside a boost that wraps it, or as the subject of a notification.
func (t *Timeline) ReplaceStatus(updated *mastodon.Status) bool {
	if updated == nil {
		return false
	}
	changed := false
	for i, item := range t.Items {
		var replacement TimelineItem
		switch it := item.(type) {
		case StatusItem:
			switch {
			case it.Status == nil:
				continue
			case it.Status.ID == updated.ID:
				replacement = StatusItem{Status: updated}
			case it.Status.Reblog != nil && it.Status.Reblog.ID == updated.ID:
				wrapper := *it.Status
				wrapper.Reblog = updated
				replacement = StatusItem{Status: &wrapper}
			default:
				continue
			}
		case NotificationItem:
			if it.Notification == nil || it.Status == nil || it.Status.ID != updated.ID {
				continue
			}
			n := *it.Notification
			n.Status = updated
			replacement = NotificationItem{Notification: &n}
		default:
			continue
		}
		t.Items[i] = replacement
		if t.Selected != nil && t.Selected.ID() == item.ID() {
			t.Selected = replacement
		}
		changed = true
	}
	return changed
}

// Delete removes the item with the given ID, moving the selection to a
// neighbor when it was selected.
func (t *Timeline) Delete(targetID mastodon.ID) bool {
	index := -1
	for i, item := range t.Items {
		if item.ID() == targetID {
			index = i
			break
		}
	}
	if index == -1 {
		return false
	}

	remaining := make([]TimelineItem, 0, len(t.Items)-1)
	remaining = append(remaining, t.Items[:index]...)
	remaining = append(remaining, t.Items[index+1:]...)
	t.Items = remaining

	if t.Selected != nil && t.Selected.ID() == targetID {
		switch {
		case len(remaining) == 0:
			t.Selected = nil
		case index >= len(remaining):
			t.Selected = remaining[len(remaining)-1]
		default:
			t.Selected = remaining[index]
		}
	}
	if t.scrollOffset > 0 && t.scrollOffset >= len(remaining) {
		t.scrollOffset = max(0, len(remaining)-1)
	}
	return true
}

// NewTimeline creates an empty timeline for a source with a stable id.
func (v *TimelineView) NewTimeline(source Source) *Timeline {
	v.nextID++
	return &Timeline{id: v.nextID, Source: source}
}

// SetRoot replaces the whole stack with one timeline.
func (v *TimelineView) SetRoot(t *Timeline) {
	v.stack = []*Timeline{t}
	v.afterChange(t)
}

// Push shows t on top of the current timeline.
func (v *TimelineView) Push(t *Timeline) {
	v.stack = append(v.stack, t)
	v.afterChange(t)
}

// Pop returns to the previous timeline and reports whether it did.
func (v *TimelineView) Pop() bool {
	if len(v.stack) <= 1 {
		return false
	}
	v.stack = v.stack[:len(v.stack)-1]
	v.afterChange(v.stack[len(v.stack)-1])
	return true
}

func (v *TimelineView) afterChange(t *Timeline) {
	if t.Selected != nil {
		v.markRead(t.Selected.ID())
	}
	v.scrollToSelection(t)
	v.setTitle()
}

// Current returns the timeline being shown, or nil before the first load.
func (v *TimelineView) Current() *Timeline {
	if len(v.stack) == 0 {
		return nil
	}
	return v.stack[len(v.stack)-1]
}

// Root returns the bottom of the stack, or nil before the first load.
func (v *TimelineView) Root() *Timeline {
	if len(v.stack) == 0 {
		return nil
	}
	return v.stack[0]
}

// Depth is the number of timelines on the stack.
func (v *TimelineView) Depth() int {
	return len(v.stack)
}

// Stack returns the timelines from root to current.
func (v *TimelineView) Stack() []*Timeline {
	return v.stack
}

func (v *TimelineView) SelectedItem() TimelineItem {
	t := v.Current()
	if t == nil {
		return nil
	}
	return t.Selected
}

func (v *TimelineView) setTitle() {
	titles := make([]string, 0, len(v.stack))
	for _, t := range v.stack {
		titles = append(titles, t.Title())
	}
	v.app.header.SetText(strings.Join(titles, " → "))
	root := v.Root()
	v.app.header.SetBadgeVisible(root == nil || root.Title() != titleNotifications)
}
