# md

`md` (mastodon) is a fully featured terminal client for
[Mastodon](https://joinmastodon.org), with a clean interactive interface and
**live timeline streaming**. Written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea).

```
┌ md  @you · mastodon.social  ▸ home  ● live ─────────────────────┐
│ [1] Ada Lovelace  @ada@mastodon.social  ·  3m   ● new           │
│ Just shipped the analytical engine v2 🎉                        │
│ 💬 4   ↻ 12   ★ 30                                              │
│                                                                 │
│ [2] Alan @alan  ↻ boosted by Grace                              │
│ Reminder: turing-complete is not the same as turing-tested.     │
│ 💬 1   ↻ 8   ★ 22                                               │
├─────────────────────────────────────────────────────────────────┤
│ ❯ Write a toot, or /help for commands…                          │
└─────────────────────────────────────────────────────────────────┘
 Posted ✓                    /help · ⏎ send · ^J newline · ^C quit
```

## Features

- **Interactive TUI** with a scrolling feed and a bordered compose box pinned to
  the bottom.
- **Live updates** — `/stream` connects to the Mastodon user stream (SSE) and
  new posts/notifications appear at the top in real time, with automatic
  reconnect + backoff.
- **Timelines**: home, public (federated), and local.
- **Notifications** view (favourites, boosts, follows, mentions).
- **Actions by number** — every visible post is numbered, so `/boost 2`,
  `/fav 1`, `/reply 3 nice!`, `/follow 2`, `/open 1` just work.
- **OAuth login** with a loopback redirect: enter your instance domain, the
  browser opens, and you're sent straight back to the terminal — no copy/paste.
- **Scriptable CLI**: `md post`, `md timeline` for use in pipes.

## Install

### Homebrew (macOS)

```sh
brew install prvrtl/tap/md
```

### From source

```sh
go build -o md .
# optionally: mv md /usr/local/bin/
```

Prebuilt binaries for macOS and Linux (amd64/arm64) are attached to each
[GitHub Release](https://github.com/prvrtl/mastodon-cli/releases).

## Usage

```sh
md                 # launch the interactive client (prompts login first time)
md login           # authenticate with an instance
md logout          # remove stored credentials
md post "hello"    # post from the shell
md timeline local  # print a timeline and exit
```

### Logging in

Run `md` (or `md login`). You'll be asked for your instance domain
(e.g. `mastodon.social`); the browser opens to the authorization page, and after
you approve, the local callback captures the token automatically. Credentials are
stored in `~/.config/mastocli/config.json` (mode `0600`).

### Interactive commands

| Command | Action |
| --- | --- |
| *type text* + ⏎ | post a new toot |
| `^J` | insert a newline in the compose box |
| `/` | open the command menu (↑↓ select · Tab complete · Esc close) |
| `/home` `/public` `/local` | switch timeline (live) |
| `/lists` · `/list <n>` | pick a list · open list #n (live) |
| `/tag <name>` | hashtag feed (live-streamed) |
| `/account <@user>` | an account's posts (auto-refreshing) |
| `/bookmarks` · `/bookmark <n>` | view bookmarks · bookmark post #n |
| `/thread <n>` | show the thread around post #n |
| `/notifications`, `/n` | show notifications (live) |
| `/refresh`, `/r` | reload the current view |
| `/reply <n> <text>` | reply to post #n |
| `/boost <n>` | boost / unboost post #n |
| `/fav <n>` | favourite / unfavourite post #n |
| `/follow <n>` | follow the author of post #n |
| `/open <n>` | open post #n in the browser |
| `/stream [on\|off]` | toggle the live stream |
| `/whoami` | show the logged-in account |
| `/help` · `/quit` | help · exit |

`PgUp`/`PgDn` and the mouse wheel scroll the feed.

## Testing

```sh
go test ./...            # run all tests
go test -race ./...      # with the race detector (covers streaming goroutines)
go test -cover ./...     # coverage per package
```

The suite (66 tests) covers: config persistence, the full REST client and SSE
streaming (via `httptest` servers — no network needed), and the UI model
(command routing, the slash menu, list picker, per-view stream selection, live
event filtering, account polling, and feed rendering).

## Project layout

```
main.go                 entry point + non-interactive subcommands
internal/config         token/instance persistence
internal/mastodon       REST client, types, SSE streaming
internal/auth           OAuth loopback login flow
internal/ui             Bubble Tea model, rendering, slash commands
```

## License

MIT
