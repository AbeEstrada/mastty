package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AbeEstrada/tuit/constants"
)

type ConfigAuth struct {
	Server       string `json:"server"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AccessToken  string `json:"access_token"`
}

// Preferences holds user-tunable behavior. Missing fields fall back to the
// defaults applied by applyDefaults, so a config without a "preferences" block
// keeps working.
type Preferences struct {
	// SplitRatio is "left:right" for the timeline and detail panes, e.g. "2:3".
	SplitRatio string `json:"split_ratio"`
	// ShowImages turns image rendering off entirely when false.
	ShowImages *bool `json:"show_images,omitempty"`
	// ExpandCW shows content behind content warnings without pressing x.
	ExpandCW bool `json:"expand_cw"`
	// ShowSensitiveMedia shows sensitive media without pressing x.
	ShowSensitiveMedia bool `json:"show_sensitive_media"`
	// Timestamp is "absolute" (2006-01-02 15:04, the default) or "relative" (5m, 2h, 3d).
	Timestamp string `json:"timestamp"`
	// DefaultVisibility applies to new posts: public, unlisted, private, direct.
	DefaultVisibility string `json:"default_visibility"`
	// Editor overrides $VISUAL and $EDITOR for composing posts.
	Editor string `json:"editor"`
	// DesktopNotifications raises a terminal notification on new mentions.
	DesktopNotifications bool `json:"desktop_notifications"`
	// ConfirmBoost asks before boosting or unboosting.
	ConfirmBoost *bool `json:"confirm_boost,omitempty"`
	// Keys overrides default keybindings by action name, e.g. {"down": "n"}.
	// Several keys can be listed separated by commas.
	Keys map[string]string `json:"keys,omitempty"`
}

type Config struct {
	Auth        ConfigAuth  `json:"auth"`
	Preferences Preferences `json:"preferences"`
}

var configDirName = strings.ToLower(constants.AppName)
var configFileName = "config.json"

func boolPtr(b bool) *bool { return &b }

func (p *Preferences) applyDefaults() {
	if p.SplitRatio == "" {
		p.SplitRatio = "2:3"
	}
	if p.ShowImages == nil {
		p.ShowImages = boolPtr(true)
	}
	if p.Timestamp == "" {
		p.Timestamp = "absolute"
	}
	if p.DefaultVisibility == "" {
		p.DefaultVisibility = "public"
	}
	if p.ConfirmBoost == nil {
		p.ConfirmBoost = boolPtr(true)
	}
}

// ImagesEnabled reports whether images should be rendered.
func (p Preferences) ImagesEnabled() bool {
	return p.ShowImages == nil || *p.ShowImages
}

// BoostNeedsConfirm reports whether boosting asks for confirmation.
func (p Preferences) BoostNeedsConfirm() bool {
	return p.ConfirmBoost == nil || *p.ConfirmBoost
}

// Split returns the pane ratio, falling back to 2:3 on malformed input.
func (p Preferences) Split() (left, right int) {
	parts := strings.SplitN(p.SplitRatio, ":", 2)
	if len(parts) == 2 {
		l, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		r, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil && l > 0 && r > 0 {
			return l, r
		}
	}
	return 2, 3
}

// GetConfigDir returns the directory holding config.json. On Windows this is
// %APPDATA%\tuit; everywhere else it honors $XDG_CONFIG_HOME and falls back to
// ~/.config/tuit (macOS included).
func GetConfigDir() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, configDirName)
		}
	}

	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, configDirName)
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".config", configDirName)
	}

	// Cannot determine config directory, using current directory
	return filepath.Join(".", configDirName)
}

func GetConfigFile() string {
	configDir := GetConfigDir()
	return filepath.Join(configDir, configFileName)
}

func LoadConfig() (*Config, error) {
	configFile := GetConfigFile()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", configFile, err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON from %s: %w", configFile, err)
	}
	config.Preferences.applyDefaults()

	// The file holds the access token and client secret. Older versions wrote it
	// world readable; tighten it on the way in (best effort).
	if runtime.GOOS != "windows" {
		_ = os.Chmod(configFile, 0o600)
	}

	return &config, nil
}

// Save writes the config with owner-only permissions, creating the directory
// when needed. Defaults are filled in so the file documents every preference.
func Save(cfg *Config) error {
	cfg.Preferences.applyDefaults()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("error encoding config: %w", err)
	}
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("error creating directory %s: %w", dir, err)
	}
	file := GetConfigFile()
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("error writing %s: %w", file, err)
	}
	return nil
}
