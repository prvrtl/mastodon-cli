package ui

import (
	"html"
	"regexp"
	"strings"
)

var (
	reBR    = regexp.MustCompile(`(?i)<br\s*/?>`)
	rePEnd  = regexp.MustCompile(`(?i)</p>`)
	rePStrt = regexp.MustCompile(`(?i)<p[^>]*>`)
	reTag   = regexp.MustCompile(`<[^>]+>`)
	reWS    = regexp.MustCompile(`[ \t]+`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

func PlainText(s string) string { return htmlToText(s) }

func htmlToText(s string) string {
	s = reBR.ReplaceAllString(s, "\n")
	s = rePEnd.ReplaceAllString(s, "\n\n")
	s = rePStrt.ReplaceAllString(s, "")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)

	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(reWS.ReplaceAllString(ln, " "), " ")
	}
	s = strings.Join(lines, "\n")
	s = reBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
