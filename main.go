package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/prvrtl/mastocli/internal/auth"
	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
	"github.com/prvrtl/mastocli/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "login":
		return cmdLogin()
	case "logout":
		return cmdLogout()
	case "post", "toot":
		return cmdPost(args[1:])
	case "timeline":
		return cmdTimeline(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version", "--version":
		fmt.Println("md", version)
		return nil
	case "":
		return cmdTUI()
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

var version = "dev"

func clearTerminal() {
	fmt.Print("\x1b[2J\x1b[3J\x1b[H")
}

func cmdTUI() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.LoggedIn() {
		fmt.Println("Welcome to md! Let's log in first.")
		cfg, err = auth.Login(context.Background(), bufio.NewReader(os.Stdin))
		if err != nil {
			return err
		}
		clearTerminal()
	}

	acct := cfg.Current()
	client := mastodon.New(acct.Instance, acct.AccessToken)
	ui.Version = version
	model := ui.New(client, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func cmdLogin() error {
	_, err := auth.Login(context.Background(), bufio.NewReader(os.Stdin))
	return err
}

func cmdLogout() error {
	p, err := config.Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("Logged out. Credentials removed.")
	return nil
}

func requireClient() (*mastodon.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	if !cfg.LoggedIn() {
		return nil, nil, fmt.Errorf("not logged in; run: md login")
	}
	acct := cfg.Current()
	return mastodon.New(acct.Instance, acct.AccessToken), cfg, nil
}

func cmdPost(args []string) error {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text == "" {
		return fmt.Errorf("usage: md post <text>")
	}
	client, _, err := requireClient()
	if err != nil {
		return err
	}
	s, err := client.PostStatus(context.Background(), text, "", "")
	if err != nil {
		return err
	}
	fmt.Println("Posted:", s.URL)
	return nil
}

func cmdTimeline(args []string) error {
	kind := mastodon.TimelineHome
	if len(args) > 0 {
		switch args[0] {
		case "public", "fed":
			kind = mastodon.TimelinePublic
		case "local":
			kind = mastodon.TimelineLocal
		}
	}
	client, _, err := requireClient()
	if err != nil {
		return err
	}
	statuses, err := client.Timeline(context.Background(), kind, 20)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		st := s
		if st.Reblog != nil {
			fmt.Printf("@%s boosted @%s\n", st.Account.Acct, st.Reblog.Account.Acct)
			st = *st.Reblog
		}
		fmt.Printf("@%s · %s\n", st.Account.Acct, st.CreatedAt.Local().Format("Jan 2 15:04"))
		fmt.Println(ui.PlainText(st.Content))
		fmt.Printf("  💬 %d  ↻ %d  ★ %d\n\n", st.RepliesCount, st.ReblogsCount, st.FavouritesCount)
	}
	return nil
}

func printUsage() {
	fmt.Print(`md — a terminal client for Mastodon

Usage:
  md                 launch the interactive client (logs in if needed)
  md login           authenticate with a Mastodon instance
  md logout          remove stored credentials
  md post <text>     post a toot from the command line
  md timeline [home|public|local]
                     print a timeline and exit
  md version         print version
  md help            show this help

In the interactive client, type /help for the full command list.
`)
}
