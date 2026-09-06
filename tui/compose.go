package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/mattn/go-mastodon"
)

var visibilities = []string{"public", "unlisted", "private", "direct"}

func nextVisibility(current string) string {
	for i, v := range visibilities {
		if v == current {
			return visibilities[(i+1)%len(visibilities)]
		}
	}
	return visibilities[0]
}

// composeDraft holds what the user is about to post.
type composeDraft struct {
	text       string
	inReplyTo  *mastodon.Status
	visibility string
	spoiler    string
}

// EditText suspends the UI, opens the user's editor on a temporary file that
// holds initial, and returns the edited text once the editor exits.
func (app *App) EditText(initial string) (string, error) {
	editor := strings.TrimSpace(app.config.Preferences.Editor)
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	args := strings.Fields(editor)

	file, err := os.CreateTemp("", "tuit-post-*.txt")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(initial); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	if err := app.vx.Suspend(); err != nil {
		return "", err
	}
	cmd := exec.Command(args[0], append(args[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := cmd.Run()
	resumeErr := app.vx.Resume()
	if runErr != nil {
		return "", fmt.Errorf("editor %q: %w", editor, runErr)
	}
	if resumeErr != nil {
		return "", resumeErr
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// replyPrefix lists the accounts a reply should mention: the author and
// everyone already mentioned, except the user.
func (v *HomeView) replyPrefix(status *mastodon.Status) string {
	var self string
	if v.app.currentAccount != nil {
		self = v.app.currentAccount.Acct
	}
	seen := map[string]bool{self: true}
	var handles []string
	add := func(acct string) {
		if acct == "" || seen[acct] {
			return
		}
		seen[acct] = true
		handles = append(handles, "@"+acct)
	}
	add(status.Account.Acct)
	for _, m := range status.Mentions {
		add(m.Acct)
	}
	if len(handles) == 0 {
		return ""
	}
	return strings.Join(handles, " ") + " "
}

// compose writes a new post, or a reply to the selected status, in the user's
// editor and then asks for a content warning and confirmation.
func (v *HomeView) compose(reply bool) {
	draft := &composeDraft{visibility: v.app.config.Preferences.DefaultVisibility}
	if reply {
		status := v.selectedStatus()
		if status == nil {
			v.app.Flash("Select a status to reply to")
			return
		}
		original := originalStatus(status)
		draft.inReplyTo = original
		draft.visibility = original.Visibility
		draft.spoiler = original.SpoilerText
		draft.text = v.replyPrefix(original)
	}

	text, err := v.app.EditText(draft.text)
	if err != nil {
		v.app.SetError(err.Error())
		return
	}
	text = strings.TrimRight(text, " \t\r\n")
	if strings.TrimSpace(text) == "" || text == strings.TrimSpace(draft.text) {
		v.app.Flash("Post discarded")
		return
	}
	draft.text = text

	prompt := v.app.PromptInput("Content warning (Enter for none): ", draft.spoiler, func(cw string) {
		draft.spoiler = strings.TrimSpace(cw)
		v.confirmPost(draft)
	})
	prompt.onCancel = func() { v.app.Flash("Post discarded") }
}

func (v *HomeView) confirmPost(draft *composeDraft) {
	count := utf8.RuneCountInString(draft.text)
	prompt := &Prompt{}
	refresh := func() {
		what := "Post"
		if draft.inReplyTo != nil {
			what = "Reply"
		}
		prompt.Text = fmt.Sprintf("%s %d characters as %s? v cycles visibility", what, count, draft.visibility)
	}
	refresh()
	prompt.OnYes = func() { v.submitPost(draft) }
	prompt.OnNo = func() { v.app.Flash("Post discarded") }
	prompt.OnOther = func(key vaxis.Key) bool {
		if key.Matches('v') {
			draft.visibility = nextVisibility(draft.visibility)
			refresh()
			return true
		}
		return false
	}
	v.app.prompt = prompt
}

func (v *HomeView) submitPost(draft *composeDraft) {
	client := v.app.client
	toot := &mastodon.Toot{
		Status:      draft.text,
		Visibility:  draft.visibility,
		SpoilerText: draft.spoiler,
		Sensitive:   draft.spoiler != "",
	}
	if draft.inReplyTo != nil {
		toot.InReplyToID = draft.inReplyTo.ID
	}

	load(v.app, func(ctx context.Context) (*mastodon.Status, error) {
		return client.PostStatus(ctx, toot)
	}, func(posted *mastodon.Status) {
		if posted == nil {
			return
		}
		if home := v.roots[titleHome]; home != nil && posted.Visibility != "direct" {
			home.Prepend([]TimelineItem{StatusItem{Status: posted}})
		}
		if draft.inReplyTo != nil {
			parent := *draft.inReplyTo
			parent.RepliesCount++
			v.replaceStatusEverywhere(&parent)
			if current := v.timeline.Current(); current != nil {
				if _, isThread := current.Source.(*threadSource); isThread {
					current.Append([]TimelineItem{StatusItem{Status: posted}})
				}
			}
		}
		v.app.Flash("Posted")
	})
}
