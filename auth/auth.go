package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AbeEstrada/tuit/config"
	"github.com/AbeEstrada/tuit/constants"
	"github.com/mattn/go-mastodon"
)

// SetupAuth runs the interactive OAuth flow on the terminal, saves the
// resulting credentials, and returns the loaded config.
func SetupAuth() (*config.Config, error) {
	in := bufio.NewReader(os.Stdin)

	server, err := prompt(in, "Enter the URL of your Mastodon server: ")
	if err != nil {
		return nil, err
	}
	if server == "" {
		return nil, fmt.Errorf("no server given")
	}
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	server = strings.TrimRight(server, "/")

	appConfig := &mastodon.AppConfig{
		ClientName:   constants.AppName,
		Website:      constants.AppUrl,
		Server:       server,
		Scopes:       "read write",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
	}

	ctx := context.Background()
	app, err := mastodon.RegisterApp(ctx, appConfig)
	if err != nil {
		return nil, fmt.Errorf("registering app with %s: %w", server, err)
	}

	fmt.Println("Open this URL in your browser and authorize the app:")
	fmt.Println(app.AuthURI)

	authCode, err := prompt(in, "Paste the authorization code here: ")
	if err != nil {
		return nil, err
	}
	if authCode == "" {
		return nil, fmt.Errorf("no authorization code given")
	}

	client := mastodon.NewClient(&mastodon.Config{
		Server:       server,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
	})
	if err := client.GetUserAccessToken(ctx, authCode, app.RedirectURI); err != nil {
		return nil, fmt.Errorf("exchanging authorization code: %w", err)
	}

	cfg := &config.Config{
		Auth: config.ConfigAuth{
			Server:       client.Config.Server,
			ClientID:     client.Config.ClientID,
			ClientSecret: client.Config.ClientSecret,
			AccessToken:  client.Config.AccessToken,
		},
	}
	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	fmt.Printf("Credentials saved to %s\n", config.GetConfigFile())
	return cfg, nil
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}
