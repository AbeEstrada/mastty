package tui

import (
	"fmt"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/constants"
)

type Header struct {
	text      string
	badge     int
	showBadge bool
	focus     string
}

func CreateHeader() *Header {
	return &Header{showBadge: true}
}

func (h *Header) SetText(text string) {
	h.text = text
}

func (h *Header) SetBadge(n int) {
	h.badge = n
}

func (h *Header) IncrementBadge() {
	h.badge++
}

func (h *Header) SetBadgeVisible(visible bool) {
	h.showBadge = visible
}

// SetFocus names the focused pane, shown at the right edge of the header.
func (h *Header) SetFocus(label string) {
	h.focus = label
}

func (h *Header) Draw(win vaxis.Window) {
	width, _ := win.Size()
	segments := []vaxis.Segment{
		{Text: constants.AppName, Style: boldStyle},
		{Text: " "},
		{Text: h.text},
	}
	if h.badge > 0 && h.showBadge {
		segments = append(segments, vaxis.Segment{
			Text:  fmt.Sprintf(" [%d]", h.badge),
			Style: boldStyle,
		})
	}
	win.PrintTruncate(0, segments...)

	if h.focus != "" {
		label := "[" + h.focus + "]"
		if x := width - len(label) - 1; x > 0 {
			win.New(x, 0, len(label), 1).Println(0, vaxis.Segment{Text: label, Style: dimStyle})
		}
	}
}
