package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

func TestPlainTextExported(t *testing.T) {
	if PlainText("<p>hello <b>world</b></p>") != "hello world" {
		t.Errorf("PlainText = %q", PlainText("<p>hello <b>world</b></p>"))
	}
}

func TestTabCompletesMenu(t *testing.T) {
	m := readyModel(t)
	m.input.SetValue("/li")
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if got := res.(Model).input.Value(); got != "/lists " {
		t.Errorf("Tab completion = %q, want %q", got, "/lists ")
	}
}

func TestActionAndPollMenusRenderInView(t *testing.T) {
	m := readyModel(t)
	s := status("100", "alice")
	s.Poll = &mastodon.Poll{ID: "p", Options: []mastodon.PollOption{{Title: "Apples"}, {Title: "Pears"}}}
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{s})

	res, _ := m.submit("1")
	am := res.(Model)
	out := plain(am.View())
	for _, want := range []string{"Vote in poll", "Boost", "Reply", "Mute @alice", "Block @alice"} {
		if !strings.Contains(out, want) {
			t.Errorf("action menu view missing %q", want)
		}
	}

	res, _ = am.runAction(0)
	pv := res.(Model)
	pout := plain(pv.View())
	if !strings.Contains(pout, "Apples") || !strings.Contains(pout, "Pears") {
		t.Errorf("poll vote view missing options:\n%s", pout)
	}
}

func TestOpenListPickerBehaviour(t *testing.T) {
	m := readyModel(t)
	if _, cmd := m.openListPicker(); cmd == nil {
		t.Error("no cached lists should trigger a load")
	}
	m.lists = []mastodon.List{{ID: "1", Title: "Friends"}}
	res, cmd := m.openListPicker()
	if !res.(Model).listPickerOpen || cmd != nil {
		t.Error("cached lists should open the picker without loading")
	}
}

func TestApplyPollUpdatesStatus(t *testing.T) {
	m := readyModel(t)
	s := status("5", "a")
	s.Poll = &mastodon.Poll{ID: "p", Voted: false}
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{s})
	m.applyPoll("5", &mastodon.Poll{ID: "p", Voted: true})
	if !m.statusByIndex[1].Poll.Voted {
		t.Error("applyPoll should mark the poll voted")
	}
}

func TestPollVotedAndModerationMessages(t *testing.T) {
	m := readyModel(t)
	s := status("5", "a")
	s.Poll = &mastodon.Poll{ID: "p"}
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{s})

	res, _ := m.Update(pollVotedMsg{statusID: "5", poll: &mastodon.Poll{ID: "p", Voted: true}})
	mm := res.(Model)
	if !strings.Contains(mm.statusLine, "Vote") || !mm.statusByIndex[1].Poll.Voted {
		t.Error("pollVotedMsg should record the vote")
	}

	res, _ = m.Update(pollVotedMsg{err: errString("nope")})
	if !hasErrorItem(res.(Model)) {
		t.Error("failed vote should surface an error")
	}
}

func TestLoadCommandsReturnNonNil(t *testing.T) {
	c := (*mastodon.Client)(nil)
	cmds := []tea.Cmd{
		loadBookmarksCmd(c),
		loadNotificationsCmd(c),
		loadListsCmd(c),
		searchCmd(c, "x"),
		animCmd(1),
		tickCmd(),
	}
	for i, cmd := range cmds {
		if cmd == nil {
			t.Errorf("command %d should not be nil", i)
		}
	}
}
