<p align="center">
  <img width="300" alt="tuit" src="https://github.com/user-attachments/assets/89ebd846-4a4a-4058-a6d0-0844dbabb92d" />
</p>

# Tuit

TUI Mastodon Client

<p align="center">
  <img width="1808" height="1064" alt="Screenshot" src="https://github.com/user-attachments/assets/270db4d3-49a6-4eae-8516-313b9fad15f1" />
</p>

<p align="center">
  <img width="1810" height="1036" src="https://github.com/user-attachments/assets/7be999fd-375c-4ba4-99ec-10e80dae4063" />
</p>

## Build and Installation

This project uses [`just`](https://github.com/casey/just) as a command runner and requires [Go](https://golang.org/) for building. Below are the available commands:

### Available Commands

- `just` or `just install` - Build and install the binary to `PREFIX/bin/` (default: `/usr/local/bin`)
- `just build` - Build the binary in the current directory
- `just uninstall` - Remove the installed binary
- `just clean` - Remove the built binary from the current directory

## Usage

```sh
tuit
```

## Authentication & Config

### First Time Setup

On first run, the app will guide you through Mastodon authentication:

```bash
$ tuit
Enter the URL of your Mastodon server: mastodon.social
Open your browser and visit the authorization URL...
Paste the code here: [paste-code-from-browser]
```

### Config Locations

Credentials are automatically saved to:

| OS          | Location                                         |
| ----------- | ------------------------------------------------ |
| **Linux**   | `~/.config/tuit/config.json`                     |
| **macOS**   | `~/Library/Application Support/tuit/config.json` |
| **Windows** | `%APPDATA%\tuit\config.json`                     |

### How It Works

1. First run → OAuth2 flow with Mastodon
2. Credentials saved to OS config directory
3. Subsequent runs → Auto load from config file

The app handles all authentication automatically and stores your access token for future use.

### Security Notes

- Credentials are stored in plain text in the user's config directory
- The config directory has permissions set to `0755` (readable by user only on Unix systems)
- Access tokens have "read write" scope for full Mastodon functionality
- Users should protect their config directory from unauthorized access

### Troubleshooting

If authentication fails:

1. Delete the config file and restart the app to re-authenticate
2. Check that the Mastodon server URL is correct and accessible
3. Verify the authorization code was copied correctly during setup

## Keybindings

### Global

| Key | Action                                        |
| --- | --------------------------------------------- |
| `q` | Show quit confirmation / Close current thread |

### Navigation

| Key   | Action                              |
| ----- | ----------------------------------- |
| `Tab` | Switch between left and right views |
| `h`   | Focus timeline view                 |
| `l`   | Focus status view                   |
| `r`   | Reload home timeline                |
| `u`   | Go to user timeline                 |
| `t`   | Go to thread                        |
| `q`   | Quit / Remove thread view           |

### Timeline

| Key      | Action                                                |
| -------- | ----------------------------------------------------- |
| `j`      | Move to next status                                   |
| `k`      | Move to previous status                               |
| `ctrl+d` | Scroll down one page                                  |
| `ctrl+u` | Scroll up one page                                    |
| `g`      | Jump to first status                                  |
| `G`      | Jump to last status                                   |
| `O`      | Open status with original URL in browser              |
| `o`      | Open status in current server instance URL in browser |
| `v`      | Open card URL in browser                              |
