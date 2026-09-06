package tui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"git.sr.ht/~rockorager/vaxis"
	"git.sr.ht/~rockorager/vaxis/widgets/spinner"
	"github.com/AbeEstrada/tuit/api"
	"github.com/AbeEstrada/tuit/auth"
	"github.com/AbeEstrada/tuit/config"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

const messageDuration = 6 * time.Second

// Prompt is a yes/no question shown in the footer.
type Prompt struct {
	Text  string
	OnYes func()
	OnNo  func()
	// OnOther handles any other key; return true to keep the prompt open
	// after handling it.
	OnOther func(key vaxis.Key) bool
}

// App owns the terminal, the Mastodon client, and the active view. All fields
// are read and written on the main thread only; background goroutines hand
// their results to the main thread through Sync.
type App struct {
	vx      *vaxis.Vaxis
	views   map[string]View
	view    View
	header  *Header
	footer  *Footer
	help    *HelpView
	input   *InputPrompt
	keys    *Keymap
	spinner *spinner.Model

	running      bool
	closed       atomic.Bool
	vxClosed     bool // vaxis closed itself (signal); do not close it again
	loadingCount int
	fatalErr     error
	prompt       *Prompt
	message      string
	hint         string
	messageStyle vaxis.Style
	messageGen   int
	split        int // column of the vertical separator, set by the active view
	cellPixW     int // terminal cell size in pixels from resize events, 0 when unknown
	cellPixH     int

	config         *config.Config
	client         *mastodon.Client
	customClient   *api.Client
	currentAccount *mastodon.Account
}

func CreateApp() (*App, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "tuit: %v\n", err)
		}
		fmt.Println("No usable credentials found, starting authentication setup.")
		cfg, err = auth.SetupAuth()
		if err != nil {
			return nil, fmt.Errorf("authentication setup failed: %w", err)
		}
	}

	vx, err := vaxis.New(vaxis.Options{})
	if err != nil {
		return nil, err
	}

	app := &App{
		vx:      vx,
		views:   make(map[string]View),
		header:  CreateHeader(),
		footer:  CreateFooter(),
		help:    &HelpView{},
		keys:    NewKeymap(cfg.Preferences.Keys),
		spinner: spinner.New(vx, 120*time.Millisecond),
		running: true,
		config:  cfg,
	}
	app.views["home"] = CreateHomeView()
	app.view = app.views["home"]
	for _, view := range app.views {
		view.SetApp(app)
	}

	utils.InitImageCache(vx, app.RequestRedraw, cfg.Preferences.ImagesEnabled())

	return app, nil
}

func (app *App) Run() error {
	go app.initClient()

	for app.running {
		app.draw()
		app.handleEvent()
	}
	return app.fatalErr
}

// Close stops background work and restores the terminal. Safe to call twice.
func (app *App) Close() {
	if app.closed.Swap(true) {
		return
	}
	for _, view := range app.views {
		if c, ok := view.(Closer); ok {
			c.Close()
		}
	}
	if !app.vxClosed {
		app.vx.Close()
	}
}

// Sync runs fn on the main thread and redraws afterwards. It must be called
// from a background goroutine; main-thread code mutates state directly.
func (app *App) Sync(fn func()) {
	if app.closed.Load() {
		return
	}
	app.vx.PostEventBlocking(vaxis.SyncFunc(fn))
}

// RequestRedraw wakes the main loop so it redraws. Safe from any goroutine.
func (app *App) RequestRedraw() {
	if app.closed.Load() {
		return
	}
	app.vx.PostEvent(vaxis.Redraw{})
}

func (app *App) draw() {
	win := app.vx.Window()
	win.Clear()
	width, height := win.Size()

	// The body sits between the header row and the rule plus footer rows. It is
	// drawn first so the header can reflect the view's focus state.
	if height > 3 && app.view != nil {
		app.view.Draw(win.New(0, 1, width, height-3))
	}

	if height > 0 {
		app.header.Draw(win.New(0, 0, width, 1))
	}

	if height >= 2 {
		ruleStyle := vaxis.Style{Foreground: vaxis.IndexColor(0)}
		for col := 0; col < width; col++ {
			grapheme := "─"
			if col == app.split && app.split > 0 {
				grapheme = "┴"
			}
			win.SetCell(col, height-2, vaxis.Cell{
				Character: vaxis.Character{Grapheme: grapheme},
				Style:     ruleStyle,
			})
		}
	}

	app.vx.HideCursor()
	if app.input != nil {
		app.input.Draw(win)
	} else {
		app.footer.Draw(win, app.footerSegments()...)
		if app.prompt == nil && app.message == "" && app.loadingCount > 0 && height > 0 {
			app.spinner.Draw(win.New(0, height-1, 1, 1))
		}
	}

	app.help.Draw(win, app.keys)

	app.vx.Render()
}

func (app *App) footerSegments() []vaxis.Segment {
	switch {
	case app.prompt != nil:
		return []vaxis.Segment{{
			Text:  app.prompt.Text + " (y/n)",
			Style: vaxis.Style{Attribute: vaxis.AttrBold},
		}}
	case app.message != "":
		return []vaxis.Segment{{Text: app.message, Style: app.messageStyle}}
	case app.hint != "":
		return []vaxis.Segment{{Text: app.hint, Style: dimStyle}}
	case app.loadingCount > 0:
		return []vaxis.Segment{{Text: "  Loading...", Style: dimStyle}}
	}
	return nil
}

func (app *App) beginLoading() {
	app.loadingCount++
	if app.loadingCount == 1 {
		app.spinner.Start()
	}
}

func (app *App) endLoading() {
	if app.loadingCount == 0 {
		return
	}
	app.loadingCount--
	if app.loadingCount == 0 {
		app.spinner.Stop()
	}
}

func (app *App) IsLoading() bool {
	return app.loadingCount > 0
}

// SetError shows msg in the footer for a few seconds and logs it.
func (app *App) SetError(msg string) {
	log.Printf("error: %s", msg)
	app.showMessage(msg, vaxis.Style{Foreground: vaxis.IndexColor(1), Attribute: vaxis.AttrBold})
}

// SetHint shows a persistent footer hint until cleared with an empty string.
func (app *App) SetHint(text string) {
	app.hint = text
}

// Flash shows a short informational message in the footer.
func (app *App) Flash(msg string) {
	app.showMessage(msg, vaxis.Style{})
}

func (app *App) showMessage(msg string, style vaxis.Style) {
	app.message = msg
	app.messageStyle = style
	app.messageGen++
	gen := app.messageGen
	time.AfterFunc(messageDuration, func() {
		app.Sync(func() {
			if app.messageGen == gen {
				app.message = ""
			}
		})
	})
}

// Confirm asks a yes/no question in the footer and runs onYes on confirmation.
func (app *App) Confirm(text string, onYes func()) {
	app.prompt = &Prompt{Text: text, OnYes: onYes}
}

func (app *App) RequestQuit() {
	app.Confirm("Quit?", func() { app.running = false })
}

// OpenURL opens url in the system browser and reports failures in the footer.
func (app *App) OpenURL(url string) {
	if url == "" {
		return
	}
	if err := utils.OpenBrowser(url); err != nil {
		app.SetError("Could not open browser: " + err.Error())
	}
}

func (app *App) initClient() {
	app.Sync(app.beginLoading)

	client := mastodon.NewClient(&mastodon.Config{
		Server:       app.config.Auth.Server,
		ClientID:     app.config.Auth.ClientID,
		ClientSecret: app.config.Auth.ClientSecret,
		AccessToken:  app.config.Auth.AccessToken,
	})

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	account, err := client.GetAccountCurrentUser(ctx)
	if err != nil {
		app.Sync(func() {
			app.endLoading()
			app.fatalErr = fmt.Errorf("failed to authenticate with %s: %w", app.config.Auth.Server, err)
			app.running = false
		})
		return
	}

	app.Sync(func() {
		app.endLoading()
		app.client = client
		app.customClient = api.NewClient(client)
		app.currentAccount = account
		if app.view != nil {
			app.view.OnActivate()
		}
		go app.fetchUnreadNotifications(app.customClient)
	})
}

func (app *App) fetchUnreadNotifications(client *api.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	count, err := client.GetNotificationsUnreadCount(ctx)
	if err != nil {
		log.Printf("Failed to fetch unread notifications: %v", err)
		return
	}
	app.Sync(func() { app.header.SetBadge(count) })
}

func (app *App) handleEvent() {
	switch ev := app.vx.PollEvent().(type) {
	case vaxis.Key:
		app.handleKeyEvent(ev)
	case vaxis.Mouse:
		app.handleMouse(ev)
	case vaxis.SyncFunc:
		ev()
	case vaxis.PasteStartEvent, vaxis.PasteEndEvent:
		if app.input != nil {
			app.input.input.Update(ev)
		}
	case vaxis.Resize:
		app.applyResize(ev)
	case vaxis.QuitEvent:
		// vaxis posts this after closing itself on SIGINT or SIGTERM.
		app.vxClosed = true
		app.running = false
	}
}

// applyResize passes a terminal size to vaxis and records the cell pixel size
// for image scaling. Reports without pixel sizes (ioctl on macOS, including the
// one vaxis issues after the editor closes) reuse the last known cell size so
// vaxis never computes a zero cell geometry, which would make its image
// scaler divide by zero.
func (app *App) applyResize(ev vaxis.Resize) {
	if ev.Cols <= 0 || ev.Rows <= 0 {
		return
	}
	switch {
	case ev.XPixel > 0 && ev.YPixel > 0:
		app.cellPixW, app.cellPixH = ev.XPixel/ev.Cols, ev.YPixel/ev.Rows
	case app.cellPixW > 0 && app.cellPixH > 0:
		ev.XPixel, ev.YPixel = ev.Cols*app.cellPixW, ev.Rows*app.cellPixH
	}
	app.vx.Resize(ev)
	utils.ImageCache.SetCellPixelSize(app.cellPixW, app.cellPixH)
}

func (app *App) handleMouse(m vaxis.Mouse) {
	if app.help.visible || app.prompt != nil || app.input != nil {
		return
	}
	if h, ok := app.view.(MouseHandler); ok {
		h.HandleMouse(m)
	}
}

func (app *App) handleKeyEvent(key vaxis.Key) {
	if key.EventType == vaxis.EventRelease {
		return
	}

	if app.input != nil {
		if app.input.HandleKey(key) {
			app.input = nil
		}
		return
	}

	if app.prompt != nil {
		prompt := app.prompt
		switch {
		case key.Matches('y'), key.Matches(vaxis.KeyEnter):
			app.prompt = nil
			if prompt.OnYes != nil {
				prompt.OnYes()
			}
		case key.Matches('n'), key.Matches(vaxis.KeyEsc), key.Matches('q'):
			app.prompt = nil
			if prompt.OnNo != nil {
				prompt.OnNo()
			}
		default:
			if prompt.OnOther != nil {
				prompt.OnOther(key)
			}
		}
		return
	}

	if app.help.visible {
		app.help.HandleKey(key, app.keys)
		return
	}

	switch {
	case app.keys.Is(key, ActForceQuit):
		app.running = false
		return
	case app.keys.Is(key, ActHelp):
		app.help.Toggle()
		return
	}

	// Delegate keys to the current view
	if app.view != nil {
		app.view.HandleKey(key)
	}
}
