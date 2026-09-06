package tui

import (
	"git.sr.ht/~rockorager/vaxis"
)

// Footer draws the last row of the screen. The App decides what it shows: a
// confirmation prompt, a transient message, or the loading indicator.
type Footer struct{}

func CreateFooter() *Footer {
	return &Footer{}
}

func (f *Footer) Draw(win vaxis.Window, segs ...vaxis.Segment) {
	_, height := win.Size()
	win.Println(height-1, segs...)
}
