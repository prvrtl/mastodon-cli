package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

const successHTML = `<!doctype html><html><head><meta charset="utf-8">
<title>masto-cli</title><style>
body{font-family:system-ui,sans-serif;background:#1a1b26;color:#c0caf5;
display:flex;height:100vh;align-items:center;justify-content:center;margin:0}
.card{text-align:center}.card h1{color:#9ece6a}</style></head>
<body><div class="card"><h1>&#10003; Authorized</h1>
<p>You're logged in to masto-cli. You can close this tab and return to your terminal.</p>
</div></body></html>`

func Login(ctx context.Context, in *bufio.Reader) (*config.Config, error) {
	fmt.Print("Mastodon instance domain (e.g. mastodon.social): ")
	line, err := in.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading domain: %w", err)
	}
	domain := strings.TrimSpace(line)
	if domain == "" {
		return nil, fmt.Errorf("no domain entered")
	}

	client := mastodon.New(domain, "")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	defer ln.Close()
	redirectURI := fmt.Sprintf("http://%s/callback", ln.Addr().String())

	fmt.Println("Registering application...")
	clientID, clientSecret, err := client.RegisterApp(ctx, "masto-cli", redirectURI)
	if err != nil {
		return nil, fmt.Errorf("registering app: %w", err)
	}

	state := randomState()
	authURL := client.AuthorizeURL(clientID, redirectURI) + "&state=" + state

	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)
	srv := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "Authorization denied: "+e, http.StatusBadRequest)
			resCh <- result{err: fmt.Errorf("authorization denied: %s", e)}
			return
		}
		if q.Get("state") != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			resCh <- result{err: fmt.Errorf("state mismatch (possible CSRF)")}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "No code in callback", http.StatusBadRequest)
			resCh <- result{err: fmt.Errorf("no authorization code returned")}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, successHTML)
		resCh <- result{code: code}
	})
	srv.Handler = mux
	go srv.Serve(ln)
	defer srv.Close()

	fmt.Println("\nOpening your browser to authorize masto-cli...")
	fmt.Printf("If it doesn't open, visit this URL manually:\n\n  %s\n\n", authURL)
	_ = openBrowser(authURL)
	fmt.Println("Waiting for authorization...")

	var code string
	select {
	case res := <-resCh:
		if res.err != nil {
			return nil, res.err
		}
		code = res.code
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for authorization")
	}

	fmt.Println("Exchanging authorization code for token...")
	token, err := client.ExchangeToken(ctx, clientID, clientSecret, code, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("exchanging token: %w", err)
	}

	authed := mastodon.New(domain, token)
	acct, err := authed.VerifyCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("verifying credentials: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	cfg.Add(config.Account{
		Instance:     authed.Instance(),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  token,
		Username:     acct.Acct,
	})
	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("\n✓ Logged in as @%s on %s\n", acct.Acct, authed.Instance())
	return cfg, nil
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
