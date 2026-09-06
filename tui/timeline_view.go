package tui

import (
	"fmt"
	"strings"
	"time"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

const maxReadStatuses = 10000

type TimelineView struct {
	app          *App
	stack        []*Timeline
	nextID       int
	onLoadMore   func()
	readStatuses map[mastodon.ID]bool
	viewHeight   int // rows available in the last Draw
	viewWidth    int
}

func CreateTimelineView() *TimelineView {
	return &TimelineView{
		readStatuses: make(map[mastodon.ID]bool),
	}
}

func (v *TimelineView) SetApp(app *App) {
	v.app = app
}

func (v *TimelineView) markRead(id mastodon.ID) {
	if len(v.readStatuses) >= maxReadStatuses {
		v.readStatuses = make(map[mastodon.ID]bool)
	}
	v.readStatuses[id] = true
}

// statusFlags returns compact markers for a status: content warning, media,
// poll, and the user's own favourite and bookmark state.
func statusFlags(s *mastodon.Status) string {
	var b strings.Builder
	if s.SpoilerText != "" || len(s.Filtered) > 0 || (s.Sensitive && len(s.MediaAttachments) > 0) {
		b.WriteString("⚠")
	}
	if len(s.MediaAttachments) > 0 {
		b.WriteString("▣")
	}
	if s.Poll != nil {
		b.WriteString("▤")
	}
	if boolField(s.Favourited) {
		b.WriteString("★")
	}
	if boolField(s.Bookmarked) {
		b.WriteString("⚑")
	}
	return b.String()
}

func accountName(a *mastodon.Account) string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Username
}

// timestamp formats a row time: the full date and time by default, the time
// alone in narrow panes, or a compact relative form when preferred.
func (v *TimelineView) timestamp(t time.Time, now time.Time, width int) string {
	if v.app.config.Preferences.Timestamp == "relative" {
		return fmt.Sprintf("%5s", utils.FormatTimeShort(t.Local(), now))
	}
	if width < 60 {
		return t.Local().Format("15:04")
	}
	return t.Local().Format("2006-01-02 15:04")
}

// rowSegments renders one list row: reply indentation, timestamp, type glyph,
// account handle, and marker glyphs.
func (v *TimelineView) rowSegments(item TimelineItem, timeline *Timeline, width int, now time.Time) []vaxis.Segment {
	switch t := item.(type) {
	case StatusItem:
		if t.Status == nil {
			return nil
		}
		display := t.Original()

		indent := ""
		if depth, ok := timeline.depths[t.ID()]; ok && depth > 0 {
			indent = strings.Repeat(" ", min(depth, 6))
		}

		glyph := " "
		switch {
		case timeline.focalID != "" && t.ID() == timeline.focalID:
			glyph = "▶"
		case t.Reblog != nil:
			glyph = "♺"
		case t.InReplyToID != nil:
			glyph = "↩"
		}

		segs := []vaxis.Segment{
			{Text: indent + v.timestamp(t.CreatedAt, now, width) + " ", Style: dimStyle},
			{Text: glyph + " "},
			{Text: "@" + t.Account.Acct},
		}
		if flags := statusFlags(display); flags != "" {
			segs = append(segs, vaxis.Segment{Text: " " + flags})
		}
		return segs

	case NotificationItem:
		if t.Notification == nil {
			return nil
		}
		return []vaxis.Segment{
			{Text: v.timestamp(t.CreatedAt, now, width) + " ", Style: dimStyle},
			{Text: notificationGlyph(t.Type) + " "},
			{Text: "@" + t.Account.Acct},
			{Text: " " + notificationVerb(t.Type), Style: dimStyle},
		}

	case AccountItem:
		if t.Account == nil {
			return nil
		}
		return []vaxis.Segment{{Text: "@" + t.Acct}}
	}
	return nil
}

func (v *TimelineView) Draw(win vaxis.Window, focused bool) {
	width, height := win.Size()
	v.viewHeight, v.viewWidth = height, width

	timeline := v.Current()
	if timeline == nil || len(timeline.Items) == 0 {
		text := "Nothing here yet"
		if timeline == nil || v.app.IsLoading() {
			text = "Loading..."
		}
		win.Println(0, vaxis.Segment{Text: text, Style: dimStyle})
		return
	}

	items := timeline.Items
	selected := timeline.Selected
	now := time.Now()

	y := 0
	for i := timeline.scrollOffset; i < len(items) && y < height; i++ {
		item := items[i]
		segs := v.rowSegments(item, timeline, width, now)
		if segs == nil {
			continue
		}

		isSelected := selected != nil && item.ID() == selected.ID()
		rowWin := win.New(0, y, width, 1)
		switch {
		case isSelected && focused:
			rowWin.Fill(vaxis.Cell{
				Character: vaxis.Character{Grapheme: " ", Width: 1},
				Style:     vaxis.Style{Attribute: vaxis.AttrReverse},
			})
			for s := range segs {
				segs[s].Style.Attribute = segs[s].Style.Attribute&^vaxis.AttrDim | vaxis.AttrReverse
			}
		case isSelected:
			for s := range segs {
				segs[s].Style.Attribute = segs[s].Style.Attribute&^vaxis.AttrDim | vaxis.AttrBold
				segs[s].Style.UnderlineStyle = vaxis.UnderlineSingle
			}
		case v.readStatuses[item.ID()]:
			for s := range segs {
				segs[s].Style.Attribute = segs[s].Style.Attribute&^vaxis.AttrBold | vaxis.AttrDim
			}
		}
		rowWin.PrintTruncate(0, segs...)
		y++
	}
}

func (v *TimelineView) HandleKey(key vaxis.Key) {
	timeline := v.Current()
	if timeline == nil || len(timeline.Items) == 0 {
		return
	}

	km := v.app.keys
	current := timeline.SelectedIndex()
	last := len(timeline.Items) - 1
	jump := max(1, v.viewHeight/2)

	target := current
	switch {
	case km.Is(key, ActDown):
		if current < 0 {
			target = 0
		} else {
			target = current + 1
		}
	case km.Is(key, ActUp):
		if current < 0 {
			target = last
		} else {
			target = current - 1
		}
	case km.Is(key, ActPageDown):
		if current < 0 {
			target = jump
		} else {
			target = current + jump
		}
	case km.Is(key, ActPageUp):
		if current < 0 {
			target = 0
		} else {
			target = current - jump
		}
	case km.Is(key, ActTop):
		target = 0
	case km.Is(key, ActBottom):
		target = last
	default:
		return
	}
	v.moveTo(timeline, target)
}

// Move shifts the selection by delta rows.
func (v *TimelineView) Move(delta int) {
	timeline := v.Current()
	if timeline == nil || len(timeline.Items) == 0 {
		return
	}
	v.moveTo(timeline, max(0, timeline.SelectedIndex())+delta)
}

// SelectRow selects the item drawn on the given row of the view.
func (v *TimelineView) SelectRow(row int) bool {
	timeline := v.Current()
	if timeline == nil || row < 0 {
		return false
	}
	index := timeline.scrollOffset + row
	if index >= len(timeline.Items) {
		return false
	}
	timeline.Selected = timeline.Items[index]
	v.markRead(timeline.Selected.ID())
	return true
}

// moveTo selects the item at target, requesting more items when moving past
// the end, and keeps the selection visible.
func (v *TimelineView) moveTo(timeline *Timeline, target int) {
	items := timeline.Items
	if target >= len(items) {
		if v.onLoadMore != nil && !v.app.IsLoading() && !timeline.exhausted {
			v.onLoadMore()
		}
		target = len(items) - 1
	}
	if target < 0 {
		target = 0
	}
	timeline.Selected = items[target]
	v.markRead(items[target].ID())
	v.scrollToSelection(timeline)
}

// scrollToSelection adjusts the scroll offset so the selection is visible.
func (v *TimelineView) scrollToSelection(timeline *Timeline) {
	if v.viewHeight <= 0 {
		return
	}
	index := timeline.SelectedIndex()
	if index < 0 {
		return
	}
	if index >= timeline.scrollOffset+v.viewHeight {
		timeline.scrollOffset = index - v.viewHeight + 1
	}
	if index < timeline.scrollOffset {
		timeline.scrollOffset = index
	}
}
