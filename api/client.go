package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mattn/go-mastodon"
)

// Client extends go-mastodon with endpoints it does not expose.
type Client struct {
	*mastodon.Client
}

func NewClient(client *mastodon.Client) *Client {
	return &Client{Client: client}
}

type UnreadCount struct {
	Count int `json:"count"`
}

func (c *Client) newRequest(ctx context.Context, method, apiPath string, body string) (*http.Request, error) {
	u, err := url.Parse(c.Config.Server)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath(apiPath)

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Config.AccessToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req, nil
}

// GetNotificationsUnreadCount returns how many notifications are newer than
// the user's notification read marker (Mastodon 4.3 and later).
func (c *Client) GetNotificationsUnreadCount(ctx context.Context) (int, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "api/v1/notifications/unread_count", "")
	if err != nil {
		return 0, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result UnreadCount
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.Count, nil
}

// SetNotificationsMarker records the newest notification the user has seen so
// the server's unread count resets.
func (c *Client) SetNotificationsMarker(ctx context.Context, lastReadID mastodon.ID) error {
	form := url.Values{}
	form.Set("notifications[last_read_id]", string(lastReadID))
	req, err := c.newRequest(ctx, http.MethodPost, "api/v1/markers", form.Encode())
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
