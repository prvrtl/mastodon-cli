package ui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapTextHardBreaksLongTokens(t *testing.T) {
	url := "https://techcrunch.com/2026/06/14/as-ai-companies-race-to-go-public-who-else"
	out := wrapText(url, 20)
	for _, line := range strings.Split(out, "\n") {
		if utf8.RuneCountInString(line) > 20 {
			t.Fatalf("line exceeds width: %q (%d)", line, utf8.RuneCountInString(line))
		}
	}
	if strings.ReplaceAll(out, "\n", "") != url {
		t.Fatalf("wrap lost characters: %q", out)
	}
}

func TestWrapTextWordWrap(t *testing.T) {
	out := wrapText("the quick brown fox jumps", 10)
	for _, line := range strings.Split(out, "\n") {
		if utf8.RuneCountInString(line) > 10 {
			t.Fatalf("line too long: %q", line)
		}
	}
}

func TestHTMLToText(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<p>Hello world</p>`, "Hello world"},
		{`<p>line one<br>line two</p>`, "line one\nline two"},
		{`<p>first</p><p>second</p>`, "first\n\nsecond"},
		{`<p>a &amp; b &lt;tag&gt;</p>`, "a & b <tag>"},
		{`<p>visit <a href="x">link</a> now</p>`, "visit link now"},
	}
	for _, c := range cases {
		if got := htmlToText(c.in); got != c.want {
			t.Errorf("htmlToText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
