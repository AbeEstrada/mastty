package tui

import (
	"fmt"
	"slices"

	"git.sr.ht/~rockorager/vaxis"
	"git.sr.ht/~rockorager/vaxis/widgets/scrollbar"
	"github.com/AbeEstrada/tuit/utils"
	"github.com/mattn/go-mastodon"
)

const (
	avatarWidth  = 6
	avatarHeight = 3
	// farOffscreen positions a window far above the screen so Wrap measures
	// text without any of it being visible or clipped.
	farOffscreen = 1 << 20
)

var (
	dimStyle  = vaxis.Style{Attribute: vaxis.AttrDim}
	boldStyle = vaxis.Style{Attribute: vaxis.AttrBold}
	warnStyle = vaxis.Style{Foreground: vaxis.IndexColor(3), Attribute: vaxis.AttrBold}
)

type StatusView struct {
	app          *App
	scrollOffset int
	totalHeight  int
	viewHeight   int
	expanded     map[mastodon.ID]bool
	scrollbar    scrollbar.Model
	voteChoices  map[int]bool // options picked while voting, nil otherwise

	// Single-entry cache of the parsed body for the status being shown.
	cacheID      mastodon.ID
	cacheContent string
	cacheWidth   int
	cacheSegs    []vaxis.Segment
	cacheRows    int
}

func CreateStatusView() *StatusView {
	return &StatusView{
		expanded:  make(map[mastodon.ID]bool),
		scrollbar: scrollbar.Model{Style: vaxis.Style{Foreground: vaxis.IndexColor(8)}},
	}
}

func (v *StatusView) SetApp(app *App) {
	v.app = app
}

// SetVoteSelection shows which poll options are picked while voting; nil ends it.
func (v *StatusView) SetVoteSelection(choices map[int]bool) {
	v.voteChoices = choices
}

// ToggleExpanded shows or hides the body and media of a status that carries a
// content warning or sensitive media.
func (v *StatusView) ToggleExpanded(id mastodon.ID) {
	if id == "" {
		return
	}
	v.expanded[id] = !v.expanded[id]
}

func imageVisible(screenY, imgHeight, winHeight int) bool {
	return screenY >= 0 && screenY+imgHeight <= winHeight
}

// mediaRows returns how many rows an image of imgW x imgH pixels needs to span
// width columns, assuming cells are twice as tall as they are wide, capped so
// the image always fits in the pane.
func mediaRows(width int, imgW, imgH int64, maxRows int) int {
	rows := 10
	if imgW > 0 && imgH > 0 {
		rows = int(float64(width) * float64(imgH) / float64(imgW) * 0.5)
	}
	return max(1, min(rows, maxRows))
}

// measureWrap counts the rows segs occupy when wrapped to width, independent
// of where they will be drawn.
func measureWrap(win vaxis.Window, width int, segs ...vaxis.Segment) int {
	if width <= 0 || len(segs) == 0 {
		return 0
	}
	_, rows := win.New(0, -farOffscreen, width, 2*farOffscreen).Wrap(segs...)
	return rows
}

// drawWrapped draws segs wrapped to width starting at screenY, which may be
// off screen, and returns the rows they occupy.
func drawWrapped(win vaxis.Window, screenY, width int, segs ...vaxis.Segment) int {
	rows := measureWrap(win, width, segs...)
	_, height := win.Size()
	if rows > 0 && screenY < height && screenY+rows > 0 {
		win.New(0, screenY, width, farOffscreen).Wrap(segs...)
	}
	return rows
}

// drawLine draws one truncated line at screenY when it is visible.
func drawLine(win vaxis.Window, x, screenY, width int, segs ...vaxis.Segment) {
	_, height := win.Size()
	if screenY < 0 || screenY >= height || width <= 0 {
		return
	}
	win.New(x, screenY, width, 1).PrintTruncate(0, segs...)
}

// drawMedia draws the image at url at screen row screenY and returns the rows
// it occupies. While the image is still loading, the expected height is
// reserved so the layout does not jump when it arrives.
func drawMedia(win vaxis.Window, url string, width int, imgW, imgH int64, screenY, height int) int {
	if !utils.ImageCache.Enabled() || url == "" || width <= 0 {
		return 0
	}
	rows := mediaRows(width, imgW, imgH, max(1, height-1))
	vxImage, cached := utils.ImageCache.Get(url, width, rows)
	if !cached {
		return rows
	}
	_, mediaHeight := vxImage.CellSize()
	if mediaHeight <= 0 {
		// Sixel reports no size until encoding finishes; keep the space reserved.
		return rows
	}
	if imageVisible(screenY, mediaHeight, height) {
		vxImage.Draw(win.New(0, screenY, width, mediaHeight))
	}
	return mediaHeight
}

// contentWarning returns the text that should gate a status: its content
// warning, or the title of a server-side filter it matched.
func contentWarning(s *mastodon.Status) string {
	if s.SpoilerText != "" {
		return s.SpoilerText
	}
	for _, f := range s.Filtered {
		if f.Filter.Title != "" {
			return "Filtered: " + f.Filter.Title
		}
	}
	return ""
}

func mediaLabel(m mastodon.Attachment) string {
	switch m.Type {
	case "image":
		return "image"
	case "video":
		return "video"
	case "gifv":
		return "gif"
	case "audio":
		return "audio"
	}
	return "attachment"
}

// body returns the parsed content and its wrapped height, recomputing only
// when the status text or the pane width changes.
func (v *StatusView) body(win vaxis.Window, s *mastodon.Status, width int) ([]vaxis.Segment, int) {
	if v.cacheSegs != nil && v.cacheID == s.ID && v.cacheContent == s.Content && v.cacheWidth == width {
		return v.cacheSegs, v.cacheRows
	}
	segs := utils.ParseStatus(s.Content, s.Tags)
	if segs == nil {
		segs = []vaxis.Segment{}
	}
	v.cacheID, v.cacheContent, v.cacheWidth = s.ID, s.Content, width
	v.cacheSegs, v.cacheRows = segs, measureWrap(win, width, segs...)
	return v.cacheSegs, v.cacheRows
}

func (v *StatusView) Draw(win vaxis.Window, focused bool, status *mastodon.Status) {
	if status == nil {
		v.totalHeight = 0
		return
	}

	fullWidth, height := win.Size()
	v.viewHeight = height
	width := fullWidth - 1 // the last column is reserved for the scrollbar
	if width < 1 || height < 1 {
		return
	}

	display := originalStatus(status)
	so := v.scrollOffset
	y := 0
	line := func(x int, segs ...vaxis.Segment) {
		drawLine(win, x, y-so, width-x, segs...)
		y++
	}
	wrapped := func(segs ...vaxis.Segment) {
		y += drawWrapped(win, y-so, width, segs...)
	}

	if status.Reblog != nil {
		line(0, vaxis.Segment{Text: fmt.Sprintf("♺ Boosted by @%s", status.Account.Acct), Style: dimStyle})
		y++
	} else if status.InReplyToID != nil {
		line(0, vaxis.Segment{Text: "↩ Continued thread", Style: dimStyle})
		y++
	}

	// Header: avatar on the left, metadata on the right.
	headerY := y
	metaX := 0
	if utils.ImageCache.Enabled() && width > avatarWidth+8 {
		metaX = avatarWidth + 1
		if vxImage, cached := utils.ImageCache.Get(display.Account.AvatarStatic, avatarWidth, avatarHeight); cached {
			if imageVisible(headerY-so, avatarHeight, height) {
				vxImage.Draw(win.New(0, headerY-so, avatarWidth, avatarHeight))
			}
		}
	}

	name := display.Account.DisplayName
	if name == "" {
		name = display.Account.Username
	}
	author := []vaxis.Segment{
		{Text: name, Style: boldStyle},
		{Text: " (" + display.Account.Acct + ")", Style: boldStyle},
	}
	if display.Account.Bot {
		author = append(author, vaxis.Segment{Text: " · Automated"})
	}
	info := utils.FormatTimeSince(display.CreatedAt.Local()) + " · " + utils.TitleCase(display.Visibility)
	if !display.EditedAt.IsZero() {
		info += " · edited " + utils.FormatTimeSince(display.EditedAt.Local())
	}
	if display.Language != "" {
		info += " · " + display.Language
	}
	stats := fmt.Sprintf("%d replies · %d boosts · %d favorites", display.RepliesCount, display.ReblogsCount, display.FavouritesCount)
	if boolField(display.Favourited) {
		stats += " · ★ favourited"
	}
	if boolField(display.Reblogged) {
		stats += " · ♺ boosted"
	}
	if boolField(display.Bookmarked) {
		stats += " · ⚑ bookmarked"
	}
	line(metaX, author...)
	line(metaX, vaxis.Segment{Text: info})
	line(metaX, vaxis.Segment{Text: stats})
	y = max(y, headerY+avatarHeight) + 1

	// Content warning or filter: gate everything below it until expanded.
	warning := contentWarning(display)
	expanded := v.expanded[display.ID]
	if warning != "" {
		line(0, vaxis.Segment{Text: "⚠ CW: ", Style: warnStyle}, vaxis.Segment{Text: warning, Style: boldStyle})
		hint := "[x] show content"
		if expanded {
			hint = "[x] hide content"
		}
		line(0, vaxis.Segment{Text: hint, Style: dimStyle})
		y++
		if !expanded {
			v.finish(win, fullWidth, height, y)
			return
		}
	}

	segs, rows := v.body(win, display, width)
	if rows > 0 {
		if screenY := y - so; screenY < height && screenY+rows > 0 {
			win.New(0, screenY, width, farOffscreen).Wrap(segs...)
		}
		y += rows
	}

	if poll := display.Poll; poll != nil {
		line(0, vaxis.Segment{Text: "Poll", Style: boldStyle})
		if v.voteChoices != nil {
			line(0, vaxis.Segment{Text: "Press the option number to choose", Style: dimStyle})
		}
		indicator := "○"
		if poll.Multiple {
			indicator = "☐"
		}
		showResults := poll.Voted || poll.Expired
		for i, option := range poll.Options {
			prefix := indicator
			if slices.Contains(poll.OwnVotes, i) {
				prefix = "✓"
			}
			if v.voteChoices[i] {
				prefix = "◉"
			}
			votes := ""
			if showResults {
				percentage := 0.0
				if poll.VotersCount > 0 {
					percentage = float64(option.VotesCount) / float64(poll.VotersCount) * 100
				}
				votes = fmt.Sprintf("%3.0f%% ", percentage)
			}
			line(0, vaxis.Segment{Text: fmt.Sprintf("%s %s%s", prefix, votes, option.Title)})
		}
		if showResults {
			summary := fmt.Sprintf("%d voters", poll.VotersCount)
			if poll.Expired {
				summary += " · closed"
			}
			line(0, vaxis.Segment{Text: summary, Style: dimStyle})
		}
		y++
	}

	if card := display.Card; card != nil && (card.URL != "" || card.Image != "") {
		if card.URL != "" {
			title := card.Title
			if title == "" {
				title = card.URL
			}
			line(0,
				vaxis.Segment{Text: "↗ "},
				vaxis.Segment{Text: title, Style: vaxis.Style{Hyperlink: card.URL, UnderlineStyle: vaxis.UnderlineSingle}},
			)
			if card.ProviderName != "" {
				line(0, vaxis.Segment{Text: card.ProviderName, Style: dimStyle})
			}
			y++
		}
		if card.Image != "" {
			y += drawMedia(win, card.Image, width, card.Width, card.Height, y-so, height)
		}
		y++
	}

	if len(display.MediaAttachments) > 0 {
		hiddenMedia := display.Sensitive && !expanded
		for i, media := range display.MediaAttachments {
			if i > 0 {
				y++
			}
			label := mediaLabel(media)
			if hiddenMedia {
				line(0, vaxis.Segment{Text: fmt.Sprintf("▒ sensitive %s hidden · [x] show", label), Style: dimStyle})
				if media.Description != "" {
					wrapped(vaxis.Segment{Text: media.Description, Style: dimStyle})
				}
				continue
			}

			switch media.Type {
			case "video", "gifv":
				line(0, vaxis.Segment{Text: "▶ " + label})
			case "audio":
				line(0, vaxis.Segment{Text: "♪ " + label})
			}

			imageURL := media.PreviewURL
			if imageURL == "" {
				imageURL = media.URL
			}
			drawn := 0
			if imageURL != "" && media.Type != "audio" {
				meta := media.Meta.Original
				drawn = drawMedia(win, imageURL, width, int64(meta.Width), int64(meta.Height), y-so, height)
				y += drawn
			}
			if drawn == 0 && media.Type == "image" {
				line(0, vaxis.Segment{Text: "[image]", Style: dimStyle})
			}
			if media.Description != "" {
				wrapped(vaxis.Segment{Text: media.Description, Style: dimStyle})
			}
		}
		y++
	}

	v.finish(win, fullWidth, height, y)
}

// finish records the total height, keeps the scroll offset in range, and draws
// the scrollbar in the reserved right column.
func (v *StatusView) finish(win vaxis.Window, fullWidth, height, total int) {
	v.totalHeight = total
	if maxOffset := max(0, total-height); v.scrollOffset > maxOffset {
		v.scrollOffset = maxOffset
	}
	v.scrollbar.TotalHeight = total
	v.scrollbar.ViewHeight = height
	v.scrollbar.Top = v.scrollOffset
	v.scrollbar.Draw(win.New(fullWidth-1, 0, 1, height))
}

// Scroll moves the view by delta rows, clamped to the content.
func (v *StatusView) Scroll(delta int) {
	maxOffset := max(0, v.totalHeight-v.viewHeight)
	v.scrollOffset = max(0, min(maxOffset, v.scrollOffset+delta))
}

func (v *StatusView) ResetScroll() {
	v.scrollOffset = 0
}

func (v *StatusView) HandleKey(key vaxis.Key) {
	if v.totalHeight <= v.viewHeight {
		v.scrollOffset = 0
		return
	}
	maxOffset := v.totalHeight - v.viewHeight
	switch {
	case key.Matches('j'):
		if v.scrollOffset < maxOffset {
			v.scrollOffset++
		}
	case key.Matches('k'):
		if v.scrollOffset > 0 {
			v.scrollOffset--
		}
	case key.MatchString("Ctrl+d"):
		v.scrollOffset = min(maxOffset, v.scrollOffset+max(1, v.viewHeight/2))
	case key.MatchString("Ctrl+u"):
		v.scrollOffset = max(0, v.scrollOffset-max(1, v.viewHeight/2))
	case key.Matches('g'):
		v.scrollOffset = 0
	case key.Matches('G'):
		v.scrollOffset = maxOffset
	}
}
