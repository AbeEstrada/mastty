package tui

import (
	"strings"

	"git.sr.ht/~rockorager/vaxis"
)

// Action names a user-triggerable operation. The string value is what users
// write in the "keys" preference to rebind it.
type Action string

const (
	ActHelp          Action = "help"
	ActQuit          Action = "quit"
	ActForceQuit     Action = "force_quit"
	ActBack          Action = "back"
	ActFocusNext     Action = "focus_next"
	ActFocusTimeline Action = "focus_timeline"
	ActFocusDetail   Action = "focus_detail"
	ActReload        Action = "reload"
	ActThread        Action = "thread"
	ActProfile       Action = "profile"
	ActOwnProfile    Action = "own_profile"
	ActLinks         Action = "links"
	ActToggleCW      Action = "toggle_cw"
	ActYank          Action = "yank"
	ActOpenOriginal  Action = "open_original"
	ActOpenLocal     Action = "open_local"
	ActOpenLink      Action = "open_link"
	ActDown          Action = "down"
	ActUp            Action = "up"
	ActPageDown      Action = "page_down"
	ActPageUp        Action = "page_up"
	ActTop           Action = "top"
	ActBottom        Action = "bottom"
	ActSelect        Action = "select"
	ActHome          Action = "home"
	ActNotifications Action = "notifications"
	ActLocal         Action = "local"
	ActFederated     Action = "federated"
	ActBookmarks     Action = "bookmarks"
	ActFavourites    Action = "favourites"
	ActLists         Action = "lists"
	ActHashtag       Action = "hashtag"
	ActSearch        Action = "search"
	ActFavourite     Action = "favourite"
	ActBoost         Action = "boost"
	ActBookmark      Action = "bookmark"
	ActFollow        Action = "follow"
	ActVote          Action = "vote"
	ActCompose       Action = "compose"
	ActReply         Action = "reply"
)

var defaultKeys = map[Action][]string{
	ActHelp:          {"?"},
	ActQuit:          {"q"},
	ActForceQuit:     {"Ctrl+c"},
	ActBack:          {"Esc"},
	ActFocusNext:     {"Tab"},
	ActFocusTimeline: {"h"},
	ActFocusDetail:   {"l"},
	ActReload:        {"r"},
	ActThread:        {"t"},
	ActProfile:       {"u"},
	ActOwnProfile:    {"U"},
	ActLinks:         {"i"},
	ActToggleCW:      {"x"},
	ActYank:          {"y"},
	ActOpenOriginal:  {"O"},
	ActOpenLocal:     {"o"},
	ActOpenLink:      {"v"},
	ActDown:          {"j"},
	ActUp:            {"k"},
	ActPageDown:      {"Ctrl+d"},
	ActPageUp:        {"Ctrl+u"},
	ActTop:           {"g"},
	ActBottom:        {"G"},
	ActSelect:        {"Enter"},
	ActHome:          {"1"},
	ActNotifications: {"2"},
	ActLocal:         {"3"},
	ActFederated:     {"4"},
	ActBookmarks:     {"5"},
	ActFavourites:    {"6"},
	ActLists:         {"L"},
	ActHashtag:       {"#"},
	ActSearch:        {"/"},
	ActFavourite:     {"f"},
	ActBoost:         {"b"},
	ActBookmark:      {"s"},
	ActFollow:        {"F"},
	ActVote:          {"p"},
	ActCompose:       {"c"},
	ActReply:         {"R"},
}

// HelpEntry is one row of the help overlay. Keys is a display override for
// entries that are not bound through an Action (such as number keys).
type HelpEntry struct {
	Context string
	Action  Action
	Desc    string
	Keys    string
}

var helpEntries = []HelpEntry{
	{"Global", ActHelp, "Show or hide this help", ""},
	{"Global", ActQuit, "Close the current thread or profile; at the home timeline, ask to quit", ""},
	{"Global", ActForceQuit, "Quit immediately", ""},
	{"Global", ActBack, "Close the current thread, profile, or links view", ""},
	{"Global", ActFocusNext, "Switch focus between the timeline and the detail pane", ""},
	{"Global", ActFocusTimeline, "Focus the timeline", ""},
	{"Global", ActFocusDetail, "Focus the detail pane", ""},
	{"Global", ActReload, "Fetch new statuses for the home timeline", ""},
	{"Global", ActThread, "Open the thread of the selected status", ""},
	{"Global", ActProfile, "Open the profile of the selected status's author", ""},
	{"Global", ActOwnProfile, "Open your own profile", ""},
	{"Global", ActLinks, "List the links and media of the selected status", ""},
	{"Global", ActToggleCW, "Show or hide content behind a content warning or sensitive media", ""},
	{"Global", ActYank, "Copy the selected status or link URL to the clipboard", ""},
	{"Global", ActOpenOriginal, "Open the status on its original server in the browser", ""},
	{"Global", ActOpenLocal, "Open the status on your server in the browser", ""},
	{"Global", ActOpenLink, "Open the link card, or the first link in the status", ""},
	{"Timelines", ActHome, "Home timeline", ""},
	{"Timelines", ActNotifications, "Notifications (opening them marks them read)", ""},
	{"Timelines", ActLocal, "Local timeline", ""},
	{"Timelines", ActFederated, "Federated timeline", ""},
	{"Timelines", ActBookmarks, "Your bookmarks", ""},
	{"Timelines", ActFavourites, "Your favourites", ""},
	{"Timelines", ActLists, "Choose one of your lists", ""},
	{"Timelines", ActHashtag, "Open a hashtag timeline", ""},
	{"Timelines", ActSearch, "Search statuses and accounts", ""},
	{"Actions", ActFavourite, "Favourite or unfavourite the selected status", ""},
	{"Actions", ActBoost, "Boost or unboost the selected status", ""},
	{"Actions", ActBookmark, "Bookmark or unbookmark the selected status", ""},
	{"Actions", ActFollow, "Follow or unfollow the selected account or author", ""},
	{"Actions", ActVote, "Vote in the selected status's poll", ""},
	{"Actions", ActCompose, "Write a new post in your editor", ""},
	{"Actions", ActReply, "Reply to the selected status in your editor", ""},
	{"Timeline", ActDown, "Next status (loads more at the end)", ""},
	{"Timeline", ActUp, "Previous status", ""},
	{"Timeline", ActPageDown, "Down half a page", ""},
	{"Timeline", ActPageUp, "Up half a page", ""},
	{"Timeline", ActTop, "First status", ""},
	{"Timeline", ActBottom, "Last status", ""},
	{"Detail pane", ActDown, "Scroll down", ""},
	{"Detail pane", ActUp, "Scroll up", ""},
	{"Detail pane", ActPageDown, "Scroll down half a page", ""},
	{"Detail pane", ActPageUp, "Scroll up half a page", ""},
	{"Detail pane", ActTop, "Jump to the top", ""},
	{"Detail pane", ActBottom, "Jump to the bottom", ""},
	{"Links view", ActDown, "Next link", ""},
	{"Links view", ActUp, "Previous link", ""},
	{"Links view", ActSelect, "Open the selected link", ""},
	{"Links view", "", "Open a link by number", "1..9"},
	{"Links view", ActYank, "Copy the selected link", ""},
	{"Links view", ActQuit, "Close the links view", ""},
	{"Mouse", "", "Select a status or focus a pane", "click"},
	{"Mouse", "", "Move through the timeline or scroll the detail pane", "wheel"},
}

// Keymap resolves keys to actions, starting from the defaults and applying the
// user's overrides from the "keys" preference.
type Keymap struct {
	keys map[Action][]string
}

func NewKeymap(overrides map[string]string) *Keymap {
	km := &Keymap{keys: make(map[Action][]string, len(defaultKeys))}
	for action, keys := range defaultKeys {
		km.keys[action] = append([]string(nil), keys...)
	}
	for name, spec := range overrides {
		action := Action(strings.TrimSpace(name))
		if _, known := defaultKeys[action]; !known {
			continue
		}
		var keys []string
		for _, k := range strings.Split(spec, ",") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			km.keys[action] = keys
		}
	}
	return km
}

// Is reports whether key triggers any of the given actions.
func (km *Keymap) Is(key vaxis.Key, actions ...Action) bool {
	for _, action := range actions {
		for _, name := range km.keys[action] {
			if matchKeyName(key, name) {
				return true
			}
		}
	}
	return false
}

// Keys returns the bound keys of an action for display, e.g. "q" or "q / Esc".
func (km *Keymap) Keys(action Action) string {
	return strings.Join(km.keys[action], " / ")
}

// Entries returns the help rows with their current key labels.
func (km *Keymap) Entries() []HelpEntry {
	entries := make([]HelpEntry, 0, len(helpEntries))
	for _, e := range helpEntries {
		if e.Keys == "" {
			e.Keys = km.Keys(e.Action)
		}
		entries = append(entries, e)
	}
	return entries
}

func matchKeyName(key vaxis.Key, name string) bool {
	switch strings.ToLower(name) {
	case "enter", "return":
		return key.Matches(vaxis.KeyEnter)
	case "esc", "escape":
		return key.Matches(vaxis.KeyEsc)
	case "tab":
		return key.Matches(vaxis.KeyTab)
	case "space":
		return key.Matches(' ')
	case "up":
		return key.Matches(vaxis.KeyUp)
	case "down":
		return key.Matches(vaxis.KeyDown)
	case "left":
		return key.Matches(vaxis.KeyLeft)
	case "right":
		return key.Matches(vaxis.KeyRight)
	case "pgup", "pageup":
		return key.Matches(vaxis.KeyPgUp)
	case "pgdown", "pagedown":
		return key.Matches(vaxis.KeyPgDown)
	case "home":
		return key.Matches(vaxis.KeyHome)
	case "end":
		return key.Matches(vaxis.KeyEnd)
	}
	return key.MatchString(name)
}
