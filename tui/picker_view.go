package tui

import (
	"git.sr.ht/~rockorager/vaxis"
	"git.sr.ht/~rockorager/vaxis/widgets/align"
	"git.sr.ht/~rockorager/vaxis/widgets/border"
)

// PickerView is a small centered menu for choosing one item.
type PickerView struct {
	title    string
	items    []string
	selected int
	scroll   int
	rows     int
	onPick   func(index int)
}

func NewPicker(title string, items []string, onPick func(index int)) *PickerView {
	return &PickerView{title: title, items: items, onPick: onPick}
}

// HandleKey moves the selection, picks, or cancels. It reports whether the
// picker is finished.
func (p *PickerView) HandleKey(key vaxis.Key, km *Keymap) bool {
	switch {
	case km.Is(key, ActDown):
		p.selected = min(len(p.items)-1, p.selected+1)
	case km.Is(key, ActUp):
		p.selected = max(0, p.selected-1)
	case km.Is(key, ActTop):
		p.selected = 0
	case km.Is(key, ActBottom):
		p.selected = max(0, len(p.items)-1)
	case km.Is(key, ActSelect):
		if p.selected >= 0 && p.selected < len(p.items) && p.onPick != nil {
			p.onPick(p.selected)
		}
		return true
	case km.Is(key, ActBack), km.Is(key, ActQuit):
		return true
	default:
		if key.Keycode >= '1' && key.Keycode <= '9' {
			if index := int(key.Keycode - '1'); index < len(p.items) && p.onPick != nil {
				p.onPick(index)
				return true
			}
		}
	}
	return false
}

func (p *PickerView) Draw(win vaxis.Window) {
	width, height := win.Size()
	if width < 20 || height < 5 {
		return
	}
	boxWidth := min(60, width-2)
	boxHeight := min(len(p.items)+4, height-2)
	box := align.Center(win, boxWidth, boxHeight)
	box.Fill(vaxis.Cell{Character: vaxis.Character{Grapheme: " ", Width: 1}})
	inner := border.All(box, vaxis.Style{Foreground: vaxis.IndexColor(4)})
	innerWidth, innerHeight := inner.Size()
	inner.PrintTruncate(0, vaxis.Segment{Text: p.title, Style: boldStyle})

	p.rows = innerHeight - 2
	if p.rows <= 0 {
		return
	}
	if p.selected >= p.scroll+p.rows {
		p.scroll = p.selected - p.rows + 1
	}
	if p.selected < p.scroll {
		p.scroll = p.selected
	}
	if len(p.items) == 0 {
		inner.PrintTruncate(2, vaxis.Segment{Text: "Nothing to choose from", Style: dimStyle})
		return
	}
	for i := 0; i < p.rows && p.scroll+i < len(p.items); i++ {
		index := p.scroll + i
		row := inner.New(0, i+2, innerWidth, 1)
		style := vaxis.Style{}
		if index == p.selected {
			style = vaxis.Style{Attribute: vaxis.AttrReverse}
			row.Fill(vaxis.Cell{Character: vaxis.Character{Grapheme: " ", Width: 1}, Style: style})
		}
		label := p.items[index]
		if index < 9 {
			label = string(rune('1'+index)) + ". " + label
		} else {
			label = "   " + label
		}
		row.PrintTruncate(0, vaxis.Segment{Text: " " + label, Style: style})
	}
}
