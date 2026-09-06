package tui

import (
	"context"
	"fmt"
	"log"
	"sort"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/mattn/go-mastodon"
)

// selectedAccount returns the account behind the selection: the profile item,
// the author of the status, or the actor of a notification.
func (v *HomeView) selectedAccount() *mastodon.Account {
	switch item := v.timeline.SelectedItem().(type) {
	case AccountItem:
		return item.Account
	case StatusItem:
		if item.Status != nil {
			account := item.Original().Account
			return &account
		}
	case NotificationItem:
		if item.Notification != nil {
			account := item.Account
			return &account
		}
	}
	return nil
}

// mutateStatus shows an optimistic version of a status right away, runs the
// API call in the background, and applies the server's answer, reverting to
// the original when the call fails.
func (v *HomeView) mutateStatus(original, optimistic *mastodon.Status, call func(ctx context.Context) (*mastodon.Status, error), done, failed string) {
	v.replaceStatusEverywhere(optimistic)
	id := original.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		updated, err := call(ctx)
		v.app.Sync(func() {
			if err != nil {
				v.replaceStatusEverywhere(original)
				v.app.SetError(failed + ": " + err.Error())
				return
			}
			// Boosting returns the boost itself, which wraps the status we act on.
			if updated != nil && updated.Reblog != nil && updated.Reblog.ID == id {
				updated = updated.Reblog
			}
			if updated != nil && updated.ID == id {
				v.replaceStatusEverywhere(updated)
			}
			v.app.Flash(done)
		})
	}()
}

func (v *HomeView) toggleFavourite() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	client := v.app.client
	favourited := boolField(original.Favourited)

	optimistic := *original
	optimistic.Favourited = !favourited
	if favourited {
		optimistic.FavouritesCount = max(0, original.FavouritesCount-1)
	} else {
		optimistic.FavouritesCount = original.FavouritesCount + 1
	}

	if favourited {
		v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
			return client.Unfavourite(ctx, original.ID)
		}, "Favourite removed", "Could not remove favourite")
		return
	}
	v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
		return client.Favourite(ctx, original.ID)
	}, "Favourited", "Could not favourite")
}

func (v *HomeView) toggleBoost() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	client := v.app.client
	boosted := boolField(original.Reblogged)
	own := v.app.currentAccount != nil && original.Account.ID == v.app.currentAccount.ID
	if !boosted && !own && (original.Visibility == "private" || original.Visibility == "direct") {
		v.app.Flash("This post cannot be boosted")
		return
	}

	run := func() {
		optimistic := *original
		optimistic.Reblogged = !boosted
		if boosted {
			optimistic.ReblogsCount = max(0, original.ReblogsCount-1)
		} else {
			optimistic.ReblogsCount = original.ReblogsCount + 1
		}
		if boosted {
			v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
				return client.Unreblog(ctx, original.ID)
			}, "Boost removed", "Could not remove boost")
			return
		}
		v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
			return client.Reblog(ctx, original.ID)
		}, "Boosted", "Could not boost")
	}

	if !v.app.config.Preferences.BoostNeedsConfirm() {
		run()
		return
	}
	question := fmt.Sprintf("Boost @%s's post?", original.Account.Acct)
	if boosted {
		question = "Remove your boost?"
	}
	v.app.Confirm(question, run)
}

func (v *HomeView) toggleBookmark() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	client := v.app.client
	bookmarked := boolField(original.Bookmarked)

	optimistic := *original
	optimistic.Bookmarked = !bookmarked

	if bookmarked {
		v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
			return client.Unbookmark(ctx, original.ID)
		}, "Bookmark removed", "Could not remove bookmark")
		return
	}
	v.mutateStatus(original, &optimistic, func(ctx context.Context) (*mastodon.Status, error) {
		return client.Bookmark(ctx, original.ID)
	}, "Bookmarked", "Could not bookmark")
}

// fetchRelationship loads how the user relates to an account so the profile
// view can show it.
func (v *HomeView) fetchRelationship(id mastodon.ID) {
	if v.app.currentAccount != nil && id == v.app.currentAccount.ID {
		return
	}
	client := v.app.client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		rels, err := client.GetAccountRelationships(ctx, []string{string(id)})
		if err != nil || len(rels) == 0 {
			log.Printf("relationship lookup failed for %s: %v", id, err)
			return
		}
		v.app.Sync(func() { v.relationships[id] = rels[0] })
	}()
}

// toggleFollow follows the selected account, or asks before unfollowing.
func (v *HomeView) toggleFollow() {
	account := v.selectedAccount()
	if account == nil {
		return
	}
	if v.app.currentAccount != nil && account.ID == v.app.currentAccount.ID {
		v.app.Flash("That is you")
		return
	}
	client := v.app.client
	id := account.ID
	acct := account.Acct

	apply := func(call func(ctx context.Context) (*mastodon.Relationship, error), done, failed string) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()
			rel, err := call(ctx)
			v.app.Sync(func() {
				if err != nil {
					v.app.SetError(failed + ": " + err.Error())
					return
				}
				v.relationships[id] = rel
				if rel.Requested {
					v.app.Flash("Follow requested")
					return
				}
				v.app.Flash(done)
			})
		}()
	}

	decide := func(rel *mastodon.Relationship) {
		if rel != nil && (rel.Following || rel.Requested) {
			v.app.Confirm(fmt.Sprintf("Unfollow @%s?", acct), func() {
				apply(func(ctx context.Context) (*mastodon.Relationship, error) {
					return client.AccountUnfollow(ctx, id)
				}, "Unfollowed @"+acct, "Could not unfollow")
			})
			return
		}
		apply(func(ctx context.Context) (*mastodon.Relationship, error) {
			return client.AccountFollow(ctx, id)
		}, "Following @"+acct, "Could not follow")
	}

	if rel, ok := v.relationships[id]; ok {
		decide(rel)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		rels, err := client.GetAccountRelationships(ctx, []string{string(id)})
		v.app.Sync(func() {
			if err != nil || len(rels) == 0 {
				v.app.SetError(fmt.Sprintf("Could not look up @%s: %v", acct, err))
				return
			}
			v.relationships[id] = rels[0]
			decide(rels[0])
		})
	}()
}

// pollVote is the in-progress selection while voting on a poll.
type pollVote struct {
	status  *mastodon.Status
	choices map[int]bool
}

func (v *HomeView) startVote() {
	status := v.selectedStatus()
	if status == nil {
		return
	}
	original := originalStatus(status)
	poll := original.Poll
	switch {
	case poll == nil:
		v.app.Flash("No poll in this status")
		return
	case poll.Expired:
		v.app.Flash("This poll has ended")
		return
	case poll.Voted:
		v.app.Flash("You already voted in this poll")
		return
	}
	v.vote = &pollVote{status: original, choices: make(map[int]bool)}
	v.statusView.SetVoteSelection(v.vote.choices)
	mode := "pick one"
	if poll.Multiple {
		mode = "toggle choices"
	}
	v.app.SetHint(fmt.Sprintf("Vote: 1-%d %s · Enter submits · Esc cancels", len(poll.Options), mode))
}

func (v *HomeView) endVote() {
	v.vote = nil
	v.statusView.SetVoteSelection(nil)
	v.app.SetHint("")
}

func (v *HomeView) handleVoteKey(key vaxis.Key) {
	vote := v.vote
	poll := vote.status.Poll
	switch {
	case key.Matches(vaxis.KeyEsc), v.app.keys.Is(key, ActQuit):
		v.endVote()
	case key.Matches(vaxis.KeyEnter):
		v.submitVote()
	default:
		if key.Keycode < '1' || key.Keycode > '9' {
			return
		}
		index := int(key.Keycode - '1')
		if index >= len(poll.Options) {
			return
		}
		if !poll.Multiple {
			for k := range vote.choices {
				delete(vote.choices, k)
			}
		}
		vote.choices[index] = !vote.choices[index]
		if !vote.choices[index] {
			delete(vote.choices, index)
		}
	}
}

func (v *HomeView) submitVote() {
	vote := v.vote
	if vote == nil {
		return
	}
	choices := make([]int, 0, len(vote.choices))
	for index := range vote.choices {
		choices = append(choices, index)
	}
	if len(choices) == 0 {
		v.app.Flash("Choose an option first")
		return
	}
	sort.Ints(choices)
	status := vote.status
	client := v.app.client
	v.endVote()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		poll, err := client.PollVote(ctx, status.Poll.ID, choices...)
		v.app.Sync(func() {
			if err != nil {
				v.app.SetError("Vote failed: " + err.Error())
				return
			}
			updated := *status
			updated.Poll = poll
			v.replaceStatusEverywhere(&updated)
			v.app.Flash("Vote recorded")
		})
	}()
}
