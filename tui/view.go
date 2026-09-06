package tui

import "git.sr.ht/~rockorager/vaxis"

type View interface {
	SetApp(app *App)
	OnActivate()
	Draw(win vaxis.Window)
	HandleKey(key vaxis.Key)
}

// Closer is implemented by views that own background work to stop on exit.
type Closer interface {
	Close()
}

// MouseHandler is implemented by views that react to mouse events.
type MouseHandler interface {
	HandleMouse(m vaxis.Mouse)
}
