package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/prvrtl/mastocli/internal/mastodon"
)

func wrapText(s string, width int) string {
	if width <= 1 {
		return s
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		lines = append(lines, wrapLine(para, width))
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, width int) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	cur := 0
	for _, w := range words {

		for utf8.RuneCountInString(w) > width {
			if cur > 0 {
				b.WriteByte('\n')
				cur = 0
			}
			r := []rune(w)
			b.WriteString(string(r[:width]))
			b.WriteByte('\n')
			w = string(r[width:])
		}
		wl := utf8.RuneCountInString(w)
		switch {
		case cur == 0:
			b.WriteString(w)
			cur = wl
		case cur+1+wl <= width:
			b.WriteByte(' ')
			b.WriteString(w)
			cur += 1 + wl
		default:
			b.WriteByte('\n')
			b.WriteString(w)
			cur = wl
		}
	}
	return b.String()
}

type itemKind int

const (
	kindStatus itemKind = iota
	kindNotif
	kindSystem
	kindError
)

type feedItem struct {
	kind   itemKind
	status *mastodon.Status
	notif  *mastodon.Notification
	text   string
	index  int
	isNew  bool
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func (it feedItem) render(width int) string {
	switch it.kind {
	case kindSystem:
		return styleSystem.Width(width).Render(it.text)
	case kindError:
		return styleErr.Width(width).Render("✗ " + it.text)
	case kindNotif:
		return renderNotif(it, width)
	default:
		return renderStatus(it, width)
	}
}

func renderStatus(it feedItem, width int) string {
	s := it.status
	var b strings.Builder

	booster := ""
	if s.Reblog != nil {
		booster = s.Account.Name()
		s = s.Reblog
	}

	var head strings.Builder
	if it.index > 0 {
		head.WriteString(styleIndex.Render(fmt.Sprintf("[%d]", it.index)) + " ")
	}
	head.WriteString(styleAuthor.Render(s.Account.Name()))
	head.WriteString(" ")
	head.WriteString(styleHandle.Render("@" + s.Account.Acct))
	head.WriteString(styleTime.Render("  ·  " + relTime(s.CreatedAt)))
	if it.isNew {
		head.WriteString(styleOK.Render("  ● new"))
	}
	b.WriteString(head.String())
	b.WriteString("\n")

	if booster != "" {
		b.WriteString(styleBoost.Render("↻ boosted by " + booster))
		b.WriteString("\n")
	}

	body := htmlToText(s.Content)
	if s.SpoilerText != "" {
		b.WriteString(styleSpoil.Render("⚠ " + wrapText(s.SpoilerText, width)))
		b.WriteString("\n")
	}
	if body != "" {
		b.WriteString(styleBody.Render(wrapText(body, width)))
		b.WriteString("\n")
	}

	for _, m := range s.MediaAttachments {
		label := m.Type
		if m.Description != "" {
			label += ": " + m.Description
		}
		b.WriteString(styleMeta.Render("🖼  " + label))
		b.WriteString("\n")
	}

	if s.Poll != nil {
		b.WriteString(renderPoll(s.Poll, width))
		b.WriteString("\n")
	}

	footer := fmt.Sprintf("%s  %s  %s",
		styleReplies.Render(fmt.Sprintf("💬 %d", s.RepliesCount)),
		styleReblog.Render(boostMark(s)+fmt.Sprintf(" %d", s.ReblogsCount)),
		styleFav.Render(favMark(s)+fmt.Sprintf(" %d", s.FavouritesCount)),
	)
	if s.Bookmarked {
		footer += styleFav.Render("  🔖")
	}
	if s.Pinned {
		footer += styleMeta.Render("  📌")
	}
	b.WriteString(styleMeta.Render(footer))

	return b.String()
}

func renderPoll(p *mastodon.Poll, width int) string {
	var b strings.Builder
	for _, opt := range p.Options {
		pct := 0
		if p.VotesCount > 0 {
			pct = opt.VotesCount * 100 / p.VotesCount
		}
		barLen := pct * 20 / 100
		bar := strings.Repeat("█", barLen) + strings.Repeat("░", 20-barLen)
		b.WriteString(styleMeta.Render(fmt.Sprintf("  %s  %3d%%  %s", bar, pct, opt.Title)))
		b.WriteString("\n")
	}
	state := fmt.Sprintf("  %d votes", p.VotesCount)
	if p.Expired {
		state += " · closed"
	}
	if p.Voted {
		state += " · voted ✓"
	}
	b.WriteString(styleMeta.Render(state))
	return b.String()
}

func boostMark(s *mastodon.Status) string {
	if s.Reblogged {
		return "↻✓"
	}
	return "↻"
}

func favMark(s *mastodon.Status) string {
	if s.Favourited {
		return "★"
	}
	return "☆"
}

func renderNotif(it feedItem, width int) string {
	n := it.notif
	var verb string
	switch n.Type {
	case "favourite":
		verb = "★ favourited your post"
	case "reblog":
		verb = "↻ boosted your post"
	case "follow":
		verb = "＋ followed you"
	case "mention":
		verb = "@ mentioned you"
	case "poll":
		verb = "📊 a poll ended"
	case "update":
		verb = "✎ edited a post"
	default:
		verb = n.Type
	}

	var b strings.Builder
	if it.index > 0 {
		b.WriteString(styleIndex.Render(fmt.Sprintf("[%d]", it.index)) + " ")
	}
	b.WriteString(styleNotif.Render(n.Account.Name()))
	b.WriteString(" ")
	b.WriteString(styleHandle.Render("@" + n.Account.Acct))
	b.WriteString(styleNotif.Render("  " + verb))
	b.WriteString(styleTime.Render("  ·  " + relTime(n.CreatedAt)))
	if n.Status != nil {
		body := htmlToText(n.Status.Content)
		if body != "" {
			b.WriteString("\n")
			b.WriteString(styleMeta.Render(quote(wrapText(body, width-2))))
		}
	}
	return b.String()
}

func quote(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = "│ " + ln
	}
	return strings.Join(lines, "\n")
}
