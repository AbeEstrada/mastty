package utils

import (
	"reflect"
	"strings"
	"testing"

	"git.sr.ht/~rockorager/vaxis"
	"github.com/mattn/go-mastodon"
)

func segText(segs []vaxis.Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

func segLinks(segs []vaxis.Segment) []string {
	seen := map[string]bool{}
	var links []string
	for _, s := range segs {
		if s.Style.Hyperlink != "" && !seen[s.Style.Hyperlink] {
			seen[s.Style.Hyperlink] = true
			links = append(links, s.Style.Hyperlink)
		}
	}
	return links
}

func hasAttr(segs []vaxis.Segment, attr vaxis.AttributeMask, text string) bool {
	for _, s := range segs {
		if s.Style.Attribute&attr != 0 && strings.Contains(s.Text, text) {
			return true
		}
	}
	return false
}

func TestParseStatusText(t *testing.T) {
	cases := []struct {
		name     string
		html     string
		tags     []mastodon.Tag
		wantText string
	}{
		{"paragraph", "<p>Hello world</p>", nil, "Hello world\n\n"},
		{"two paragraphs", "<p>One</p><p>Two</p>", nil, "One\n\nTwo\n\n"},
		{"pretty printed", "<p>One</p>\n  <p>Two</p>\n", nil, "One\n\nTwo\n\n"},
		{"line break", "<p>a<br>b<br />c</p>", nil, "a\nb\nc\n\n"},
		{"entities", "<p>&lt;tag&gt; &amp; &quot;q&quot;</p>", nil, "<tag> & \"q\"\n\n"},
		{"mention", `<p><span class="h-card"><a href="https://example.social/@alice" class="u-url mention">@<span>alice</span></a></span> hi</p>`, nil, "@alice hi\n\n"},
		{"hashtag", `<p><a href="https://mastodon.social/tags/golang" class="mention hashtag" rel="tag">#<span>golang</span></a></p>`, []mastodon.Tag{{Name: "golang"}}, "#golang\n\n"},
		{"list", "<ul><li>one</li><li>two</li></ul>", nil, "• one\n• two\n\n"},
		{"ordered list", "<p>Steps</p><ol><li>first</li><li>second</li></ol><p>done</p>", nil, "Steps\n\n1. first\n2. second\n\ndone\n\n"},
		{"nested list", "<ul><li>a<ul><li>b</li></ul></li></ul>", nil, "• a\n  • b\n\n"},
		{"ellipsis", `<p><a href="https://example.com/a/very/long/path"><span class="invisible">https://</span><span class="ellipsis">example.com/a/very</span><span class="invisible">/long/path</span></a></p>`, nil, "example.com/a/very…\n\n"},
		{"blockquote", "<p>said:</p><blockquote><p>quoted</p></blockquote><p>after</p>", nil, "said:\n\n│ quoted\n\nafter\n\n"},
		{"pre keeps newlines", "<pre><code>a\n  b\n</code></pre>", nil, "a\n  b\n\n"},
		{"heading", "<h1>Title</h1><p>body</p>", nil, "Title\n\nbody\n\n"},
		{"image alt", `<p>look <img alt=":blob:" src="x"> ok</p>`, nil, "look :blob: ok\n\n"},
		{"whitespace only", "<p> </p>", nil, ""},
		{"empty", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := segText(ParseStatus(tc.html, tc.tags))
			if got != tc.wantText {
				t.Fatalf("ParseStatus(%q) text = %q, want %q", tc.html, got, tc.wantText)
			}
		})
	}
}

func TestParseStatusLinkUnescaped(t *testing.T) {
	html := `<p>See <a href="https://example.com/?a=1&amp;b=2" rel="nofollow noopener" target="_blank"><span class="invisible">https://</span><span class="">example.com/?a=1&amp;b=2</span><span class="invisible"></span></a></p>`
	segs := ParseStatus(html, nil)
	links := segLinks(segs)
	want := []string{"https://example.com/?a=1&b=2"}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("links = %v, want %v", links, want)
	}
	if !strings.Contains(segText(segs), "example.com/?a=1&b=2") {
		t.Fatalf("link text missing from %q", segText(segs))
	}
}

func TestParseStatusStyles(t *testing.T) {
	segs := ParseStatus("<p>Hello <strong>world</strong> and <b>again</b> <em>soft</em> <code>x := 1</code> <del>gone</del></p>", nil)
	if !hasAttr(segs, vaxis.AttrBold, "world") || !hasAttr(segs, vaxis.AttrBold, "again") {
		t.Fatalf("expected bold segments, got %#v", segs)
	}
	if !hasAttr(segs, vaxis.AttrItalic, "soft") {
		t.Fatalf("expected italic segment, got %#v", segs)
	}
	if !hasAttr(segs, vaxis.AttrStrikethrough, "gone") {
		t.Fatalf("expected strikethrough segment, got %#v", segs)
	}
	foundCode := false
	for _, s := range segs {
		if s.Text == "x := 1" && s.Style.Foreground == vaxis.IndexColor(6) {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatalf("expected code segment, got %#v", segs)
	}
	if got := segText(segs); got != "Hello world and again soft x := 1 gone\n\n" {
		t.Fatalf("text = %q", got)
	}
}

func TestParseStatusNestedLink(t *testing.T) {
	segs := ParseStatus(`<p><strong>see <a href="https://example.com">this <em>link</em></a></strong></p>`, nil)
	var linkSegs []vaxis.Segment
	for _, s := range segs {
		if s.Style.Hyperlink == "https://example.com" {
			linkSegs = append(linkSegs, s)
		}
	}
	if len(linkSegs) == 0 {
		t.Fatalf("link inside strong lost its hyperlink: %#v", segs)
	}
	if segText(linkSegs) != "this link" {
		t.Fatalf("link text = %q", segText(linkSegs))
	}
	for _, s := range linkSegs {
		if s.Style.Attribute&vaxis.AttrBold == 0 {
			t.Fatalf("link segment lost bold: %#v", s)
		}
	}
}

func TestParseStatusMalformed(t *testing.T) {
	for _, html := range []string{
		"<p>unclosed <b>bold",
		"<p>broken <a href=\"https://example.com\">link",
		"text <",
		"<p>tag <unknown attr=1>inside</unknown></p>",
		"</p></b>stray closers",
	} {
		segs := ParseStatus(html, nil)
		if segText(segs) == "" {
			t.Fatalf("ParseStatus(%q) produced no text", html)
		}
	}
}

func TestExtractLinks(t *testing.T) {
	html := `<p><span class="h-card"><a href="https://example.social/@alice" class="u-url mention">@<span>alice</span></a></span> ` +
		`<a href="https://a.example/?x=1&amp;y=2" rel="nofollow"><span class="invisible">https://</span><span class="ellipsis">a.example/?x=1&amp;y</span><span class="invisible">=2</span></a> ` +
		`<a href="/relative">r</a> <a href="https://a.example/?x=1&amp;y=2">dup</a> ` +
		`<a href="https://b.example/tags/go" class="mention hashtag" rel="tag">#<span>go</span></a></p>`
	got := ExtractLinks(html)
	want := []Link{
		{URL: "https://example.social/@alice", Text: "@alice", Kind: LinkKindMention},
		{URL: "https://a.example/?x=1&y=2", Text: "a.example/?x=1&y…", Kind: LinkKindLink},
		{URL: "https://b.example/tags/go", Text: "#go", Kind: LinkKindHashtag},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractLinks = %#v, want %#v", got, want)
	}
	urls := ExtractAllURLs(html)
	if !reflect.DeepEqual(urls, []string{want[0].URL, want[1].URL, want[2].URL}) {
		t.Fatalf("ExtractAllURLs = %v", urls)
	}
	if first := ExtractFirstExternalURL(`<a href="https://b.example/tags/go">t</a> <a href="https://c.example/">c</a>`); first != "https://c.example/" {
		t.Fatalf("ExtractFirstExternalURL = %q", first)
	}
}

func TestPlainText(t *testing.T) {
	got := PlainText("<p>Hello <b>world</b></p><p>second   line<br>third</p>")
	if got != "Hello world second line third" {
		t.Fatalf("PlainText = %q", got)
	}
}

func TestIsTagLinkAndValidURL(t *testing.T) {
	if !IsTagLink("https://mastodon.social/tags/golang") {
		t.Fatal("expected tag link")
	}
	if IsTagLink("https://mastodon.social/@user") {
		t.Fatal("did not expect tag link")
	}
	if !IsValidURL("https://example.com") || IsValidURL("/relative") || IsValidURL("") || IsValidURL("mailto:") {
		t.Fatal("IsValidURL mismatch")
	}
}

func TestFormatNumber(t *testing.T) {
	cases := map[int64]string{0: "0", 999: "999", 1500: "1.5k", 12000: "12.0k", 2000000: "2.0m"}
	for n, want := range cases {
		if got := FormatNumber(n); got != want {
			t.Errorf("FormatNumber(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestTitleCaseAndStripTags(t *testing.T) {
	if got := TitleCase("public"); got != "Public" {
		t.Errorf("TitleCase = %q", got)
	}
	if got := TitleCase("direct message"); got != "Direct Message" {
		t.Errorf("TitleCase = %q", got)
	}
	if got := StripTags("<span>a</span> <b>b</b> &amp; c"); got != "a b & c" {
		t.Errorf("StripTags = %q", got)
	}
}
