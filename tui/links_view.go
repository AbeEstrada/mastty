package tui

import (
	"fmt"

	"git.sr.ht/~rockorager/vaxis"
)

type LinkItem struct {
	Label string
	URL   string
}

type LinksView struct {
	links    []LinkItem
	selected int
}

func CreateLinksView() *LinksView {
	return &LinksView{}
}

func (v *LinksView) SetLinks(links []LinkItem) {
	v.links = links
	v.selected = 0
}

func (v *LinksView) Len() int {
	return len(v.links)
}

func (v *LinksView) Selected() (LinkItem, bool) {
	if v.selected < 0 || v.selected >= len(v.links) {
		return LinkItem{}, false
	}
	return v.links[v.selected], true
}

// Move changes the selection by delta rows, clamped to the list.
func (v *LinksView) Move(delta int) {
	v.selected = max(0, min(len(v.links)-1, v.selected+delta))
}

// Select makes index the selection and reports whether it exists.
func (v *LinksView) Select(index int) bool {
	if index < 0 || index >= len(v.links) {
		return false
	}
	v.selected = index
	return true
}

func (v *LinksView) Draw(win vaxis.Window, focused bool) {
	width, height := win.Size()

	win.Println(0, vaxis.Segment{Text: "Links", Style: boldStyle})

	for i, item := range v.links {
		y := i + 2
		if y >= height {
			break
		}

		label := item.Label
		if label == "" {
			label = item.URL
		}

		segs := []vaxis.Segment{
			{Text: fmt.Sprintf(" %d. ", i+1), Style: dimStyle},
			{Text: label},
		}
		if label != item.URL {
			segs = append(segs, vaxis.Segment{Text: "  " + item.URL, Style: dimStyle})
		}
		rowWin := win.New(0, y, width, 1)
		if i == v.selected && focused {
			rowWin.Fill(vaxis.Cell{Character: vaxis.Character{Grapheme: " ", Width: 1}, Style: vaxis.Style{Attribute: vaxis.AttrReverse}})
			for s := range segs {
				segs[s].Style.Attribute = segs[s].Style.Attribute&^vaxis.AttrDim | vaxis.AttrReverse
			}
		}
		rowWin.PrintTruncate(0, segs...)
	}
}
