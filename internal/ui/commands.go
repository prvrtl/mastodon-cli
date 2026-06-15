package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

type statusesLoadedMsg struct {
	kind     mastodon.TimelineKind
	statuses []mastodon.Status
	err      error
}

type notificationsLoadedMsg struct {
	notifs []mastodon.Notification
	err    error
}

type listsLoadedMsg struct {
	lists []mastodon.List
	err   error
}

type listTimelineLoadedMsg struct {
	title    string
	statuses []mastodon.Status
	err      error
}

type tagLoadedMsg struct {
	tag      string
	statuses []mastodon.Status
	err      error
}

type accountLoadedMsg struct {
	acct      string
	accountID string
	statuses  []mastodon.Status
	err       error
}

type bookmarksLoadedMsg struct {
	statuses []mastodon.Status
	err      error
}

type contextLoadedMsg struct {
	focus *mastodon.Status
	ctx   *mastodon.Context
	err   error
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type animTickMsg struct{ gen int }

func animCmd(gen int) tea.Cmd {
	return tea.Tick(160*time.Millisecond, func(time.Time) tea.Msg {
		return animTickMsg{gen: gen}
	})
}

type accountPollFireMsg struct{ gen int }
type accountPollMsg struct {
	gen      int
	statuses []mastodon.Status
}

type postedMsg struct {
	status *mastodon.Status
	err    error
}

type actionMsg struct {
	ok      string
	updated *mastodon.Status
	err     error
}

type streamMsg struct {
	ev  mastodon.StreamEvent
	ch  <-chan mastodon.StreamEvent
	gen int
}

func loadTimelineCmd(c *mastodon.Client, kind mastodon.TimelineKind) tea.Cmd {
	return func() tea.Msg {
		st, err := c.Timeline(context.Background(), kind, 30)
		return statusesLoadedMsg{kind: kind, statuses: st, err: err}
	}
}

func loadListsCmd(c *mastodon.Client) tea.Cmd {
	return func() tea.Msg {
		ls, err := c.Lists(context.Background())
		return listsLoadedMsg{lists: ls, err: err}
	}
}

func loadListTimelineCmd(c *mastodon.Client, id, title string) tea.Cmd {
	return func() tea.Msg {
		st, err := c.ListTimeline(context.Background(), id, 30)
		return listTimelineLoadedMsg{title: title, statuses: st, err: err}
	}
}

func loadTagCmd(c *mastodon.Client, tag string) tea.Cmd {
	return func() tea.Msg {
		st, err := c.TagTimeline(context.Background(), tag, 30)
		return tagLoadedMsg{tag: tag, statuses: st, err: err}
	}
}

func loadAccountCmd(c *mastodon.Client, acct string) tea.Cmd {
	return func() tea.Msg {
		a, err := c.LookupAccount(context.Background(), acct)
		if err != nil {
			return accountLoadedMsg{acct: acct, err: err}
		}
		st, err := c.AccountStatuses(context.Background(), a.ID, "", 30)
		return accountLoadedMsg{acct: a.Acct, accountID: a.ID, statuses: st, err: err}
	}
}

func scheduleAccountPoll(gen int) tea.Cmd {
	return tea.Tick(20*time.Second, func(time.Time) tea.Msg {
		return accountPollFireMsg{gen: gen}
	})
}

func pollAccountCmd(c *mastodon.Client, accountID, sinceID string, gen int) tea.Cmd {
	return func() tea.Msg {
		st, err := c.AccountStatuses(context.Background(), accountID, sinceID, 30)
		if err != nil {
			return accountPollMsg{gen: gen}
		}
		return accountPollMsg{gen: gen, statuses: st}
	}
}

func loadBookmarksCmd(c *mastodon.Client) tea.Cmd {
	return func() tea.Msg {
		st, err := c.Bookmarks(context.Background(), 30)
		return bookmarksLoadedMsg{statuses: st, err: err}
	}
}

func loadContextCmd(c *mastodon.Client, focus *mastodon.Status) tea.Cmd {
	return func() tea.Msg {
		cx, err := c.StatusContext(context.Background(), focus.ID)
		return contextLoadedMsg{focus: focus, ctx: cx, err: err}
	}
}

func bookmarkCmd(c *mastodon.Client, s *mastodon.Status) tea.Cmd {
	return func() tea.Msg {
		var (
			res *mastodon.Status
			err error
			ok  string
		)
		if s.Bookmarked {
			res, err = c.Unbookmark(context.Background(), s.ID)
			ok = "Removed bookmark"
		} else {
			res, err = c.Bookmark(context.Background(), s.ID)
			ok = "Bookmarked 🔖"
		}
		return actionMsg{ok: ok, updated: res, err: err}
	}
}

func loadNotificationsCmd(c *mastodon.Client) tea.Cmd {
	return func() tea.Msg {
		ns, err := c.Notifications(context.Background(), 30)
		return notificationsLoadedMsg{notifs: ns, err: err}
	}
}

func postCmd(c *mastodon.Client, text, replyTo, visibility string) tea.Cmd {
	return func() tea.Msg {
		s, err := c.PostStatus(context.Background(), text, replyTo, visibility)
		return postedMsg{status: s, err: err}
	}
}

func favCmd(c *mastodon.Client, s *mastodon.Status) tea.Cmd {
	return func() tea.Msg {
		var (
			res *mastodon.Status
			err error
			ok  string
		)
		if s.Favourited {
			res, err = c.Unfavourite(context.Background(), s.ID)
			ok = "Removed favourite"
		} else {
			res, err = c.Favourite(context.Background(), s.ID)
			ok = "Favourited ★"
		}
		return actionMsg{ok: ok, updated: res, err: err}
	}
}

func boostCmd(c *mastodon.Client, s *mastodon.Status) tea.Cmd {
	return func() tea.Msg {
		var (
			res *mastodon.Status
			err error
			ok  string
		)
		if s.Reblogged {
			res, err = c.Unboost(context.Background(), s.ID)
			ok = "Removed boost"
		} else {
			res, err = c.Boost(context.Background(), s.ID)
			ok = "Boosted ↻"
		}
		return actionMsg{ok: ok, updated: res, err: err}
	}
}

func followCmd(c *mastodon.Client, accountID, name string) tea.Cmd {
	return func() tea.Msg {
		err := c.Follow(context.Background(), accountID)
		return actionMsg{ok: "Followed " + name, err: err}
	}
}

func moderateCmd(c *mastodon.Client, action, accountID, name string) tea.Cmd {
	return func() tea.Msg {
		var err error
		var ok string
		switch action {
		case "mute":
			err, ok = c.Mute(context.Background(), accountID), "Muted "+name
		case "block":
			err, ok = c.Block(context.Background(), accountID), "Blocked "+name
		case "unfollow":
			err, ok = c.Unfollow(context.Background(), accountID), "Unfollowed "+name
		}
		return actionMsg{ok: ok, err: err}
	}
}

type pollVotedMsg struct {
	statusID string
	poll     *mastodon.Poll
	err      error
}

func votePollCmd(c *mastodon.Client, statusID, pollID string, choices []int) tea.Cmd {
	return func() tea.Msg {
		p, err := c.VotePoll(context.Background(), pollID, choices)
		return pollVotedMsg{statusID: statusID, poll: p, err: err}
	}
}

func postPollCmd(c *mastodon.Client, text string, options []string) tea.Cmd {
	return func() tea.Msg {
		s, err := c.PostPoll(context.Background(), text, options, 86400, false, "")
		return postedMsg{status: s, err: err}
	}
}

type searchLoadedMsg struct {
	query string
	res   *mastodon.SearchResults
	err   error
}

func searchCmd(c *mastodon.Client, q string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.Search(context.Background(), q, 20)
		return searchLoadedMsg{query: q, res: res, err: err}
	}
}

func waitForStreamCmd(ch <-chan mastodon.StreamEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return streamMsg{ev: ev, ch: ch, gen: gen}
	}
}
