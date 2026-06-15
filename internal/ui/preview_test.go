package ui

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderPreview(t *testing.T) {
	m := New(nil, testCfg("prvrtl", "mastodon.online"))
	res, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = res.(Model)
	m.viewLabel = "home"
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{
		{ID: "1", CreatedAt: time.Now().Add(-3 * time.Minute), Content: "<p>Just shipped the analytical engine v2 🎉</p>",
			Account: mastodon.Account{DisplayName: "Ada Lovelace", Acct: "ada@mastodon.online"}, RepliesCount: 4, ReblogsCount: 12, FavouritesCount: 30},
		{ID: "2", CreatedAt: time.Now().Add(-19 * time.Minute), Content: "<p>turing-complete is not the same as turing-tested.</p>",
			Account: mastodon.Account{DisplayName: "Alan", Acct: "alan"}, RepliesCount: 1, ReblogsCount: 8, FavouritesCount: 22},
	})
	m.refreshViewport()
	m.statusLine = "Loaded 2 posts from home"

	out := ansiRE.ReplaceAllString(m.View(), "")
	fmt.Println("\n" + out)
}

func TestRenderListPicker(t *testing.T) {
	m := New(nil, testCfg("prvrtl", "mastodon.online"))
	res, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 22})
	m = res.(Model)
	m.lists = []mastodon.List{{Title: "Apex"}, {Title: "1 TG"}, {Title: "Gaming"}, {Title: "News"}, {Title: "nsfw"}, {Title: "2 MASTOBOT"}}
	m.listPickerOpen = true
	m.listPickerIndex = 2
	m.statusLine = "Select a list to open"
	fmt.Println("\n" + ansiRE.ReplaceAllString(m.View(), ""))
}
