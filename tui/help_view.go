package tui

import (
	"fmt"

	"git.sr.ht/~rockorager/vaxis"
	"git.sr.ht/~rockorager/vaxis/widgets/align"
	"git.sr.ht/~rockorager/vaxis/widgets/border"
)

// HelpView is a modal listing every keybinding, generated from the keymap so
// it never drifts from the actual bindings.
type HelpView struct {
	visible bool
	scroll  int
	rows    int
}

func (h *HelpView) Toggle() {
	h.visible = !h.visible
	h.scroll = 0
}

// HandleKey scrolls the help with the navigation keys and closes it on any
// other key.
func (h *HelpView) HandleKey(key vaxis.Key, km *Keymap) {
	switch {
	case km.Is(key, ActDown):
		h.scroll++
	case km.Is(key, ActUp):
		if h.scroll > 0 {
			h.scroll--
		}
	case km.Is(key, ActPageDown):
		h.scroll += max(1, h.rows/2)
	case km.Is(key, ActPageUp):
		h.scroll = max(0, h.scroll-max(1, h.rows/2))
	default:
		h.visible = false
	}
}

func (h *HelpView) Draw(win vaxis.Window, km *Keymap) {
	if !h.visible {
		return
	}
	width, height := win.Size()
	if width < 20 || height < 5 {
		return
	}

	var lines [][]vaxis.Segment
	context := ""
	for _, e := range km.Entries() {
		if e.Context != context {
			if context != "" {
				lines = append(lines, nil)
			}
			context = e.Context
			lines = append(lines, []vaxis.Segment{{Text: context, Style: boldStyle}})
		}
		lines = append(lines, []vaxis.Segment{
			{Text: fmt.Sprintf("  %-10s ", e.Keys), Style: vaxis.Style{Foreground: vaxis.IndexColor(6)}},
			{Text: e.Desc},
		})
	}
	lines = append(lines, nil, []vaxis.Segment{{Text: "Press any other key to close", Style: dimStyle}})

	boxWidth := min(80, width-2)
	boxHeight := min(len(lines)+2, height-2)
	box := align.Center(win, boxWidth, boxHeight)
	box.Fill(vaxis.Cell{Character: vaxis.Character{Grapheme: " ", Width: 1}})
	inner := border.All(box, vaxis.Style{Foreground: vaxis.IndexColor(4)})
	_, innerHeight := inner.Size()
	h.rows = innerHeight

	maxScroll := max(0, len(lines)-innerHeight)
	if h.scroll > maxScroll {
		h.scroll = maxScroll
	}
	for i := 0; i < innerHeight && h.scroll+i < len(lines); i++ {
		line := lines[h.scroll+i]
		if line == nil {
			continue
		}
		inner.PrintTruncate(i, line...)
	}
}
