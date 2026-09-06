package tui

import (
	"fmt"
	"strings"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

const (
	profileAvatarWidth  = 24
	profileAvatarHeight = 12
)

type AccountView struct {
	app           *App
	Relationships map[mastodon.ID]*mastodon.Relationship
}

func CreateAccountView() *AccountView {
	return &AccountView{}
}

func (v *AccountView) SetApp(app *App) {
	v.app = app
}

func (v *AccountView) Draw(win vaxis.Window, focused bool, account *mastodon.Account) {
	if account == nil {
		return
	}

	width, height := win.Size()
	y := 0

	if utils.ImageCache.Enabled() && width > profileAvatarWidth {
		if vxImage, cached := utils.ImageCache.Get(account.AvatarStatic, profileAvatarWidth, profileAvatarHeight); cached {
			vxImage.Draw(win.New(0, y, profileAvatarWidth, profileAvatarHeight))
		}
	}

	metaX := profileAvatarWidth + 1
	metaY := 0
	metaWin := win.New(metaX, 0, width-metaX, profileAvatarHeight)
	metaWin.PrintTruncate(metaY, vaxis.Segment{
		Text:  account.DisplayName,
		Style: vaxis.Style{Attribute: vaxis.AttrBold},
	})
	metaY++
	metaWin.PrintTruncate(metaY, vaxis.Segment{
		Text:  fmt.Sprintf("@%s", account.Acct),
		Style: vaxis.Style{Attribute: vaxis.AttrBold},
	})
	metaY++
	if account.Bot {
		metaWin.PrintTruncate(metaY, vaxis.Segment{Text: "Automated"})
		metaY++
	}
	metaWin.PrintTruncate(metaY, vaxis.Segment{
		Text: fmt.Sprintf("Joined %s", account.CreatedAt.Local().Format("Jan 2, 2006")),
	})
	metaY += 2
	metaWin.PrintTruncate(metaY, vaxis.Segment{Text: fmt.Sprintf("%s posts", utils.FormatNumber(account.StatusesCount))})
	metaY++
	metaWin.PrintTruncate(metaY, vaxis.Segment{Text: fmt.Sprintf("%s following", utils.FormatNumber(account.FollowingCount))})
	metaY++
	metaWin.PrintTruncate(metaY, vaxis.Segment{Text: fmt.Sprintf("%s followers", utils.FormatNumber(account.FollowersCount))})
	if rel := v.Relationships[account.ID]; rel != nil {
		var parts []string
		switch {
		case rel.Following:
			parts = append(parts, "✓ Following")
		case rel.Requested:
			parts = append(parts, "Follow requested")
		}
		if rel.FollowedBy {
			parts = append(parts, "Follows you")
		}
		if rel.Blocking {
			parts = append(parts, "Blocked")
		}
		if rel.Muting {
			parts = append(parts, "Muted")
		}
		if len(parts) > 0 {
			metaY++
			metaWin.PrintTruncate(metaY, vaxis.Segment{Text: strings.Join(parts, " · "), Style: vaxis.Style{Foreground: vaxis.IndexColor(2)}})
		}
	}

	y += profileAvatarHeight

	fieldsWin := win.New(0, y, width, len(account.Fields))
	for i, field := range account.Fields {
		valueSegments := utils.ParseStatus(field.Value, nil)

		var flatText strings.Builder
		for _, seg := range valueSegments {
			clean := strings.ReplaceAll(seg.Text, "\n", " ")
			clean = strings.TrimSpace(clean)
			if clean != "" {
				flatText.WriteString(clean + " ")
			}
		}
		flatValue := strings.TrimSpace(flatText.String())

		verified := ""
		verifiedStyle := vaxis.Style{}
		if !field.VerifiedAt.IsZero() {
			verified = "✓ "
			verifiedStyle = vaxis.Style{Foreground: vaxis.IndexColor(2)}
		}

		valueStyle := vaxis.Style{}
		if utils.IsValidURL(flatValue) {
			valueStyle = vaxis.Style{
				Hyperlink:      flatValue,
				UnderlineStyle: vaxis.UnderlineSingle,
			}
		}

		fieldsWin.PrintTruncate(
			i,
			vaxis.Segment{Text: verified, Style: verifiedStyle},
			vaxis.Segment{
				Text:  fmt.Sprintf("%s: ", field.Name),
				Style: vaxis.Style{Attribute: vaxis.AttrBold},
			},
			vaxis.Segment{Text: flatValue, Style: valueStyle},
		)
		y++
	}
	y++

	if y < height {
		contentWin := win.New(0, y, width, height-y)
		contentWin.Wrap(utils.ParseStatus(account.Note, nil)...)
	}
}

func (v *AccountView) HandleKey(key vaxis.Key) {}
