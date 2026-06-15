package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type slashCmd struct {
	name     string
	desc     string
	takesArg bool
}

var slashCommands = []slashCmd{
	{"/home", "Your home timeline", false},
	{"/public", "Federated public timeline", false},
	{"/local", "Local instance timeline", false},
	{"/lists", "Show your lists", false},
	{"/list", "Open a list: /list <n>", true},
	{"/tag", "Open a hashtag feed (live): /tag <name>", true},
	{"/account", "View an account's posts: /account <@user>", true},
	{"/search", "Search accounts, tags, posts: /search <query>", true},
	{"/poll", "Post a poll: /poll <q> | opt1 | opt2", true},
	{"/bookmarks", "Show your bookmarks", false},
	{"/notifications", "Show notifications", false},
	{"/refresh", "Reload the current view", false},
	{"/reply", "Reply to a post: /reply <n> <text>", true},
	{"/boost", "Boost / unboost: /boost <n>", true},
	{"/fav", "Favourite / unfavourite: /fav <n>", true},
	{"/bookmark", "Bookmark / unbookmark: /bookmark <n>", true},
	{"/thread", "Show a post's thread: /thread <n>", true},
	{"/follow", "Follow a post's author: /follow <n>", true},
	{"/open", "Open a post in the browser: /open <n>", true},
	{"/stream", "Toggle live updates: /stream [on|off]", true},
	{"/accounts", "List logged-in accounts", false},
	{"/switch", "Switch account: /switch <n>", true},
	{"/whoami", "Show the logged-in account", false},
	{"/help", "Show all commands", false},
	{"/quit", "Exit md", false},
}

func (m Model) menuMatches() []slashCmd {
	v := strings.ToLower(m.input.Value())
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, " \n") {
		return nil
	}
	var out []slashCmd
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, v) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) menuVisible() bool {
	return !m.menuDismissed && len(m.menuMatches()) > 0
}

const menuMaxRows = 8

func (m Model) renderMenu() string {
	matches := m.menuMatches()
	if m.menuDismissed || len(matches) == 0 {
		return ""
	}

	start := 0
	if m.menuIndex >= menuMaxRows {
		start = m.menuIndex - menuMaxRows + 1
	}
	end := start + menuMaxRows
	if end > len(matches) {
		end = len(matches)
	}

	rowW := m.width - 2
	header := styleMenuHint.Render(padTo(" Commands   ↑↓ select · Tab complete · Esc close", rowW))
	lines := []string{header}
	for i := start; i < end; i++ {
		c := matches[i]
		text := fmt.Sprintf("  %-16s %s", c.name, c.desc)
		text = padTo(truncate(text, rowW), rowW)
		if i == m.menuIndex && m.menuNavigated {
			lines = append(lines, styleMenuSel.Render(text))
		} else {
			lines = append(lines, styleMenuItem.Render(text))
		}
	}
	return strings.Join(lines, "\n")
}

func padTo(s string, w int) string {
	n := w - lipgloss.Width(s)
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(" ", n)
}
