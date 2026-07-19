package tui

import (
	"context"
	"log"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/api"
	"github.com/AbeEstrada/tuit/auth"
	"github.com/AbeEstrada/tuit/config"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

type App struct {
	vx           *vaxis.Vaxis
	views        map[string]View
	view         View
	header       *Header
	footer       *Footer
	showQuit     bool
	running      bool
	loading      bool
	config       *config.Config
	client       *mastodon.Client
	customClient *api.Client
}

func CreateApp() (*App, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		auth.SetupAuth()
		log.Fatalf("Authentication required; re-run after setup")
	}

	vx, err := vaxis.New(vaxis.Options{})
	if err != nil {
		return nil, err
	}

	views := make(map[string]View)
	views["home"] = CreateHomeView()

	app := &App{
		vx:       vx,
		views:    views,
		view:     views["home"],
		header:   CreateHeader(),
		footer:   CreateFooter(vx),
		showQuit: false,
		running:  true,
		loading:  false,
		config:   cfg,
	}

	for _, view := range views {
		view.SetApp(app)
	}

	utils.InitImageCache(vx)

	return app, nil
}

func (app *App) SetView(name string) {
	if view, ok := app.views[name]; ok {
		app.view = view
		view.OnActivate()
	}
}

func (app *App) Run() error {
	go app.initClient()

	for app.running {
		app.draw()
		app.handleEvent()
	}
	return nil
}

func (app *App) Close() {
	app.vx.Close()
}

func (app *App) draw() {
	win := app.vx.Window()
	win.Clear()

	app.header.Draw(win)

	if app.view != nil {
		app.view.Draw(win)
	}

	app.footer.Draw(win)

	width, height := win.Size()
	separatorStyle := vaxis.Style{
		Foreground: vaxis.IndexColor(0),
	}

	for col := range width {
		win.SetCell(col+1, height-2, vaxis.Cell{
			Character: vaxis.Character{
				Grapheme: "─",
			},
			Style: separatorStyle,
		})
	}

	app.vx.Render()
}

func (app *App) SetLoading(loading bool) {
	app.loading = loading
	text := ""
	if loading {
		text = "Loading..."
	}
	app.footer.SetText(text)
	app.vx.PostEvent(vaxis.Redraw{})
}

func (app *App) initClient() {
	app.SetLoading(true)
	config := &mastodon.Config{
		Server:       app.config.Auth.Server,
		ClientID:     app.config.Auth.ClientID,
		ClientSecret: app.config.Auth.ClientSecret,
		AccessToken:  app.config.Auth.AccessToken,
	}

	client := mastodon.NewClient(config)

	if _, err := client.GetAccountCurrentUser(context.Background()); err != nil {
		app.Close()
		log.Fatalf("Failed to authenticate with Mastodon: %v", err)
	}

	app.client = client
	app.customClient = api.NewClient(client)

	if app.view != nil {
		app.view.OnActivate()
	}
	app.SetLoading(false)

	go app.fetchUnreadNotifications()
}

func (app *App) fetchUnreadNotifications() {
	count, err := app.customClient.GetNotificationsUnreadCount(context.Background())
	if err != nil {
		log.Printf("Failed to fetch unread notifications: %v", err)
		return
	}
	app.header.SetBadge(count)
	app.vx.PostEvent(vaxis.Redraw{})
}

func (app *App) handleEvent() {
	event := app.vx.PollEvent()
	if event == nil {
		return
	}
	if key, ok := event.(vaxis.Key); ok {
		app.handleKeyEvent(key)
	}
}

func (app *App) RequestQuit() {
	app.showQuit = true
	app.footer.SetText("Quit?")
	app.vx.PostEvent(vaxis.Redraw{})
}

func (app *App) handleKeyEvent(key vaxis.Key) {
	if key.EventType == vaxis.EventRelease {
		return
	}

	if app.showQuit {
		if key.Matches('y') || key.Matches(vaxis.KeyEnter) {
			app.running = false
		} else if key.Matches('n') || key.Matches(vaxis.KeyEsc) || key.Matches('q') {
			app.showQuit = false
			app.footer.SetText("")

		}
		return
	}

	// Delegate keys to the current view
	if app.view != nil {
		app.view.HandleKey(key)
	}
}
