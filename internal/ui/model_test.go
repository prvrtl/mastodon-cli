package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

func testCfg(user, instance string) *config.Config {
	return &config.Config{
		Accounts: []config.Account{{Username: user, Instance: instance, AccessToken: "t"}},
	}
}

func readyModel(t *testing.T) Model {
	t.Helper()
	m := New(nil, testCfg("me", "inst.example"))
	res, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return res.(Model)
}

func status(id, acct string) mastodon.Status {
	return mastodon.Status{ID: id, Account: mastodon.Account{Acct: acct}}
}

func TestCurrentStreamSpec(t *testing.T) {
	m := readyModel(t)

	m.view, m.currentKind = viewTimeline, mastodon.TimelineHome
	if s, ok := m.currentStreamSpec(); !ok || s.Path != "user" {
		t.Errorf("home -> %+v ok=%v", s, ok)
	}
	m.currentKind = mastodon.TimelineLocal
	if s, _ := m.currentStreamSpec(); s.Path != "public/local" {
		t.Errorf("local -> %s", s.Path)
	}
	m.currentKind = mastodon.TimelinePublic
	if s, _ := m.currentStreamSpec(); s.Path != "public" {
		t.Errorf("public -> %s", s.Path)
	}

	m.view, m.currentListID = viewList, "42"
	if s, ok := m.currentStreamSpec(); !ok || s.Path != "list" || s.Query.Get("list") != "42" {
		t.Errorf("list -> %+v ok=%v", s, ok)
	}
	m.currentListID = ""
	if _, ok := m.currentStreamSpec(); ok {
		t.Error("list with no id should not stream")
	}

	m.view, m.currentTag = viewTag, "golang"
	if s, ok := m.currentStreamSpec(); !ok || s.Path != "hashtag" || s.Query.Get("tag") != "golang" {
		t.Errorf("tag -> %+v ok=%v", s, ok)
	}

	for _, v := range []viewKind{viewAccount, viewBookmarks, viewThread} {
		m.view = v
		if _, ok := m.currentStreamSpec(); ok {
			t.Errorf("view %d should not stream", v)
		}
	}

	m.view = viewNotifications
	if s, ok := m.currentStreamSpec(); !ok || s.Path != "user" {
		t.Errorf("notifications -> %+v ok=%v", s, ok)
	}
}

func TestFeedAcceptsStatusUpdates(t *testing.T) {
	m := readyModel(t)
	accept := map[viewKind]bool{
		viewTimeline: true, viewList: true, viewTag: true,
		viewAccount: false, viewBookmarks: false, viewThread: false, viewNotifications: false,
	}
	for v, want := range accept {
		m.view = v
		if got := m.feedAcceptsStatusUpdates(); got != want {
			t.Errorf("view %d accepts=%v, want %v", v, got, want)
		}
	}
}

func TestHandleStreamEventFiltering(t *testing.T) {

	m := readyModel(t)
	m.view = viewTimeline
	m.handleStreamEvent(mastodon.StreamEvent{Type: mastodon.EventUpdate, Status: ptr(status("1", "a"))})
	if len(m.items) != 1 || !m.items[0].isNew || m.items[0].status.ID != "1" {
		t.Fatalf("update not prepended: %+v", m.items)
	}
	if m.statusByIndex[m.items[0].index] == nil {
		t.Error("streamed status not indexed")
	}

	m2 := readyModel(t)
	m2.view = viewAccount
	m2.handleStreamEvent(mastodon.StreamEvent{Type: mastodon.EventUpdate, Status: ptr(status("1", "a"))})
	if len(m2.items) != 0 {
		t.Errorf("account view should drop status updates, got %d items", len(m2.items))
	}

	m3 := readyModel(t)
	m3.view = viewNotifications
	m3.handleStreamEvent(mastodon.StreamEvent{Type: mastodon.EventNotification,
		Notification: &mastodon.Notification{ID: "n1", Type: "mention", Account: mastodon.Account{Acct: "a"}}})
	if len(m3.items) != 1 {
		t.Errorf("notification not prepended in notifications view: %d", len(m3.items))
	}

	m4 := readyModel(t)
	m4.view = viewTimeline
	m4.handleStreamEvent(mastodon.StreamEvent{Type: mastodon.EventNotification,
		Notification: &mastodon.Notification{ID: "n1", Type: "mention"}})
	if len(m4.items) != 0 {
		t.Errorf("timeline view should not list notifications, got %d", len(m4.items))
	}
}

func TestLookupStatus(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("100", "a"), status("200", "b")})
	if s := m.lookupStatus("1"); s == nil || s.ID != "100" {
		t.Errorf("lookup 1 -> %v", s)
	}
	if s := m.lookupStatus("2"); s == nil || s.ID != "200" {
		t.Errorf("lookup 2 -> %v", s)
	}
	if m.lookupStatus("99") != nil {
		t.Error("out-of-range index should be nil")
	}
	if m.lookupStatus("x") != nil {
		t.Error("non-numeric index should be nil")
	}
}

func TestSubmitRouting(t *testing.T) {
	m := readyModel(t)

	res, cmd := m.submit("hello world")
	mm := res.(Model)
	if cmd != nil || mm.confirmOpen {
		t.Error("plain text should not post directly")
	}
	if !contains(mm.statusLine, "/post") {
		t.Errorf("expected a /post hint, got %q", mm.statusLine)
	}

	res, _ = m.submit("/post hello world")
	if pm := res.(Model); !pm.confirmOpen || pm.pending.text != "hello world" {
		t.Errorf("/post should open the confirm modal: open=%v text=%q", pm.confirmOpen, pm.pending.text)
	}

	res, _ = m.submit("/nope")
	if got := res.(Model).statusLine; got == "" || !contains(got, "Unknown") {
		t.Errorf("unknown command status = %q", got)
	}

	res, _ = m.submit("/whoami")
	if !contains(res.(Model).statusLine, "me") {
		t.Errorf("whoami status = %q", res.(Model).statusLine)
	}

	res, cmd = m.submit("/home")
	if cmd == nil || res.(Model).currentKind != mastodon.TimelineHome || !res.(Model).loading {
		t.Error("/home should load the home timeline")
	}
}

func TestMenuMatching(t *testing.T) {
	m := readyModel(t)

	m.input.SetValue("/li")
	got := m.menuMatches()
	if !containsCmd(got, "/lists") || !containsCmd(got, "/list") {
		t.Errorf("/li should match /lists and /list, got %v", names(got))
	}
	if !m.menuVisible() {
		t.Error("menu should be visible while typing a command")
	}

	m.input.SetValue("/list 1")
	if m.menuMatches() != nil || m.menuVisible() {
		t.Error("menu should hide once an argument is typed")
	}

	m.input.SetValue("hello")
	if m.menuVisible() {
		t.Error("menu should not show for plain text")
	}
}

func TestMenuEnterDoesNotHijack(t *testing.T) {

	m := readyModel(t)
	m.input.SetValue("/home")
	m.menuNavigated = false
	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	mm := res.(Model)
	if cmd == nil || mm.currentKind != mastodon.TimelineHome {
		t.Error("Enter on typed /home should load home, not a highlighted item")
	}
}

func TestMenuEnterAcceptsWhenNavigated(t *testing.T) {
	m := readyModel(t)
	m.input.SetValue("/")
	m.menuNavigated = true
	m.menuIndex = 1
	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if res.(Model).currentKind != mastodon.TimelinePublic {
		t.Errorf("navigated Enter should pick /public, currentKind=%v", res.(Model).currentKind)
	}
}

func TestListPickerKeys(t *testing.T) {
	m := readyModel(t)
	m.lists = []mastodon.List{{ID: "1", Title: "A"}, {ID: "2", Title: "B"}}
	m.listPickerOpen = true
	m.listPickerIndex = 0

	res, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(Model)
	if m.listPickerIndex != 1 {
		t.Errorf("Down -> index %d, want 1", m.listPickerIndex)
	}
	res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(Model)
	if m.listPickerIndex != 0 {
		t.Errorf("Up -> index %d, want 0", m.listPickerIndex)
	}

	res, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.listPickerOpen || m.currentListID != "1" || cmd == nil {
		t.Errorf("Enter should open list 1 and close picker (open=%v id=%q)", m.listPickerOpen, m.currentListID)
	}

	m.listPickerOpen = true
	res, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if res.(Model).listPickerOpen {
		t.Error("Esc should close the picker")
	}
}

func TestUpdateTagLoaded(t *testing.T) {
	m := readyModel(t)
	m.autoStream = false
	res, _ := m.Update(tagLoadedMsg{tag: "golang", statuses: []mastodon.Status{status("1", "a")}})
	mm := res.(Model)
	if mm.view != viewTag || mm.currentTag != "golang" || mm.viewLabel != "#golang" {
		t.Errorf("tag view not set: view=%d tag=%q label=%q", mm.view, mm.currentTag, mm.viewLabel)
	}
	if len(mm.items) != 1 {
		t.Errorf("expected 1 item, got %d", len(mm.items))
	}
}

func TestUpdateAccountLoadedStartsPolling(t *testing.T) {
	m := readyModel(t)
	res, cmd := m.Update(accountLoadedMsg{acct: "bob", accountID: "7",
		statuses: []mastodon.Status{status("11", "bob"), status("10", "bob")}})
	mm := res.(Model)
	if mm.view != viewAccount || mm.currentAcctID != "7" {
		t.Errorf("account view not set: view=%d id=%q", mm.view, mm.currentAcctID)
	}
	if mm.acctSinceID != "11" {
		t.Errorf("acctSinceID = %q, want 11 (newest)", mm.acctSinceID)
	}
	if cmd == nil {
		t.Error("account load should schedule a poll command")
	}
}

func TestUpdateBookmarksLoaded(t *testing.T) {
	m := readyModel(t)
	res, _ := m.Update(bookmarksLoadedMsg{statuses: []mastodon.Status{status("1", "a")}})
	if res.(Model).view != viewBookmarks {
		t.Error("bookmarks view not set")
	}
}

func TestUpdateContextBuildsThread(t *testing.T) {
	m := readyModel(t)
	focus := status("2", "a")
	res, _ := m.Update(contextLoadedMsg{
		focus: &focus,
		ctx: &mastodon.Context{
			Ancestors:   []mastodon.Status{status("1", "a")},
			Descendants: []mastodon.Status{status("3", "b"), status("4", "c")},
		},
	})
	mm := res.(Model)
	if mm.view != viewThread {
		t.Errorf("thread view not set: %d", mm.view)
	}

	if len(mm.items) != 5 {
		t.Errorf("thread items = %d, want 5", len(mm.items))
	}
}

func TestAccountPollMsgPrepends(t *testing.T) {
	m := readyModel(t)
	m.view = viewAccount
	m.currentAcct = "bob"
	m.pollGen = 3

	res, cmd := m.Update(accountPollMsg{gen: 3, statuses: []mastodon.Status{status("11", "bob"), status("10", "bob")}})
	mm := res.(Model)
	if len(mm.items) != 2 {
		t.Fatalf("expected 2 prepended items, got %d", len(mm.items))
	}
	if mm.items[0].status.ID != "11" {
		t.Errorf("newest should be on top, got %s", mm.items[0].status.ID)
	}
	if mm.acctSinceID != "11" {
		t.Errorf("acctSinceID = %q, want 11", mm.acctSinceID)
	}
	if cmd == nil {
		t.Error("polling should reschedule itself")
	}

	res, _ = mm.Update(accountPollMsg{gen: 1, statuses: []mastodon.Status{status("99", "bob")}})
	if len(res.(Model).items) != 2 {
		t.Error("stale poll generation should be ignored")
	}
}

func TestApplyUpdate(t *testing.T) {
	m := readyModel(t)
	m.setTimeline(mastodon.TimelineHome, []mastodon.Status{status("5", "a")})
	m.applyUpdate(&mastodon.Status{ID: "5", Favourited: true, FavouritesCount: 9, Bookmarked: true})
	s := m.statusByIndex[1]
	if s == nil || !s.Favourited || s.FavouritesCount != 9 || !s.Bookmarked {
		t.Errorf("applyUpdate did not sync flags: %+v", s)
	}
}

func TestRefreshCurrentByView(t *testing.T) {
	m := readyModel(t)
	m.view = viewThread
	res, _ := m.refreshCurrent()
	if res.(Model).loading {
		t.Error("thread refresh should not enter loading (it's a snapshot)")
	}

	m.view, m.currentTag = viewTag, "go"
	res, cmd := m.refreshCurrent()
	if !res.(Model).loading || cmd == nil {
		t.Error("tag refresh should load")
	}
}

func TestRelTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "now"},
		{3 * time.Minute, "3m"},
		{2 * time.Hour, "2h"},
		{50 * time.Hour, "2d"},
	}
	for _, c := range cases {
		if got := relTime(now.Add(-c.d)); got != c.want {
			t.Errorf("relTime(-%s) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestMarks(t *testing.T) {
	if boostMark(&mastodon.Status{Reblogged: true}) != "↻✓" || boostMark(&mastodon.Status{}) != "↻" {
		t.Error("boostMark wrong")
	}
	if favMark(&mastodon.Status{Favourited: true}) != "★" || favMark(&mastodon.Status{}) != "☆" {
		t.Error("favMark wrong")
	}
}

func TestTruncateAndPad(t *testing.T) {
	if truncate("hello world", 5) != "hell…" {
		t.Errorf("truncate = %q", truncate("hello world", 5))
	}
	if truncate("hi", 5) != "hi" {
		t.Error("short string should be unchanged")
	}
	if got := padTo("ab", 5); len([]rune(got)) != 5 {
		t.Errorf("padTo length = %d, want 5", len([]rune(got)))
	}
}

func TestRenderPoll(t *testing.T) {
	p := &mastodon.Poll{VotesCount: 10, Voted: true, Options: []mastodon.PollOption{
		{Title: "Yes", VotesCount: 7}, {Title: "No", VotesCount: 3},
	}}
	out := renderPoll(p, 60)
	if !contains(out, "Yes") || !contains(out, "70%") || !contains(out, "voted") {
		t.Errorf("poll render missing fields:\n%s", out)
	}
}

func ptr(s mastodon.Status) *mastodon.Status { return &s }

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func names(cs []slashCmd) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}

func containsCmd(cs []slashCmd, name string) bool {
	for _, c := range cs {
		if c.name == name {
			return true
		}
	}
	return false
}
