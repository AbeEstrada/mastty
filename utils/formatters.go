package utils

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/mattn/go-mastodon"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// LinkKind classifies an anchor found in status HTML.
type LinkKind int

const (
	LinkKindLink LinkKind = iota
	LinkKindMention
	LinkKindHashtag
)

// Link is an anchor extracted from status HTML. URL is already HTML-unescaped.
type Link struct {
	URL  string
	Text string
	Kind LinkKind
}

var (
	styleBold      = vaxis.Style{Attribute: vaxis.AttrBold}
	styleItalic    = vaxis.Style{Attribute: vaxis.AttrItalic}
	styleUnderline = vaxis.Style{UnderlineStyle: vaxis.UnderlineSingle}
	styleStrike    = vaxis.Style{Attribute: vaxis.AttrStrikethrough}
	styleCode      = vaxis.Style{Foreground: vaxis.IndexColor(6)}
	styleQuote     = vaxis.Style{Attribute: vaxis.AttrDim}
)

// TitleCase upper-cases the first letter of every word ("public" -> "Public").
func TitleCase(text string) string {
	runes := []rune(text)
	upperNext := true
	for i, r := range runes {
		if unicode.IsSpace(r) {
			upperNext = true
			continue
		}
		if upperNext {
			runes[i] = unicode.ToUpper(r)
			upperNext = false
		}
	}
	return string(runes)
}

// StripTags returns the text content of an HTML fragment with entities decoded.
func StripTags(s string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return b.String()
		case html.TextToken:
			b.Write(z.Text())
		}
	}
}

// PlainText renders status HTML to a single line of text for previews.
func PlainText(content string) string {
	var b strings.Builder
	for _, seg := range ParseStatus(content, nil) {
		b.WriteString(seg.Text)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func FormatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(n)/1000000)
}

// ExtractLinks returns the anchors in content, in order of appearance and
// without duplicates, with their visible text and kind.
func ExtractLinks(content string) []Link {
	z := html.NewTokenizer(strings.NewReader(content))
	var (
		links   []Link
		seen    = map[string]bool{}
		current *Link
		text    strings.Builder
		spans   []spanFlags // spans open inside the current anchor
		skip    int
	)
	finish := func() {
		if current == nil {
			return
		}
		current.Text = strings.TrimSpace(collapseSpace(text.String()))
		if IsValidURL(current.URL) && !seen[current.URL] {
			seen[current.URL] = true
			links = append(links, *current)
		}
		current = nil
		spans = spans[:0]
		skip = 0
	}
	for {
		switch z.Next() {
		case html.ErrorToken:
			finish()
			return links
		case html.TextToken:
			if current != nil && skip == 0 {
				text.Write(z.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			switch tok.DataAtom {
			case atom.A:
				finish()
				href := attrValue(tok, "href")
				current = &Link{URL: href, Kind: linkKind(tok, href)}
				text.Reset()
			case atom.Span:
				if current != nil {
					flags := spanFlagsOf(tok)
					if flags.invisible {
						skip++
					}
					spans = append(spans, flags)
				}
			}
		case html.EndTagToken:
			tok := z.Token()
			switch tok.DataAtom {
			case atom.A:
				finish()
			case atom.Span:
				if current != nil && len(spans) > 0 {
					flags := spans[len(spans)-1]
					spans = spans[:len(spans)-1]
					if flags.invisible && skip > 0 {
						skip--
					}
					if flags.ellipsis && skip == 0 {
						text.WriteString("…")
					}
				}
			}
		}
	}
}

// ExtractAllURLs returns the unique, absolute link targets found in content.
func ExtractAllURLs(content string) []string {
	links := ExtractLinks(content)
	urls := make([]string, 0, len(links))
	for _, l := range links {
		urls = append(urls, l.URL)
	}
	return urls
}

func IsTagLink(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasPrefix(u.Path, "/tags/")
}

func ExtractFirstExternalURL(content string) string {
	urls := ExtractAllURLs(content)
	for _, u := range urls {
		if !IsTagLink(u) {
			return u
		}
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}

func IsValidURL(link string) bool {
	if strings.TrimSpace(link) == "" {
		return false
	}

	u, err := url.Parse(link)
	if err != nil {
		return false
	}

	return u.Scheme != "" && u.Host != ""
}

type spanFlags struct {
	invisible bool
	ellipsis  bool
}

func spanFlagsOf(tok html.Token) spanFlags {
	class := attrValue(tok, "class")
	return spanFlags{
		invisible: hasClass(class, "invisible"),
		ellipsis:  hasClass(class, "ellipsis"),
	}
}

func attrValue(tok html.Token, name string) string {
	for _, a := range tok.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasClass(class, name string) bool {
	return slices.Contains(strings.Fields(class), name)
}

func linkKind(tok html.Token, href string) LinkKind {
	class := attrValue(tok, "class")
	switch {
	case hasClass(class, "hashtag"), hasClass(attrValue(tok, "rel"), "tag"), IsTagLink(href):
		return LinkKindHashtag
	case hasClass(class, "mention"):
		return LinkKindMention
	}
	return LinkKindLink
}

// collapseSpace replaces every run of whitespace with a single space.
func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

func mergeStyle(base, over vaxis.Style) vaxis.Style {
	var zeroColor vaxis.Color
	merged := base
	merged.Attribute |= over.Attribute
	if over.Foreground != zeroColor {
		merged.Foreground = over.Foreground
	}
	if over.Background != zeroColor {
		merged.Background = over.Background
	}
	if over.UnderlineStyle != vaxis.UnderlineOff {
		merged.UnderlineStyle = over.UnderlineStyle
	}
	if over.Hyperlink != "" {
		merged.Hyperlink = over.Hyperlink
	}
	return merged
}

type openElement struct {
	tag       atom.Atom
	styled    bool
	invisible bool
	ellipsis  bool
	list      bool
	quote     bool
	pre       bool
}

type listState struct {
	ordered bool
	index   int
}

// htmlRenderer converts tokenized HTML into styled segments. Block elements
// become line breaks, inline elements push styles onto a stack, and Mastodon's
// link-shortening spans are honored.
type htmlRenderer struct {
	segments         []vaxis.Segment
	styles           []vaxis.Style
	elements         []openElement
	lists            []*listState
	quoteDepth       int
	preDepth         int
	skipDepth        int
	trailingNewlines int
	emitted          bool
	atLineStart      bool
}

// ParseStatus converts status HTML into styled segments ready for wrapping.
// The tags parameter is accepted for compatibility; hashtags are recognized
// from the markup itself.
func ParseStatus(content string, tags []mastodon.Tag) []vaxis.Segment {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	r := &htmlRenderer{
		styles:      []vaxis.Style{{}},
		atLineStart: true,
	}
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		switch z.Next() {
		case html.ErrorToken:
			for i := len(r.elements) - 1; i >= 0; i-- {
				r.closeElement(r.elements[i])
			}
			r.elements = nil
			r.ensureNewlines(2)
			return r.segments
		case html.TextToken:
			r.text(string(z.Text()))
		case html.StartTagToken, html.SelfClosingTagToken:
			r.startTag(z.Token())
		case html.EndTagToken:
			r.endTag(z.Token().DataAtom)
		}
	}
}

func (r *htmlRenderer) top() vaxis.Style {
	return r.styles[len(r.styles)-1]
}

func (r *htmlRenderer) push(style vaxis.Style) {
	r.styles = append(r.styles, mergeStyle(r.top(), style))
}

func (r *htmlRenderer) pop() {
	if len(r.styles) > 1 {
		r.styles = r.styles[:len(r.styles)-1]
	}
}

func (r *htmlRenderer) appendSegment(text string, style vaxis.Style) {
	if n := len(r.segments); n > 0 && r.segments[n-1].Style == style {
		r.segments[n-1].Text += text
		return
	}
	r.segments = append(r.segments, vaxis.Segment{Text: text, Style: style})
}

func (r *htmlRenderer) newline() {
	r.appendSegment("\n", vaxis.Style{})
	r.trailingNewlines++
	r.atLineStart = true
}

// ensureNewlines makes sure the output ends with at least n line breaks, but
// never adds blank lines before the first visible text.
func (r *htmlRenderer) ensureNewlines(n int) {
	if !r.emitted {
		return
	}
	for r.trailingNewlines < n {
		r.newline()
	}
}

// emitText writes text with the given style, handling embedded newlines and
// blockquote prefixes.
func (r *htmlRenderer) emitText(text string, style vaxis.Style) {
	for text != "" {
		line := text
		newline := false
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			line, text, newline = text[:i], text[i+1:], true
		} else {
			text = ""
		}
		if line != "" {
			if r.atLineStart && r.quoteDepth > 0 {
				r.appendSegment(strings.Repeat("│ ", r.quoteDepth), styleQuote)
			}
			r.appendSegment(line, style)
			r.atLineStart = false
			r.trailingNewlines = 0
			r.emitted = true
		}
		if newline {
			if r.emitted {
				r.newline()
			}
		}
	}
}

func (r *htmlRenderer) text(raw string) {
	if r.skipDepth > 0 || raw == "" {
		return
	}
	if r.preDepth > 0 {
		r.emitText(raw, r.top())
		return
	}
	collapsed := collapseSpace(raw)
	if r.atLineStart || !r.emitted {
		collapsed = strings.TrimLeft(collapsed, " ")
	}
	if collapsed == "" {
		return
	}
	r.emitText(collapsed, r.top())
}

func (r *htmlRenderer) startTag(tok html.Token) {
	el := openElement{tag: tok.DataAtom}
	styled := func(s vaxis.Style) {
		r.push(s)
		el.styled = true
	}

	switch tok.DataAtom {
	case atom.Br:
		if r.emitted {
			r.newline()
		}
		return
	case atom.Hr:
		r.ensureNewlines(1)
		r.emitText("────", styleQuote)
		r.newline()
		return
	case atom.Img:
		if alt := attrValue(tok, "alt"); alt != "" {
			r.text(alt)
		}
		return
	case atom.P, atom.Div, atom.Section, atom.Article, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		r.ensureNewlines(2)
		if isHeading(tok.DataAtom) {
			styled(styleBold)
		}
	case atom.B, atom.Strong:
		styled(styleBold)
	case atom.I, atom.Em:
		styled(styleItalic)
	case atom.U:
		styled(styleUnderline)
	case atom.S, atom.Del, atom.Strike:
		styled(styleStrike)
	case atom.Code:
		if r.preDepth == 0 {
			styled(styleCode)
		}
	case atom.Pre:
		r.ensureNewlines(2)
		styled(styleCode)
		r.preDepth++
		el.pre = true
	case atom.Blockquote:
		r.ensureNewlines(2)
		r.quoteDepth++
		el.quote = true
	case atom.Ul, atom.Ol:
		if len(r.lists) == 0 {
			r.ensureNewlines(2)
		} else {
			r.ensureNewlines(1)
		}
		r.lists = append(r.lists, &listState{ordered: tok.DataAtom == atom.Ol})
		el.list = true
	case atom.Li:
		r.ensureNewlines(1)
		depth := len(r.lists)
		marker := "• "
		if depth > 0 && r.lists[depth-1].ordered {
			r.lists[depth-1].index++
			marker = fmt.Sprintf("%d. ", r.lists[depth-1].index)
		}
		r.emitText(strings.Repeat("  ", max(0, depth-1))+marker, r.top())
	case atom.A:
		style := vaxis.Style{UnderlineStyle: vaxis.UnderlineSingle}
		if href := attrValue(tok, "href"); href != "" {
			style.Hyperlink = href
		}
		styled(style)
	case atom.Span:
		flags := spanFlagsOf(tok)
		if flags.invisible {
			r.skipDepth++
			el.invisible = true
		}
		el.ellipsis = flags.ellipsis
	}

	if tok.Type == html.SelfClosingTagToken {
		r.closeElement(el)
		return
	}
	r.elements = append(r.elements, el)
}

func isHeading(a atom.Atom) bool {
	switch a {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

func (r *htmlRenderer) endTag(a atom.Atom) {
	idx := -1
	for i := len(r.elements) - 1; i >= 0; i-- {
		if r.elements[i].tag == a {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	// Close anything left open above the matching element (malformed nesting).
	for i := len(r.elements) - 1; i >= idx; i-- {
		r.closeElement(r.elements[i])
	}
	r.elements = r.elements[:idx]
}

func (r *htmlRenderer) closeElement(el openElement) {
	if el.styled {
		r.pop()
	}
	if el.invisible && r.skipDepth > 0 {
		r.skipDepth--
	}
	if el.ellipsis && r.skipDepth == 0 {
		r.emitText("…", r.top())
	}
	if el.list && len(r.lists) > 0 {
		r.lists = r.lists[:len(r.lists)-1]
	}
	if el.quote && r.quoteDepth > 0 {
		r.quoteDepth--
	}
	if el.pre && r.preDepth > 0 {
		r.preDepth--
	}

	switch el.tag {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Pre, atom.Blockquote,
		atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		r.ensureNewlines(2)
	case atom.Li:
		r.ensureNewlines(1)
	case atom.Ul, atom.Ol:
		if len(r.lists) == 0 {
			r.ensureNewlines(2)
		} else {
			r.ensureNewlines(1)
		}
	}
}
