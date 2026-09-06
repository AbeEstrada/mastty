package tui

import (
	"context"
	"time"
)

const requestTimeout = 30 * time.Second

// load runs fetch off the main thread while the loading indicator is shown,
// then applies its result on the main thread. Errors are shown in the footer.
// It must be called from the main thread.
func load[T any](app *App, fetch func(ctx context.Context) (T, error), apply func(T)) {
	app.beginLoading()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		result, err := fetch(ctx)
		app.Sync(func() {
			app.endLoading()
			if err != nil {
				app.SetError("Request failed: " + err.Error())
				return
			}
			apply(result)
		})
	}()
}
