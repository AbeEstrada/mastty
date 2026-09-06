package tui

import (
	"git.sr.ht/~rockorager/vaxis"
	"git.sr.ht/~rockorager/vaxis/widgets/textinput"
)

// InputPrompt is a one-line text field shown in the footer.
type InputPrompt struct {
	label    string
	input    *textinput.Model
	onSubmit func(text string)
	onCancel func()
}

// PromptInput asks for a line of text in the footer. onSubmit runs on Enter
// with the entered text; Esc cancels.
func (app *App) PromptInput(label, initial string, onSubmit func(text string)) *InputPrompt {
	model := textinput.New()
	model.SetContent(initial)
	app.input = &InputPrompt{label: label, input: model, onSubmit: onSubmit}
	return app.input
}

func (p *InputPrompt) Draw(win vaxis.Window) {
	width, height := win.Size()
	if height == 0 {
		return
	}
	row := win.New(0, height-1, width, 1)
	row.Println(0, vaxis.Segment{Text: p.label, Style: boldStyle})
	labelWidth := len([]rune(p.label))
	if labelWidth < width {
		p.input.Draw(row.New(labelWidth, 0, width-labelWidth, 1))
	}
}

// HandleKey routes editing keys to the field and reports whether the prompt
// finished (submitted or cancelled).
func (p *InputPrompt) HandleKey(key vaxis.Key) bool {
	switch {
	case key.Matches(vaxis.KeyEnter):
		if p.onSubmit != nil {
			p.onSubmit(p.input.String())
		}
		return true
	case key.Matches(vaxis.KeyEsc):
		if p.onCancel != nil {
			p.onCancel()
		}
		return true
	}
	p.input.Update(key)
	return false
}
