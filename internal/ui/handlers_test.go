package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestStatusActionUsageAndRouting(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "a")})

	res, cmd := m.submit("/boost")
	if cmd != nil || !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Errorf("/boost without arg: status=%q cmd=%v", res.(Model).statusLine, cmd)
	}

	res, cmd = m.submit("/boost 9")
	if cmd != nil || !strings.Contains(res.(Model).statusLine, "No post") {
		t.Errorf("/boost 9: status=%q", res.(Model).statusLine)
	}

	res, cmd = m.submit("/boost 1")
	if cmd == nil || !res.(Model).loading {
		t.Error("/boost 1 should issue a command and set loading")
	}

	if _, c := m.submit("/fav 1"); c == nil {
		t.Error("/fav 1 should issue a command")
	}
	if _, c := m.submit("/bookmark 1"); c == nil {
		t.Error("/bookmark 1 should issue a command")
	}
}

func TestReplyParsing(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "a")})

	if res, cmd := m.submit("/reply 1"); cmd != nil || !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Errorf("/reply with no text should show usage, got %q", res.(Model).statusLine)
	}
	res, _ := m.submit("/reply 1 nice post")
	mm := res.(Model)
	if !mm.confirmOpen || mm.pending.text != "nice post" || mm.pending.replyTo != "5" {
		t.Errorf("/reply should open confirm with reply target: %+v", mm.pending)
	}
}

func TestFollowAndThreadUsage(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "a")})

	if _, cmd := m.submit("/follow 1"); cmd == nil {
		t.Error("/follow 1 should issue a command")
	}
	if res, _ := m.submit("/follow"); !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/follow without arg should show usage")
	}
	if _, cmd := m.submit("/thread 1"); cmd == nil {
		t.Error("/thread 1 should issue a command")
	}
	if res, _ := m.submit("/thread"); !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/thread without arg should show usage")
	}
}

func TestStreamTogglePreference(t *testing.T) {
	m := readyModel(t)

	m.streaming = true
	res, _ := m.submit("/stream off")
	mm := res.(Model)
	if mm.autoStream || mm.streaming {
		t.Errorf("/stream off should disable auto-streaming (autoStream=%v streaming=%v)", mm.autoStream, mm.streaming)
	}
	if !strings.Contains(mm.statusLine, "paused") {
		t.Errorf("status line = %q, want it to mention paused", mm.statusLine)
	}

}

func TestTagAndAccountUsage(t *testing.T) {
	m := readyModel(t)
	if res, cmd := m.submit("/tag"); cmd != nil || !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/tag without arg should show usage")
	}
	if _, cmd := m.submit("/tag golang"); cmd == nil {
		t.Error("/tag golang should issue a command")
	}
	if res, cmd := m.submit("/account"); cmd != nil || !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/account without arg should show usage")
	}
	if _, cmd := m.submit("/account @bob"); cmd == nil {
		t.Error("/account @bob should issue a command")
	}
}

func TestUpdateTimelineAndNotifications(t *testing.T) {
	m := readyModel(t)
	m.autoStream = false

	res, _ := m.Update(statusesLoadedMsg{kind: mastodon.TimelinePublic,
		statuses: []mastodon.Status{status("1", "a"), status("2", "b")}})
	mm := res.(Model)
	if mm.view != viewTimeline || mm.currentKind != mastodon.TimelinePublic || len(mm.items) != 2 {
		t.Errorf("timeline load: view=%d kind=%v items=%d", mm.view, mm.currentKind, len(mm.items))
	}

	res, _ = mm.Update(notificationsLoadedMsg{notifs: []mastodon.Notification{
		{ID: "n1", Type: "follow", Account: mastodon.Account{Acct: "a"}},
	}})
	nm := res.(Model)
	if nm.view != viewNotifications || len(nm.items) != 1 {
		t.Errorf("notifications load: view=%d items=%d", nm.view, len(nm.items))
	}
}

func TestUpdatePostedAndAction(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "a")})

	res, _ := m.Update(postedMsg{status: ptr(mastodon.Status{ID: "9", Content: "<p>hi</p>"})})
	if !strings.Contains(res.(Model).statusLine, "Posted") {
		t.Errorf("posted status line = %q", res.(Model).statusLine)
	}

	res, _ = m.Update(postedMsg{err: errString("boom")})
	if !hasErrorItem(res.(Model)) {
		t.Error("failed post should add an error item")
	}

	res, _ = m.Update(actionMsg{ok: "Boosted", updated: &mastodon.Status{ID: "5", Reblogged: true, ReblogsCount: 3}})
	mm := res.(Model)
	if !strings.Contains(mm.statusLine, "Boosted") || !mm.statusByIndex[1].Reblogged {
		t.Error("action success should set status line and apply update")
	}
}

func TestUpdateStreamMsgGeneration(t *testing.T) {
	m := readyModel(t)
	m.view = viewTimeline
	m.streamGen = 7
	ch := make(chan mastodon.StreamEvent)

	res, cmd := m.Update(streamMsg{gen: 7, ch: ch,
		ev: mastodon.StreamEvent{Type: mastodon.EventUpdate, Status: ptr(status("1", "a"))}})
	if len(res.(Model).items) != 1 || cmd == nil {
		t.Error("matching-gen stream msg should apply and reschedule")
	}

	res, cmd = m.Update(streamMsg{gen: 1, ch: ch,
		ev: mastodon.StreamEvent{Type: mastodon.EventUpdate, Status: ptr(status("2", "b"))}})
	if len(res.(Model).items) != 0 || cmd != nil {
		t.Error("stale-gen stream msg should be dropped")
	}
}

func TestTickReschedules(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("1", "a")})
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Error("tick should reschedule itself so timestamps keep updating")
	}
}

func TestFeedItemRendering(t *testing.T) {
	rich := mastodon.Status{
		ID:              "1",
		Content:         "<p>hello <a href='x'>link</a></p>",
		Account:         mastodon.Account{DisplayName: "Ada", Acct: "ada@x"},
		RepliesCount:    2,
		ReblogsCount:    5,
		FavouritesCount: 9,
		SpoilerText:     "CW: spoiler",
		Bookmarked:      true,
		MediaAttachments: []mastodon.Media{
			{Type: "image", Description: "a cat"},
		},
	}
	out := plain(feedItem{kind: kindStatus, status: &rich, index: 3}.render(70))
	for _, want := range []string{"[3]", "Ada", "@ada@x", "hello link", "CW: spoiler", "a cat", "🔖"} {
		if !strings.Contains(out, want) {
			t.Errorf("status render missing %q in:\n%s", want, out)
		}
	}

	boost := mastodon.Status{Account: mastodon.Account{DisplayName: "Booster"}, Reblog: &rich}
	if !strings.Contains(plain(feedItem{kind: kindStatus, status: &boost, index: 1}.render(70)), "boosted by Booster") {
		t.Error("boost wrapper not rendered")
	}

	notif := feedItem{kind: kindNotif, index: 4, notif: &mastodon.Notification{
		Type: "favourite", Account: mastodon.Account{DisplayName: "Bob", Acct: "bob"},
		Status: &mastodon.Status{Content: "<p>quoted</p>"},
	}}
	no := plain(notif.render(70))
	if !strings.Contains(no, "favourited") || !strings.Contains(no, "Bob") || !strings.Contains(no, "quoted") {
		t.Errorf("notification render wrong:\n%s", no)
	}

	if !strings.Contains(plain(feedItem{kind: kindSystem, text: "note"}.render(70)), "note") {
		t.Error("system render")
	}
	if !strings.Contains(plain(feedItem{kind: kindError, text: "bad"}.render(70)), "bad") {
		t.Error("error render")
	}
}

func TestBareNumberOpensActionMenu(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("100", "alice")})

	res, cmd := m.submit("1")
	mm := res.(Model)
	if !mm.actionOpen || cmd != nil {
		t.Fatalf("bare number should open the action menu (open=%v)", mm.actionOpen)
	}
	if mm.actionTarget == nil || mm.actionTarget.ID != "100" {
		t.Errorf("action target not set to post 1")
	}
	if mm.confirmOpen {
		t.Error("a number must never start a post")
	}

	res, _ = m.submit("9")
	if res.(Model).actionOpen || !strings.Contains(res.(Model).statusLine, "No post") {
		t.Error("unknown number should report no post")
	}
}

func TestActionMenuRunActions(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("100", "alice")})
	res, _ := m.submit("1")
	m = res.(Model)

	r := m
	res, _ = r.runAction(3)
	rm := res.(Model)
	if rm.actionOpen || rm.input.Value() != "/reply 1 " {
		t.Errorf("reply action should prefill /reply, got open=%v val=%q", rm.actionOpen, rm.input.Value())
	}

	res, cmd := m.runAction(0)
	if res.(Model).actionOpen || cmd == nil {
		t.Error("boost action should issue a command and close the menu")
	}
}

func TestSearchAndPollCommands(t *testing.T) {
	m := readyModel(t)
	if res, _ := m.submit("/search"); !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/search without query should show usage")
	}
	if _, cmd := m.submit("/search golang"); cmd == nil {
		t.Error("/search <q> should issue a command")
	}
	if res, _ := m.submit("/poll one option only"); !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/poll without 2 options should show usage")
	}
	if _, cmd := m.submit("/poll Best lang? | Go | Rust"); cmd == nil {
		t.Error("/poll with options should issue a command")
	}
}

func TestSearchResultsRender(t *testing.T) {
	m := readyModel(t)
	res, _ := m.Update(searchLoadedMsg{query: "go", res: &mastodon.SearchResults{
		Accounts: []mastodon.Account{{Acct: "gopher"}},
		Hashtags: []mastodon.Tag{{Name: "golang"}},
		Statuses: []mastodon.Status{status("5", "a")},
	}})
	mm := res.(Model)
	if mm.viewLabel != "search: go" || len(mm.items) != 3 {
		t.Errorf("search results: label=%q items=%d", mm.viewLabel, len(mm.items))
	}
}

func TestActionMenuVoteAndModeration(t *testing.T) {
	m := readyModel(t)
	s := status("100", "alice")
	s.Poll = &mastodon.Poll{ID: "p", Options: []mastodon.PollOption{{Title: "Yes"}, {Title: "No"}}}
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{s})
	res, _ := m.submit("1")
	m = res.(Model)

	items := m.actionMenuItems()
	if items[0].id != "vote" {
		t.Errorf("votable poll should put Vote first, got %q", items[0].id)
	}

	res, _ = m.runAction(0)
	if !res.(Model).pollVoteOpen {
		t.Error("vote action should open the poll modal")
	}

	muteIdx := -1
	for i, it := range items {
		if it.id == "mute" {
			muteIdx = i
		}
	}
	if muteIdx < 0 {
		t.Fatal("mute action missing")
	}
	if _, cmd := m.runAction(muteIdx); cmd == nil {
		t.Error("mute should issue a command")
	}
}

func TestPollVoteModalKeys(t *testing.T) {
	m := readyModel(t)
	s := status("100", "alice")
	s.Poll = &mastodon.Poll{ID: "p", Options: []mastodon.PollOption{{Title: "Yes"}, {Title: "No"}}}
	mm := m
	mm.pollVoteOpen = true
	mm.pollVoteTarget = &s
	res, _ := mm.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if res.(Model).pollVoteIndex != 1 {
		t.Error("Down should move poll selection")
	}
	res, cmd := res.(Model).handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if res.(Model).pollVoteOpen || cmd == nil {
		t.Error("Enter should vote and close the poll modal")
	}
}

func TestAccountSwitch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	m := New(nil, &config.Config{Accounts: []config.Account{
		{Username: "a", Instance: "one.example", AccessToken: "1"},
		{Username: "b", Instance: "two.example", AccessToken: "2"},
	}})
	res, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = res.(Model)

	res, _ = m.submit("/accounts")
	if !hasSystemContaining(res.(Model), "two.example") {
		t.Error("/accounts should list accounts")
	}

	res, cmd := m.submit("/switch 2")
	mm := res.(Model)
	if mm.cfg.Active != 1 || mm.cfg.Current().Username != "b" || cmd == nil {
		t.Errorf("/switch 2 should activate account b (active=%d)", mm.cfg.Active)
	}

	fresh := New(nil, &config.Config{Accounts: []config.Account{{Username: "a", Instance: "one.example", AccessToken: "1"}}})
	res2, _ := fresh.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	res2, _ = res2.(Model).submit("/switch 9")
	if res2.(Model).cfg.Active != 0 || !strings.Contains(res2.(Model).statusLine, "No account") {
		t.Error("/switch with bad index should be rejected")
	}
}

func hasSystemContaining(m Model, sub string) bool {
	for _, it := range m.items {
		if it.kind == kindSystem && strings.Contains(it.text, sub) {
			return true
		}
	}
	return false
}

func TestPlainTextDoesNotPost(t *testing.T) {
	m := readyModel(t)
	res, cmd := m.submit("just some text")
	mm := res.(Model)
	if mm.confirmOpen || mm.loading || cmd != nil {
		t.Error("plain text must not post")
	}
}

func TestPostConfirmConfirms(t *testing.T) {
	m := readyModel(t)
	m.input.SetValue("/post hello there")

	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if !m.confirmOpen || cmd != nil {
		t.Fatalf("Enter on /post should open confirm modal (open=%v)", m.confirmOpen)
	}
	if m.input.Value() != "" {
		t.Error("input should be cleared while confirming")
	}

	res, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.confirmOpen || !m.loading || cmd == nil {
		t.Errorf("confirming should post: open=%v loading=%v cmd=%v", m.confirmOpen, m.loading, cmd)
	}
}

func TestPostConfirmWithY(t *testing.T) {
	m := readyModel(t)
	res, _ := m.submit("/post yo")
	m = res.(Model)
	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = res.(Model)
	if m.confirmOpen || cmd == nil {
		t.Error("'y' should confirm the post")
	}
}

func TestPostConfirmCancelRestoresText(t *testing.T) {
	m := readyModel(t)
	res, _ := m.submit("/post draft toot")
	m = res.(Model)

	res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if m.confirmOpen {
		t.Error("Esc should close the confirm modal")
	}
	if m.input.Value() != "draft toot" {
		t.Errorf("cancel should restore the draft, got %q", m.input.Value())
	}
	if !strings.Contains(m.statusLine, "cancelled") {
		t.Errorf("status line = %q, want it to mention cancelled", m.statusLine)
	}
}

func TestPostConfirmCancelWithN(t *testing.T) {
	m := readyModel(t)
	res, _ := m.submit("/post nope draft")
	m = res.(Model)
	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = res.(Model)
	if m.confirmOpen || cmd != nil {
		t.Error("'n' should cancel the post")
	}
	if m.input.Value() != "nope draft" {
		t.Error("'n' cancel should restore the draft text")
	}
}

func TestConfirmModalRendered(t *testing.T) {
	m := readyModel(t)
	res, _ := m.submit("/post preview me")
	m = res.(Model)
	out := plain(m.View())
	if !strings.Contains(out, "Post this toot?") || !strings.Contains(out, "preview me") {
		t.Errorf("confirm modal not rendered:\n%s", out)
	}
}

func TestReplyConfirmModalShowsTarget(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "alice")})
	res, _ := m.submit("/reply 1 hi")
	m = res.(Model)
	if !strings.Contains(plain(m.renderConfirm()), "Reply to @alice") {
		t.Error("reply confirmation should name the target account")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func hasErrorItem(m Model) bool {
	for _, it := range m.items {
		if it.kind == kindError {
			return true
		}
	}
	return false
}
