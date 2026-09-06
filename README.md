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

## Features

- Home, local, federated, list, hashtag, bookmark, favourite, and notification timelines, plus search
- Live updates over the streaming API with automatic reconnection and backfill
- Threads with reply indentation, profiles with custom fields and follow state
- Images and avatars through the kitty graphics protocol or sixel, with block-character fallback and alt text
- Content warnings and sensitive media hidden until you ask
- Favourite, boost, bookmark, follow, vote in polls
- Compose and reply in your own editor
- Links view with numbered shortcuts, copy to clipboard, open in browser
- Keyboard driven with a built-in help screen, mouse supported

## Build and Installation

This project uses [`just`](https://github.com/casey/just) as a command runner and requires [Go](https://golang.org/) for building.

- `just` or `just install` builds and installs the binary to `PREFIX/bin/` (default: `/usr/local/bin`)
- `just build` builds the binary in the current directory
- `just test` runs the unit tests
- `just lint` checks formatting and runs `go vet`
- `just race` builds a race-detector binary for manual testing
- `just uninstall` removes the installed binary
- `just clean` removes built binaries

## Usage

```sh
tuit
```

Press `?` at any time for the list of keybindings.

## Authentication and Config

### First Time Setup

On first run, the app guides you through Mastodon authentication and then starts:

```
$ tuit
Enter the URL of your Mastodon server: mastodon.social
Open this URL in your browser and authorize the app:
https://mastodon.social/oauth/authorize?...
Paste the authorization code here: [paste-code-from-browser]
Credentials saved to /Users/you/.config/tuit/config.json
```

### Config Locations

| OS          | Location                                                        |
| ----------- | --------------------------------------------------------------- |
| **Linux**   | `$XDG_CONFIG_HOME/tuit/config.json` or `~/.config/tuit/config.json` |
| **macOS**   | `$XDG_CONFIG_HOME/tuit/config.json` or `~/.config/tuit/config.json` |
| **Windows** | `%APPDATA%\tuit\config.json`                                    |

### Preferences

The `preferences` block in `config.json` is optional; missing fields use the defaults shown here.

```json
{
    "auth": { "...": "..." },
    "preferences": {
        "split_ratio": "2:3",
        "show_images": true,
        "expand_cw": false,
        "show_sensitive_media": false,
        "timestamp": "absolute",
        "default_visibility": "public",
        "editor": "",
        "desktop_notifications": false,
        "confirm_boost": true,
        "keys": {
            "down": "j,Down",
            "up": "k,Up"
        }
    }
}
```

- `split_ratio` sets the width of the timeline versus the detail pane
- `show_images` turns image rendering off when false
- `timestamp` is `absolute` (`2006-01-02 15:04`, the default) or `relative` (`5m`, `2h`, `3d`)
- `default_visibility` applies to new posts: `public`, `unlisted`, `private`, or `direct`; replies inherit the visibility of the post they answer
- `editor` overrides `$VISUAL` and `$EDITOR` for composing
- `desktop_notifications` raises a terminal notification when you are mentioned
- `confirm_boost` asks before boosting or removing a boost
- `keys` rebinds actions; the action names are the ones listed in `?` and below, and several keys can be given separated by commas (`Ctrl+d`, `Enter`, `Esc`, `Tab`, `Space`, `Up`, `Down`, `PgUp`, `PgDown`, `Home`, `End` are understood)

### Security Notes

- Credentials are stored in plain text in the config file
- The config directory is created with mode `0700` and the file with `0600` (owner only); existing files are tightened on load
- The access token has `read write` scope, which favourites, boosts, follows, votes, and posts need

### Troubleshooting

If authentication fails, the error is printed to the terminal on exit. To start over, delete the config file and run `tuit` again.

Set `TUIT_DEBUG=1` to write a debug log to `debug.log` in the user cache directory (`~/Library/Caches/tuit/` on macOS, `~/.cache/tuit/` on Linux).

## Keybindings

Action names in parentheses are what the `keys` preference accepts.

### Global

| Key      | Action                                                                  |
| -------- | ----------------------------------------------------------------------- |
| `?`      | Show or hide the help (`help`)                                          |
| `q`      | Close the current thread or profile; at a root timeline, ask to quit (`quit`) |
| `Esc`    | Close the current thread, profile, or links view (`back`)               |
| `Ctrl+c` | Quit immediately (`force_quit`)                                         |
| `Tab`    | Switch focus between the timeline and the detail pane (`focus_next`)    |
| `h`      | Focus the timeline (`focus_timeline`)                                   |
| `l`      | Focus the detail pane (`focus_detail`)                                  |
| `r`      | Fetch newer items for the current timeline (`reload`)                   |
| `t`      | Open the thread of the selected status (`thread`)                       |
| `u`      | Open the profile of the selected status's author (`profile`)            |
| `U`      | Open your own profile (`own_profile`)                                   |
| `i`      | List the links and media of the selected status (`links`)               |
| `x`      | Show or hide content behind a content warning or sensitive media (`toggle_cw`) |
| `y`      | Copy the selected status or link URL to the clipboard (`yank`)          |
| `O`      | Open the status on its original server in the browser (`open_original`) |
| `o`      | Open the status on your server in the browser (`open_local`)            |
| `v`      | Open the link card, or the first link in the status (`open_link`)       |

### Timelines

| Key | Action                                                   |
| --- | -------------------------------------------------------- |
| `1` | Home (`home`)                                            |
| `2` | Notifications; opening them marks them read (`notifications`) |
| `3` | Local timeline (`local`)                                 |
| `4` | Federated timeline (`federated`)                         |
| `5` | Your bookmarks (`bookmarks`)                             |
| `6` | Your favourites (`favourites`)                           |
| `L` | Choose one of your lists (`lists`)                       |
| `#` | Open a hashtag timeline (`hashtag`)                      |
| `/` | Search statuses and accounts (`search`)                  |

### Actions

| Key | Action                                                      |
| --- | ----------------------------------------------------------- |
| `f` | Favourite or unfavourite the selected status (`favourite`)  |
| `b` | Boost or unboost the selected status (`boost`)              |
| `s` | Bookmark or unbookmark the selected status (`bookmark`)     |
| `F` | Follow or unfollow the selected account or author (`follow`) |
| `p` | Vote in the selected status's poll (`vote`)                 |
| `c` | Write a new post in your editor (`compose`)                 |
| `R` | Reply to the selected status in your editor (`reply`)       |

### Timeline pane

| Key      | Action                                    |
| -------- | ----------------------------------------- |
| `j`      | Next status; loads more at the end (`down`) |
| `k`      | Previous status (`up`)                    |
| `Ctrl+d` | Down half a page (`page_down`)            |
| `Ctrl+u` | Up half a page (`page_up`)                |
| `g`      | First status (`top`)                      |
| `G`      | Last status (`bottom`)                    |

### Detail pane

The same keys scroll the detail pane when it has focus.

### Links view

| Key       | Action                              |
| --------- | ----------------------------------- |
| `j`/`k`   | Select next or previous link        |
| `Enter`   | Open the selected link (`select`)   |
| `1`..`9`  | Open a link by number               |
| `y`       | Copy the selected link              |
| `q`/`Esc` | Close the links view                |

### Mouse

Click a row to select it or a pane to focus it. The wheel moves through the timeline or scrolls the detail pane.

## Composing

`c` opens your editor (`preferences.editor`, then `$VISUAL`, then `$EDITOR`, then `vi`) on an empty file; `R` prefills the mentions of the selected status and inherits its visibility and content warning. Save and quit to continue, or quit with an empty file to discard the post. Tuit then asks for an optional content warning and shows the character count and visibility; press `v` to cycle the visibility and `y` to post.
