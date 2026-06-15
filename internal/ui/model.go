package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

var Version = "dev"

type viewKind int

const (
	viewTimeline viewKind = iota
	viewList
	viewTag
	viewAccount
	viewBookmarks
	viewThread
	viewNotifications
)

type Model struct {
	client *mastodon.Client
	cfg    *config.Config

	vp      viewport.Model
	input   textarea.Model
	spinner spinner.Model

	items         []feedItem
	statusByIndex map[int]*mastodon.Status
	notifByIndex  map[int]*mastodon.Notification
	nextIndex     int

	width, height int
	ready         bool
	loading       bool
	statusLine    string

	currentKind     mastodon.TimelineKind
	viewLabel       string
	lists           []mastodon.List
	currentListID   string
	currentListName string
	currentTag      string
	currentAcctID   string
	currentAcct     string
	acctSinceID     string
	pollGen         int

	view         viewKind
	autoStream   bool
	streaming    bool
	streamCh     <-chan mastodon.StreamEvent
	streamCancel context.CancelFunc
	streamGen    int

	animFrame int
	animGen   int

	menuIndex     int
	menuDismissed bool
	menuNavigated bool

	listPickerOpen  bool
	listPickerIndex int

	confirmOpen bool
	pending     pendingPost

	actionOpen   bool
	actionIndex  int
	actionNum    int
	actionTarget *mastodon.Status

	pollVoteOpen   bool
	pollVoteIndex  int
	pollVoteTarget *mastodon.Status
}

type pendingPost struct {
	text       string
	replyTo    string
	visibility string
	summary    string
}

func New(client *mastodon.Client, cfg *config.Config) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a post # for actions · /post <text> to toot · /help"
	ta.Prompt = "❯ "
	ta.ShowLineNumbers = false
	ta.CharLimit = 500
	ta.SetHeight(1)
	ta.Focus()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colMasto).Bold(true)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(colMasto).Bold(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colMasto)

	return Model{
		client:        client,
		cfg:           cfg,
		input:         ta,
		spinner:       sp,
		statusByIndex: map[int]*mastodon.Status{},
		notifByIndex:  map[int]*mastodon.Notification{},
		nextIndex:     1,
		currentKind:   mastodon.TimelineHome,
		viewLabel:     "home",
		autoStream:    true,
		statusLine:    "Loading home timeline…",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadTimelineCmd(m.client, mastodon.TimelineHome),
		textarea.Blink,
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		next, cmd := m.handleKey(msg)
		if nm, ok := next.(Model); ok {
			nm.syncInputHeight()
			return nm, cmd
		}
		return next, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:

		m.refreshViewport()
		return m, tickCmd()

	case animTickMsg:

		if msg.gen != m.animGen || !m.streaming {
			return m, nil
		}
		m.animFrame++
		return m, animCmd(msg.gen)

	case statusesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.statusLine = "Failed to load timeline"
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.currentListID, m.currentListName = "", ""
		m.view = viewTimeline
		m.viewLabel = string(msg.kind)
		m.setTimeline(msg.kind, msg.statuses)
		m.statusLine = fmt.Sprintf("Loaded %d posts from %s", len(msg.statuses), msg.kind)
		m.refreshViewport()
		m.vp.GotoTop()
		return m, m.startStream()

	case notificationsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.currentListID, m.currentListName = "", ""
		m.view = viewNotifications
		m.viewLabel = "notifications"
		m.setNotifications(msg.notifs)
		m.statusLine = fmt.Sprintf("Loaded %d notifications", len(msg.notifs))
		m.refreshViewport()
		m.vp.GotoTop()
		return m, m.startStream()

	case listsLoadedMsg:
		m.loading = false
		switch {
		case msg.err != nil:
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoBottom()
		case len(msg.lists) == 0:
			m.lists = nil
			m.addSystem(kindSystem, renderListMenu(nil))
			m.refreshViewport()
			m.vp.GotoBottom()
			m.statusLine = "You have no lists"
		default:
			m.lists = msg.lists
			m.listPickerOpen = true
			m.listPickerIndex = 0
			m.statusLine = "Select a list to open"
		}
		return m, nil

	case listTimelineLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.statusLine = "Failed to load list"
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.view = viewList
		m.viewLabel = "list: " + msg.title
		m.setTimeline(mastodon.TimelineHome, msg.statuses)
		m.statusLine = fmt.Sprintf("Loaded %d posts from “%s”", len(msg.statuses), msg.title)
		m.refreshViewport()
		m.vp.GotoTop()
		return m, m.startStream()

	case tagLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.view = viewTag
		m.currentTag = msg.tag
		m.currentListID, m.currentListName = "", ""
		m.viewLabel = "#" + msg.tag
		m.setTimeline(mastodon.TimelineHome, msg.statuses)
		m.statusLine = fmt.Sprintf("Loaded %d posts for #%s", len(msg.statuses), msg.tag)
		m.refreshViewport()
		m.vp.GotoTop()
		return m, m.startStream()

	case accountLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.stopStream()
		m.view = viewAccount
		m.currentAcctID, m.currentAcct = msg.accountID, msg.acct
		m.currentListID, m.currentListName = "", ""
		m.viewLabel = "@" + msg.acct
		m.setTimeline(mastodon.TimelineHome, msg.statuses)
		if len(msg.statuses) > 0 {
			m.acctSinceID = msg.statuses[0].ID
		}
		m.statusLine = fmt.Sprintf("Loaded %d posts from @%s — auto-refreshing", len(msg.statuses), msg.acct)
		m.refreshViewport()
		m.vp.GotoTop()

		m.pollGen++
		return m, scheduleAccountPoll(m.pollGen)

	case bookmarksLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.stopStream()
		m.view = viewBookmarks
		m.currentListID, m.currentListName = "", ""
		m.viewLabel = "bookmarks"
		m.setTimeline(mastodon.TimelineHome, msg.statuses)
		m.statusLine = fmt.Sprintf("Loaded %d bookmarks", len(msg.statuses))
		m.refreshViewport()
		m.vp.GotoTop()
		return m, nil

	case contextLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
		} else {
			m.showThread(msg.focus, msg.ctx)
		}
		m.refreshViewport()
		m.vp.GotoTop()
		return m, nil

	case accountPollFireMsg:
		if msg.gen != m.pollGen || m.view != viewAccount {
			return m, nil
		}
		return m, pollAccountCmd(m.client, m.currentAcctID, m.acctSinceID, msg.gen)

	case accountPollMsg:
		if msg.gen != m.pollGen || m.view != viewAccount {
			return m, nil
		}
		atTop := m.vp.AtTop()

		for i := len(msg.statuses) - 1; i >= 0; i-- {
			s := msg.statuses[i]
			idx := m.nextIndex
			m.nextIndex++
			sc := s
			m.statusByIndex[idx] = &sc
			m.items = append([]feedItem{{kind: kindStatus, status: &sc, index: idx, isNew: true}}, m.items...)
			m.acctSinceID = s.ID
		}
		if len(msg.statuses) > 0 {
			m.statusLine = fmt.Sprintf("● %d new from @%s", len(msg.statuses), m.currentAcct)
			m.refreshViewport()
			if atTop {
				m.vp.GotoTop()
			}
		}
		return m, scheduleAccountPoll(msg.gen)

	case postedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, "Post failed: "+msg.err.Error())
		} else {
			m.statusLine = "Posted ✓"
			m.addSystem(kindSystem, "You posted: "+truncate(htmlToText(msg.status.Content), 60))
		}
		m.refreshViewport()
		m.vp.GotoBottom()
		return m, nil

	case actionMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoBottom()
		} else {
			m.statusLine = msg.ok
			if msg.updated != nil {
				m.applyUpdate(msg.updated)
			}
			m.refreshViewport()
		}
		return m, nil

	case pollVotedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoBottom()
		} else {
			m.applyPoll(msg.statusID, msg.poll)
			m.statusLine = "Vote recorded ✓"
			m.refreshViewport()
		}
		return m, nil

	case searchLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.addSystem(kindError, msg.err.Error())
			m.refreshViewport()
			m.vp.GotoTop()
			return m, nil
		}
		m.stopStream()
		m.view = viewThread
		m.viewLabel = "search: " + msg.query
		m.showSearchResults(msg.res)
		m.refreshViewport()
		m.vp.GotoTop()
		return m, nil

	case streamMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		atTop := m.vp.AtTop()
		m.handleStreamEvent(msg.ev)
		m.refreshViewport()
		if atTop {
			m.vp.GotoTop()
		}
		return m, waitForStreamCmd(msg.ch, msg.gen)
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

	if m.pollVoteOpen {
		opts := m.pollVoteTarget.Poll.Options
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			m.pollVoteIndex = (m.pollVoteIndex - 1 + len(opts)) % len(opts)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.pollVoteIndex = (m.pollVoteIndex + 1) % len(opts)
			return m, nil
		case tea.KeyEnter:
			p := m.pollVoteTarget.Poll
			s := m.pollVoteTarget
			m.pollVoteOpen = false
			m.loading = true
			m.statusLine = "Voting…"
			return m, votePollCmd(m.client, s.ID, p.ID, []int{m.pollVoteIndex})
		case tea.KeyEsc:
			m.pollVoteOpen = false
			m.statusLine = "Cancelled"
			return m, nil
		}
		return m, nil
	}

	if m.actionOpen {
		items := m.actionMenuItems()
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			m.actionIndex = (m.actionIndex - 1 + len(items)) % len(items)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.actionIndex = (m.actionIndex + 1) % len(items)
			return m, nil
		case tea.KeyEnter:
			return m.runAction(m.actionIndex)
		case tea.KeyEsc:
			m.actionOpen = false
			m.statusLine = "Cancelled"
			return m, nil
		}
		return m, nil
	}

	if m.confirmOpen {
		switch {
		case msg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case msg.Type == tea.KeyEnter || keyRune(msg, 'y'):
			m.confirmOpen = false
			m.input.Reset()
			m.loading = true
			m.statusLine = "Posting…"
			p := m.pending
			return m, postCmd(m.client, p.text, p.replyTo, p.visibility)
		case msg.Type == tea.KeyEsc || keyRune(msg, 'n'):
			m.confirmOpen = false
			m.input.SetValue(m.pending.text)
			m.input.CursorEnd()
			m.statusLine = "Post cancelled"
			return m, nil
		}
		return m, nil
	}

	if m.listPickerOpen {
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			m.listPickerIndex = (m.listPickerIndex - 1 + len(m.lists)) % len(m.lists)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.listPickerIndex = (m.listPickerIndex + 1) % len(m.lists)
			return m, nil
		case tea.KeyEnter:
			l := m.lists[m.listPickerIndex]
			m.listPickerOpen = false
			m.currentListID, m.currentListName = l.ID, l.Title
			m.loading = true
			m.statusLine = "Loading list “" + l.Title + "”…"
			return m, loadListTimelineCmd(m.client, l.ID, l.Title)
		case tea.KeyEsc:
			m.listPickerOpen = false
			m.statusLine = "Cancelled"
			return m, nil
		}
		return m, nil
	}

	if m.menuVisible() {
		matches := m.menuMatches()
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP:
			m.menuIndex = (m.menuIndex - 1 + len(matches)) % len(matches)
			m.menuNavigated = true
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.menuIndex = (m.menuIndex + 1) % len(matches)
			m.menuNavigated = true
			return m, nil
		case tea.KeyTab:
			m.completeMenu(matches)
			return m, nil
		case tea.KeyEsc:
			m.menuDismissed = true
			return m, nil
		case tea.KeyEnter:

			if m.menuNavigated {
				return m.acceptMenu(matches)
			}
		}
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyCtrlJ:

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case tea.KeyEnter:
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}
		m.input.Reset()
		return m.submit(text)
	case tea.KeyPgUp:
		m.vp.HalfViewUp()
		return m, nil
	case tea.KeyPgDown:
		m.vp.HalfViewDown()
		return m, nil
	case tea.KeyCtrlU:
		m.vp.GotoTop()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.menuDismissed = false
	m.menuNavigated = false
	m.menuIndex = 0
	return m, cmd
}

func (m *Model) completeMenu(matches []slashCmd) {
	sel := matches[m.menuIndex]
	m.input.SetValue(sel.name + " ")
	m.input.CursorEnd()
	m.menuDismissed = true
}

func (m Model) acceptMenu(matches []slashCmd) (tea.Model, tea.Cmd) {
	sel := matches[m.menuIndex]
	if sel.takesArg {
		m.input.SetValue(sel.name + " ")
		m.input.CursorEnd()
		m.menuDismissed = true
		return m, nil
	}
	m.input.Reset()
	m.menuDismissed = true
	return m.submit(sel.name)
}

func keyRune(msg tea.KeyMsg, r rune) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return false
	}
	got := msg.Runes[0]
	return got == r || got == r-32
}

func (m Model) confirmPost(text, replyTo, visibility, summary string) (tea.Model, tea.Cmd) {
	m.pending = pendingPost{text: text, replyTo: replyTo, visibility: visibility, summary: summary}
	m.confirmOpen = true
	m.statusLine = "Review your toot — Enter to post, Esc to cancel"
	return m, nil
}

func (m Model) submit(text string) (tea.Model, tea.Cmd) {
	if !strings.HasPrefix(text, "/") {

		if isAllDigits(text) {
			if s := m.lookupStatus(text); s != nil {
				n, _ := strconv.Atoi(text)
				return m.openActionMenu(n, s)
			}
			m.statusLine = "No post numbered " + text
			return m, nil
		}

		m.statusLine = "To post a toot, type:  /post <your text>"
		return m, nil
	}

	fields := strings.Fields(text)
	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "/help", "/h", "/?":
		m.addSystem(kindSystem, helpText)
		m.refreshViewport()
		m.vp.GotoBottom()
		return m, nil

	case "/home":
		return m.loadTimeline(mastodon.TimelineHome)
	case "/public", "/fed":
		return m.loadTimeline(mastodon.TimelinePublic)
	case "/local":
		return m.loadTimeline(mastodon.TimelineLocal)

	case "/notifications", "/n":
		m.loading = true
		m.statusLine = "Loading notifications…"
		return m, loadNotificationsCmd(m.client)

	case "/lists":
		return m.openListPicker()

	case "/list", "/l":
		return m.handleList(args)

	case "/tag", "/hashtag", "/t":
		if len(args) < 1 {
			m.statusLine = "Usage: /tag <hashtag>"
			return m, nil
		}
		m.loading = true
		m.statusLine = "Loading #" + args[0] + "…"
		return m, loadTagCmd(m.client, args[0])

	case "/account", "/user", "/u":
		if len(args) < 1 {
			m.statusLine = "Usage: /account <@user or user@domain>"
			return m, nil
		}
		m.loading = true
		m.statusLine = "Looking up " + args[0] + "…"
		return m, loadAccountCmd(m.client, args[0])

	case "/bookmarks", "/bm":
		m.loading = true
		m.statusLine = "Loading bookmarks…"
		return m, loadBookmarksCmd(m.client)

	case "/bookmark", "/save":
		return m.statusAction(args, bookmarkCmd)

	case "/thread", "/context":
		return m.handleThread(args)

	case "/refresh", "/r":
		return m.refreshCurrent()

	case "/post", "/toot":
		body := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
		if body == "" {
			m.statusLine = "Usage: /post <text>"
			return m, nil
		}
		return m.confirmPost(body, "", "", "New toot")

	case "/search", "/s":
		if len(args) < 1 {
			m.statusLine = "Usage: /search <query>"
			return m, nil
		}
		q := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
		m.loading = true
		m.statusLine = "Searching…"
		return m, searchCmd(m.client, q)

	case "/poll":
		return m.handlePoll(text, fields[0])

	case "/reply":
		return m.handleReply(text, args)

	case "/boost", "/rt":
		return m.statusAction(args, boostCmd)
	case "/fav", "/like", "/star":
		return m.statusAction(args, favCmd)

	case "/follow":
		return m.handleFollow(args)

	case "/open":
		return m.handleOpen(args)

	case "/stream":
		return m.handleStream(args)

	case "/whoami":
		a := m.cfg.Current()
		m.statusLine = fmt.Sprintf("@%s on %s  ·  %d account(s) — /accounts", a.Username, a.Instance, len(m.cfg.Accounts))
		return m, nil

	case "/accounts":
		m.addSystem(kindSystem, renderAccounts(m.cfg))
		m.refreshViewport()
		m.vp.GotoBottom()
		m.statusLine = fmt.Sprintf("%d account(s) — /switch <n>, or  md login  to add one", len(m.cfg.Accounts))
		return m, nil

	case "/switch":
		return m.handleSwitch(args)

	case "/quit", "/q", "/exit":
		return m, tea.Quit

	default:
		m.statusLine = "Unknown command: " + cmd + " (try /help)"
		return m, nil
	}
}

func (m Model) loadTimeline(kind mastodon.TimelineKind) (tea.Model, tea.Cmd) {
	m.loading = true
	m.currentKind = kind
	m.statusLine = fmt.Sprintf("Loading %s…", kind)
	return m, loadTimelineCmd(m.client, kind)
}

func (m Model) refreshCurrent() (tea.Model, tea.Cmd) {
	m.loading = true
	switch m.view {
	case viewList:
		m.statusLine = "Refreshing list…"
		return m, loadListTimelineCmd(m.client, m.currentListID, m.currentListName)
	case viewTag:
		m.statusLine = "Refreshing #" + m.currentTag + "…"
		return m, loadTagCmd(m.client, m.currentTag)
	case viewAccount:
		m.statusLine = "Refreshing @" + m.currentAcct + "…"
		return m, loadAccountCmd(m.client, m.currentAcct)
	case viewBookmarks:
		m.statusLine = "Refreshing bookmarks…"
		return m, loadBookmarksCmd(m.client)
	case viewNotifications:
		m.statusLine = "Refreshing notifications…"
		return m, loadNotificationsCmd(m.client)
	case viewThread:
		m.loading = false
		m.statusLine = "Thread is a snapshot — reopen with /thread <n>"
		return m, nil
	default:
		return m.loadTimeline(m.currentKind)
	}
}

func (m Model) handleThread(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 1 {
		m.statusLine = "Usage: /thread <post number>"
		return m, nil
	}
	s := m.lookupStatus(args[0])
	if s == nil {
		m.statusLine = "No post numbered " + args[0]
		return m, nil
	}
	m.loading = true
	m.statusLine = "Loading thread…"
	return m, loadContextCmd(m.client, s)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (m Model) openActionMenu(num int, s *mastodon.Status) (tea.Model, tea.Cmd) {
	if s.Reblog != nil {
		s = s.Reblog
	}
	m.actionTarget = s
	m.actionNum = num
	m.actionOpen = true
	m.actionIndex = 0
	m.statusLine = fmt.Sprintf("Choose an action for post [%d]", num)
	return m, nil
}

type actionItem struct{ id, label string }

func (m Model) actionMenuItems() []actionItem {
	s := m.actionTarget
	var items []actionItem
	if s.Poll != nil && !s.Poll.Expired && !s.Poll.Voted {
		items = append(items, actionItem{"vote", "🗳  Vote in poll"})
	}
	boost := "↻ Boost"
	if s.Reblogged {
		boost = "↻ Unboost"
	}
	fav := "★ Favourite"
	if s.Favourited {
		fav = "★ Unfavourite"
	}
	bm := "🔖 Bookmark"
	if s.Bookmarked {
		bm = "🔖 Remove bookmark"
	}
	items = append(items,
		actionItem{"boost", boost},
		actionItem{"fav", fav},
		actionItem{"bookmark", bm},
		actionItem{"reply", "↩ Reply…"},
		actionItem{"thread", "🧵 Show thread"},
		actionItem{"open", "🌐 Open in browser"},
		actionItem{"follow", "＋ Follow @" + s.Account.Acct},
		actionItem{"mute", "🔇 Mute @" + s.Account.Acct},
		actionItem{"block", "⛔ Block @" + s.Account.Acct},
	)
	return items
}

func (m Model) runAction(idx int) (tea.Model, tea.Cmd) {
	items := m.actionMenuItems()
	if idx < 0 || idx >= len(items) {
		m.actionOpen = false
		return m, nil
	}
	s := m.actionTarget
	m.actionOpen = false
	switch items[idx].id {
	case "vote":
		return m.openPollVote(s)
	case "boost":
		m.loading = true
		return m, boostCmd(m.client, s)
	case "fav":
		m.loading = true
		return m, favCmd(m.client, s)
	case "bookmark":
		m.loading = true
		return m, bookmarkCmd(m.client, s)
	case "reply":
		m.input.SetValue(fmt.Sprintf("/reply %d ", m.actionNum))
		m.input.CursorEnd()
		m.statusLine = "Type your reply, then Enter"
		return m, nil
	case "thread":
		m.loading = true
		m.statusLine = "Loading thread…"
		return m, loadContextCmd(m.client, s)
	case "open":
		url := s.URL
		if url == "" {
			url = s.URI
		}
		_ = openInBrowser(url)
		m.statusLine = "Opened in browser"
		return m, nil
	case "follow":
		m.loading = true
		return m, followCmd(m.client, s.Account.ID, "@"+s.Account.Acct)
	case "mute":
		m.loading = true
		return m, moderateCmd(m.client, "mute", s.Account.ID, "@"+s.Account.Acct)
	case "block":
		m.loading = true
		return m, moderateCmd(m.client, "block", s.Account.ID, "@"+s.Account.Acct)
	}
	return m, nil
}

func (m Model) renderActionMenu() string {
	rowW := m.width - 2
	title := fmt.Sprintf(" Post [%d] · @%s   ↑↓ choose · Enter run · Esc cancel",
		m.actionNum, m.actionTarget.Account.Acct)
	lines := []string{styleMenuHint.Render(padTo(truncate(title, rowW), rowW))}
	for i, it := range m.actionMenuItems() {
		text := padTo(truncate("  "+it.label, rowW), rowW)
		if i == m.actionIndex {
			lines = append(lines, styleMenuSel.Render(text))
		} else {
			lines = append(lines, styleMenuItem.Render(text))
		}
	}
	return strings.Join(lines, "\n")
}

func renderAccounts(c *config.Config) string {
	var b strings.Builder
	b.WriteString("Accounts (switch with /switch <n>):\n")
	for i, a := range c.Accounts {
		mark := "  "
		if i == c.Active {
			mark = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%d. @%s · %s\n", mark, i+1, a.Username, a.Instance))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) handleSwitch(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 1 {
		m.addSystem(kindSystem, renderAccounts(m.cfg))
		m.refreshViewport()
		m.vp.GotoBottom()
		m.statusLine = "Usage: /switch <n>"
		return m, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 || n > len(m.cfg.Accounts) {
		m.statusLine = "No account numbered " + args[0]
		return m, nil
	}
	m.stopStream()
	m.cfg.Active = n - 1
	_ = m.cfg.Save()
	a := m.cfg.Current()
	m.client = mastodon.New(a.Instance, a.AccessToken)
	m.loading = true
	m.statusLine = "Switched to @" + a.Username + " — loading…"
	return m, loadTimelineCmd(m.client, mastodon.TimelineHome)
}

func (m Model) handlePoll(full, cmd string) (tea.Model, tea.Cmd) {
	body := strings.TrimSpace(strings.TrimPrefix(full, cmd))
	parts := strings.Split(body, "|")
	question := strings.TrimSpace(parts[0])
	var opts []string
	for _, p := range parts[1:] {
		if t := strings.TrimSpace(p); t != "" {
			opts = append(opts, t)
		}
	}
	if question == "" || len(opts) < 2 {
		m.statusLine = "Usage: /poll <question> | option 1 | option 2 …"
		return m, nil
	}
	m.loading = true
	m.statusLine = "Posting poll…"
	return m, postPollCmd(m.client, question, opts)
}

func (m Model) openPollVote(s *mastodon.Status) (tea.Model, tea.Cmd) {
	m.pollVoteTarget = s
	m.pollVoteOpen = true
	m.pollVoteIndex = 0
	m.statusLine = "Vote: pick an option"
	return m, nil
}

func (m Model) renderPollVote() string {
	rowW := m.width - 2
	p := m.pollVoteTarget.Poll
	lines := []string{styleMenuHint.Render(padTo(" Vote in poll   ↑↓ choose · Enter vote · Esc cancel", rowW))}
	for i, opt := range p.Options {
		text := padTo(truncate(fmt.Sprintf("  %s", opt.Title), rowW), rowW)
		if i == m.pollVoteIndex {
			lines = append(lines, styleMenuSel.Render(text))
		} else {
			lines = append(lines, styleMenuItem.Render(text))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) showThread(focus *mastodon.Status, cx *mastodon.Context) {
	m.stopStream()
	m.view = viewThread
	m.viewLabel = "thread"
	m.resetFeed()
	add := func(s mastodon.Status) {
		idx := m.nextIndex
		m.nextIndex++
		sc := s
		m.statusByIndex[idx] = &sc
		m.items = append(m.items, feedItem{kind: kindStatus, status: &sc, index: idx})
	}
	for _, a := range cx.Ancestors {
		add(a)
	}
	m.addSystem(kindSystem, "──── selected post ────")
	if focus != nil {
		add(*focus)
	}
	for _, d := range cx.Descendants {
		add(d)
	}
}

func (m Model) handleReply(full string, args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		m.statusLine = "Usage: /reply <n> <text>"
		return m, nil
	}
	s := m.lookupStatus(args[0])
	if s == nil {
		m.statusLine = "No post numbered " + args[0]
		return m, nil
	}

	idx := strings.Index(full, args[0])
	body := strings.TrimSpace(full[idx+len(args[0]):])
	if body == "" {
		m.statusLine = "Reply text is empty"
		return m, nil
	}
	return m.confirmPost(body, s.ID, s.Visibility, "Reply to @"+s.Account.Acct)
}

type statusActionFn func(*mastodon.Client, *mastodon.Status) tea.Cmd

func (m Model) statusAction(args []string, fn statusActionFn) (tea.Model, tea.Cmd) {
	if len(args) < 1 {
		m.statusLine = "Usage: <command> <post number>"
		return m, nil
	}
	s := m.lookupStatus(args[0])
	if s == nil {
		m.statusLine = "No post numbered " + args[0]
		return m, nil
	}
	m.loading = true
	return m, fn(m.client, s)
}

func (m Model) openListPicker() (tea.Model, tea.Cmd) {
	if len(m.lists) == 0 {
		m.loading = true
		m.statusLine = "Loading lists…"
		return m, loadListsCmd(m.client)
	}
	m.listPickerOpen = true
	m.listPickerIndex = 0
	m.statusLine = "Select a list to open"
	return m, nil
}

func (m Model) handleList(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 1 {

		return m.openListPicker()
	}
	if len(m.lists) == 0 {
		m.loading = true
		m.statusLine = "Loading lists…"
		return m, loadListsCmd(m.client)
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 || n > len(m.lists) {
		m.statusLine = "No list numbered " + args[0] + " (try /lists)"
		return m, nil
	}
	l := m.lists[n-1]
	m.currentListID, m.currentListName = l.ID, l.Title
	m.loading = true
	m.statusLine = "Loading list “" + l.Title + "”…"
	return m, loadListTimelineCmd(m.client, l.ID, l.Title)
}

func renderListMenu(lists []mastodon.List) string {
	if len(lists) == 0 {
		return "You have no lists. Create one in your Mastodon web/app, then /lists."
	}
	var b strings.Builder
	b.WriteString("Your lists (open with /list <n>):\n")
	for i, l := range lists {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, l.Title))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderListPicker() string {
	rowW := m.width - 2
	header := styleMenuHint.Render(padTo(" Select a list   ↑↓ choose · Enter open · Esc cancel", rowW))
	lines := []string{header}

	const maxRows = 10
	start := 0
	if m.listPickerIndex >= maxRows {
		start = m.listPickerIndex - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.lists) {
		end = len(m.lists)
	}
	for i := start; i < end; i++ {
		text := padTo(truncate(fmt.Sprintf("  %d. %s", i+1, m.lists[i].Title), rowW), rowW)
		if i == m.listPickerIndex {
			lines = append(lines, styleMenuSel.Render(text))
		} else {
			lines = append(lines, styleMenuItem.Render(text))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderConfirm() string {
	rowW := m.width - 2
	lines := []string{
		styleMenuHint.Render(padTo(" Post this toot?   Enter / y = post · Esc / n = cancel", rowW)),
	}
	meta := "  " + m.pending.summary
	if m.pending.visibility != "" {
		meta += " · " + m.pending.visibility
	}
	meta += fmt.Sprintf("  (%d chars)", len([]rune(m.pending.text)))
	lines = append(lines, styleMenuItem.Render(padTo(meta, rowW)))
	for _, ln := range strings.Split(wrapText(m.pending.text, rowW-4), "\n") {
		lines = append(lines, styleMenuSel.Render(padTo("  "+ln, rowW)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) handleFollow(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 1 {
		m.statusLine = "Usage: /follow <post number>"
		return m, nil
	}
	s := m.lookupStatus(args[0])
	if s == nil {
		m.statusLine = "No post numbered " + args[0]
		return m, nil
	}
	m.loading = true
	return m, followCmd(m.client, s.Account.ID, "@"+s.Account.Acct)
}

func (m Model) handleOpen(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 1 {
		m.statusLine = "Usage: /open <post number>"
		return m, nil
	}
	s := m.lookupStatus(args[0])
	if s == nil {
		m.statusLine = "No post numbered " + args[0]
		return m, nil
	}
	url := s.URL
	if url == "" {
		url = s.URI
	}
	_ = openInBrowser(url)
	m.statusLine = "Opened in browser"
	return m, nil
}

func (m Model) handleStream(args []string) (tea.Model, tea.Cmd) {
	on := !m.autoStream
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "start", "resume":
			on = true
		case "off", "stop", "pause":
			on = false
		}
	}
	m.autoStream = on
	if on {
		cmd := m.startStream()
		if m.streaming {
			m.statusLine = "Live updates ON ● — " + m.viewLabel
		} else {
			m.statusLine = "Live updates ON (this view has no live stream)"
		}
		return m, cmd
	}
	m.stopStream()
	m.statusLine = "Live updates paused"
	return m, nil
}

func (m *Model) startStream() tea.Cmd {
	m.stopStream()
	if !m.autoStream {
		return nil
	}
	spec, ok := m.currentStreamSpec()
	if !ok {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.streamGen++
	m.streamCh = m.client.Stream(ctx, spec)
	m.streaming = true
	m.animGen++
	return tea.Batch(waitForStreamCmd(m.streamCh, m.streamGen), animCmd(m.animGen))
}

func (m *Model) stopStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.streaming = false
	m.streamCh = nil
}

func (m Model) currentStreamSpec() (mastodon.StreamSpec, bool) {
	switch m.view {
	case viewList:
		if m.currentListID == "" {
			return mastodon.StreamSpec{}, false
		}
		return mastodon.StreamList(m.currentListID), true
	case viewTag:
		if m.currentTag == "" {
			return mastodon.StreamSpec{}, false
		}
		return mastodon.StreamHashtag(m.currentTag), true
	case viewNotifications:
		return mastodon.StreamHome(), true
	case viewAccount, viewBookmarks, viewThread:
		return mastodon.StreamSpec{}, false
	default:
		switch m.currentKind {
		case mastodon.TimelineLocal:
			return mastodon.StreamLocal(), true
		case mastodon.TimelinePublic:
			return mastodon.StreamPublic(), true
		default:
			return mastodon.StreamHome(), true
		}
	}
}

func (m Model) feedAcceptsStatusUpdates() bool {
	switch m.view {
	case viewTimeline, viewList, viewTag:
		return true
	default:
		return false
	}
}

func (m Model) lookupStatus(s string) *mastodon.Status {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	if st, ok := m.statusByIndex[n]; ok {
		return st
	}
	if nt, ok := m.notifByIndex[n]; ok && nt.Status != nil {
		return nt.Status
	}
	return nil
}

func (m *Model) handleStreamEvent(ev mastodon.StreamEvent) {
	switch ev.Type {
	case mastodon.EventUpdate:
		if !m.feedAcceptsStatusUpdates() {
			return
		}
		idx := m.nextIndex
		m.nextIndex++
		m.statusByIndex[idx] = ev.Status
		item := feedItem{kind: kindStatus, status: ev.Status, index: idx, isNew: true}
		m.items = append([]feedItem{item}, m.items...)
		m.statusLine = "● new post from @" + ev.Status.Account.Acct
	case mastodon.EventNotification:
		if m.view == viewNotifications {
			idx := m.nextIndex
			m.nextIndex++
			m.notifByIndex[idx] = ev.Notification
			item := feedItem{kind: kindNotif, notif: ev.Notification, index: idx, isNew: true}
			m.items = append([]feedItem{item}, m.items...)
		}
		m.statusLine = "🔔 new notification"
	case mastodon.EventError:
		m.statusLine = "Stream reconnecting… " + ev.Err.Error()
	}
}

func (m *Model) setTimeline(kind mastodon.TimelineKind, statuses []mastodon.Status) {
	m.currentKind = kind
	m.resetFeed()
	for i := range statuses {
		s := statuses[i]
		idx := m.nextIndex
		m.nextIndex++
		m.statusByIndex[idx] = &s
		m.items = append(m.items, feedItem{kind: kindStatus, status: &s, index: idx})
	}
}

func (m *Model) setNotifications(ns []mastodon.Notification) {
	m.resetFeed()
	for i := range ns {
		n := ns[i]
		idx := m.nextIndex
		m.nextIndex++
		m.notifByIndex[idx] = &n
		m.items = append(m.items, feedItem{kind: kindNotif, notif: &n, index: idx})
	}
}

func (m *Model) resetFeed() {
	m.items = nil
	m.statusByIndex = map[int]*mastodon.Status{}
	m.notifByIndex = map[int]*mastodon.Notification{}
	m.nextIndex = 1
}

func (m *Model) addSystem(kind itemKind, text string) {
	m.items = append(m.items, feedItem{kind: kind, text: text})
}

func (m *Model) applyUpdate(updated *mastodon.Status) {
	target := updated
	if updated.Reblog != nil {
		target = updated.Reblog
	}
	for i := range m.items {
		s := m.items[i].status
		if s == nil {
			continue
		}
		real := s
		if s.Reblog != nil {
			real = s.Reblog
		}
		if real.ID == target.ID {
			real.Favourited = target.Favourited
			real.Reblogged = target.Reblogged
			real.Bookmarked = target.Bookmarked
			real.FavouritesCount = target.FavouritesCount
			real.ReblogsCount = target.ReblogsCount
		}
	}
}

func (m *Model) applyPoll(statusID string, p *mastodon.Poll) {
	for i := range m.items {
		s := m.items[i].status
		if s == nil {
			continue
		}
		real := s
		if s.Reblog != nil {
			real = s.Reblog
		}
		if real.ID == statusID {
			real.Poll = p
		}
	}
}

func (m *Model) showSearchResults(res *mastodon.SearchResults) {
	m.resetFeed()
	if len(res.Hashtags) > 0 {
		tags := make([]string, len(res.Hashtags))
		for i, t := range res.Hashtags {
			tags[i] = "#" + t.Name
		}
		m.addSystem(kindSystem, "Hashtags: "+strings.Join(tags, "  ")+"   (open with /tag <name>)")
	}
	if len(res.Accounts) > 0 {
		accts := make([]string, len(res.Accounts))
		for i, a := range res.Accounts {
			accts[i] = "@" + a.Acct
		}
		m.addSystem(kindSystem, "Accounts: "+strings.Join(accts, "  ")+"   (open with /account <@user>)")
	}
	for i := range res.Statuses {
		s := res.Statuses[i]
		idx := m.nextIndex
		m.nextIndex++
		m.statusByIndex[idx] = &s
		m.items = append(m.items, feedItem{kind: kindStatus, status: &s, index: idx})
	}
	if len(m.items) == 0 {
		m.addSystem(kindSystem, "No results.")
	}
}

const (
	headerHeight  = 2
	statusHeight  = 1
	maxInputLines = 6
)

func (m Model) clampedInputLines() int {
	w := m.input.Width()
	if w <= 0 {
		return 1
	}
	lines := 0
	for _, line := range strings.Split(m.input.Value(), "\n") {
		rows := (lipgloss.Width(line) + w - 1) / w
		if rows < 1 {
			rows = 1
		}
		lines += rows
	}
	if lines < 1 {
		lines = 1
	}
	if lines > maxInputLines {
		lines = maxInputLines
	}
	return lines
}

func (m Model) vpHeightFor(inputLines int) int {
	h := m.height - headerHeight - (inputLines + 2) - statusHeight
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) layout() {
	m.input.SetWidth(m.width - 4)
	lines := m.clampedInputLines()
	m.input.SetHeight(lines)
	vpHeight := m.vpHeightFor(lines)
	if !m.ready {
		m.vp = viewport.New(m.width, vpHeight)
	} else {
		m.vp.Width = m.width
		m.vp.Height = vpHeight
	}
}

func (m *Model) syncInputHeight() {
	if !m.ready {
		return
	}
	lines := m.clampedInputLines()
	if lines == m.input.Height() {
		return
	}
	m.input.SetHeight(lines)
	m.vp.Height = m.vpHeightFor(lines)
}

func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	contentWidth := m.width - 2
	var blocks []string
	for _, it := range m.items {
		blocks = append(blocks, it.render(contentWidth))
	}
	sep := "\n" + styleSep.Render(strings.Repeat("─", contentWidth)) + "\n"
	m.vp.SetContent(strings.Join(blocks, sep))
}

func (m Model) View() string {
	if !m.ready {
		return "Starting md…"
	}

	header := m.renderHeader()
	status := m.renderStatusLine()
	box := stylePromptBox.Width(m.width - 2).Render(m.input.View())

	feed := m.vp.View()
	switch {
	case m.pollVoteOpen:
		feed = overlayBottom(feed, m.renderPollVote())
	case m.actionOpen:
		feed = overlayBottom(feed, m.renderActionMenu())
	case m.confirmOpen:
		feed = overlayBottom(feed, m.renderConfirm())
	case m.listPickerOpen:
		feed = overlayBottom(feed, m.renderListPicker())
	default:
		if menu := m.renderMenu(); menu != "" {
			feed = overlayBottom(feed, menu)
		}
	}

	return strings.Join([]string{header, feed, status, box}, "\n")
}

func overlayBottom(base, overlay string) string {
	bl := strings.Split(base, "\n")
	ol := strings.Split(overlay, "\n")
	if len(ol) >= len(bl) {
		return overlay
	}
	copy(bl[len(bl)-len(ol):], ol)
	return strings.Join(bl, "\n")
}

const mastoLogo = "╭∩∩"

var eqBars = []rune("▁▂▃▄▅▆▇▆▅▄▃▂")

func liveFlank(frame int, left, live bool) string {
	if !live {
		return lipgloss.NewStyle().Foreground(colDim).Render("▁▁▁")
	}
	st := lipgloss.NewStyle().Foreground(colGreen)
	var b strings.Builder
	for i := 0; i < 3; i++ {
		k := i
		if left {
			k = 2 - i
		}
		idx := ((frame + k*4) % len(eqBars))
		b.WriteString(st.Render(string(eqBars[idx])))
	}
	return b.String()
}

func (m Model) renderHeader() string {

	brand := lipgloss.NewStyle().Foreground(colMasto).Bold(true).Render(mastoLogo) +
		lipgloss.NewStyle().Foreground(colFg).Bold(true).Render(" Mastodon CLI") +
		styleStatusLine.Render(" v"+Version)
	inner := liveFlank(m.animFrame, true, m.streaming) + "   " + brand + "   " +
		liveFlank(m.animFrame, false, m.streaming)
	pad := (m.width - lipgloss.Width(inner)) / 2
	if pad < 0 {
		pad = 0
	}
	row0 := strings.Repeat(" ", pad) + inner

	ctx := styleStatusLine.Render(" ▸ "+m.viewLabel) +
		styleHandle.Render("  ·  @"+m.cfg.Current().Username) +
		styleStatusLine.Render(" · "+m.cfg.Current().Instance+" ")
	ruleW := m.width - lipgloss.Width(ctx)
	if ruleW < 0 {
		ruleW = 0
	}
	row1 := ctx + styleSep.Render(strings.Repeat("─", ruleW))

	return row0 + "\n" + row1
}

func (m Model) renderStatusLine() string {
	left := m.statusLine
	if m.loading {
		left = m.spinner.View() + left
	}
	hint := styleStatusLine.Render("/help · ⏎ send · ^J newline · PgUp/PgDn scroll · ^C quit")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	return styleStatusLine.Render(left) + strings.Repeat(" ", gap) + hint
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

const helpText = `Commands:
  <number> ⏎             open the action menu for that post
  /post <text>           post a new toot
  /home /public /local   switch timeline (live)
  /lists · /list <n>     your lists · open list #n (live)
  /tag <name>            hashtag feed (live)
  /account <@user>       an account's posts (auto-refreshing)
  /search <query>        search accounts, hashtags, posts
  /poll <q> | a | b      post a poll
  /bookmarks             your bookmarks
  /notifications, /n     show notifications (live)
  /refresh, /r           reload current view
  /reply <n> <text>      reply to post #n
  /boost <n>             boost / unboost post #n
  /fav <n>               favourite / unfavourite post #n
  /bookmark <n>          bookmark / unbookmark post #n
  /thread <n>            show the thread around post #n
  /follow <n>            follow the author of post #n
  /open <n>              open post #n in browser
  /stream [on|off]       pause/resume live updates (on by default)
  /accounts · /switch <n>   list / switch logged-in accounts
  /whoami                show the logged-in account
  /help                  this help    ·    /quit  exit`
