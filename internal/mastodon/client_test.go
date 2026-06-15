package mastodon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testClient(srv *httptest.Server) *Client {
	c := New("example.social", "test-token")
	c.base = srv.URL
	return c
}

type recordingServer struct {
	srv        *httptest.Server
	lastMethod string
	lastPath   string
	lastQuery  url.Values
	lastAuth   string
	lastBody   string
	status     int
	body       string
}

func newRecordingServer(t *testing.T, body string) *recordingServer {
	t.Helper()
	rs := &recordingServer{status: 200, body: body}
	rs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.lastMethod = r.Method
		rs.lastPath = r.URL.Path
		rs.lastQuery = r.URL.Query()
		rs.lastAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		rs.lastBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rs.status)
		io.WriteString(w, rs.body)
	}))
	t.Cleanup(rs.srv.Close)
	return rs
}

func TestTimelineEndpoints(t *testing.T) {
	rs := newRecordingServer(t, `[{"id":"1","content":"<p>hi</p>"}]`)
	c := testClient(rs.srv)

	cases := []struct {
		kind     TimelineKind
		wantPath string
		wantQ    map[string]string
	}{
		{TimelineHome, "/api/v1/timelines/home", nil},
		{TimelinePublic, "/api/v1/timelines/public", nil},
		{TimelineLocal, "/api/v1/timelines/public", map[string]string{"local": "true"}},
	}
	for _, tc := range cases {
		st, err := c.Timeline(context.Background(), tc.kind, 20)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if len(st) != 1 || st[0].ID != "1" {
			t.Fatalf("%s: unexpected statuses %+v", tc.kind, st)
		}
		if rs.lastPath != tc.wantPath {
			t.Errorf("%s: path = %s, want %s", tc.kind, rs.lastPath, tc.wantPath)
		}
		if rs.lastQuery.Get("limit") != "20" {
			t.Errorf("%s: limit not forwarded", tc.kind)
		}
		for k, v := range tc.wantQ {
			if rs.lastQuery.Get(k) != v {
				t.Errorf("%s: query %s = %q, want %q", tc.kind, k, rs.lastQuery.Get(k), v)
			}
		}
	}
	if rs.lastAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q", rs.lastAuth)
	}
}

func TestReadEndpointPaths(t *testing.T) {
	rs := newRecordingServer(t, `[]`)
	c := testClient(rs.srv)
	ctx := context.Background()

	checks := []struct {
		name string
		call func() error
		path string
	}{
		{"tag", func() error { _, e := c.TagTimeline(ctx, "#golang", 10); return e }, "/api/v1/timelines/tag/golang"},
		{"list", func() error { _, e := c.ListTimeline(ctx, "42", 10); return e }, "/api/v1/timelines/list/42"},
		{"acctStatuses", func() error { _, e := c.AccountStatuses(ctx, "7", "", 10); return e }, "/api/v1/accounts/7/statuses"},
		{"bookmarks", func() error { _, e := c.Bookmarks(ctx, 10); return e }, "/api/v1/bookmarks"},
		{"notifications", func() error { _, e := c.Notifications(ctx, 10); return e }, "/api/v1/notifications"},
	}
	for _, ck := range checks {
		if err := ck.call(); err != nil {
			t.Fatalf("%s: %v", ck.name, err)
		}
		if rs.lastPath != ck.path {
			t.Errorf("%s: path = %s, want %s", ck.name, rs.lastPath, ck.path)
		}
		if rs.lastMethod != http.MethodGet {
			t.Errorf("%s: method = %s, want GET", ck.name, rs.lastMethod)
		}
	}
}

func TestTagTimelineStripsHash(t *testing.T) {
	rs := newRecordingServer(t, `[]`)
	c := testClient(rs.srv)
	if _, err := c.TagTimeline(context.Background(), "#news", 5); err != nil {
		t.Fatal(err)
	}
	if rs.lastPath != "/api/v1/timelines/tag/news" {
		t.Errorf("path = %s; hash should be stripped", rs.lastPath)
	}
}

func TestAccountStatusesSinceID(t *testing.T) {
	rs := newRecordingServer(t, `[]`)
	c := testClient(rs.srv)
	if _, err := c.AccountStatuses(context.Background(), "7", "555", 10); err != nil {
		t.Fatal(err)
	}
	if rs.lastQuery.Get("since_id") != "555" {
		t.Errorf("since_id = %q, want 555", rs.lastQuery.Get("since_id"))
	}
	if rs.lastQuery.Get("exclude_replies") != "true" {
		t.Error("exclude_replies should be true")
	}
}

func TestLookupAccount(t *testing.T) {
	rs := newRecordingServer(t, `{"id":"1","acct":"gargron","display_name":"Eugen"}`)
	c := testClient(rs.srv)
	a, err := c.LookupAccount(context.Background(), "@gargron")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "1" || a.Acct != "gargron" {
		t.Fatalf("unexpected account %+v", a)
	}
	if rs.lastQuery.Get("acct") != "gargron" {
		t.Errorf("acct query = %q (leading @ should be stripped)", rs.lastQuery.Get("acct"))
	}
	if a.Name() != "Eugen" {
		t.Errorf("Name() = %q, want Eugen", a.Name())
	}
}

func TestStatusContext(t *testing.T) {
	rs := newRecordingServer(t, `{"ancestors":[{"id":"1"}],"descendants":[{"id":"3"},{"id":"4"}]}`)
	c := testClient(rs.srv)
	cx, err := c.StatusContext(context.Background(), "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(cx.Ancestors) != 1 || len(cx.Descendants) != 2 {
		t.Fatalf("context = %+v", cx)
	}
	if rs.lastPath != "/api/v1/statuses/2/context" {
		t.Errorf("path = %s", rs.lastPath)
	}
}

func TestPostStatusFormEncoding(t *testing.T) {
	rs := newRecordingServer(t, `{"id":"99","content":"<p>hello</p>"}`)
	c := testClient(rs.srv)
	_, err := c.PostStatus(context.Background(), "hello", "12", "unlisted")
	if err != nil {
		t.Fatal(err)
	}
	if rs.lastMethod != http.MethodPost || rs.lastPath != "/api/v1/statuses" {
		t.Fatalf("post went to %s %s", rs.lastMethod, rs.lastPath)
	}
	form, _ := url.ParseQuery(rs.lastBody)
	if form.Get("status") != "hello" {
		t.Errorf("status = %q", form.Get("status"))
	}
	if form.Get("in_reply_to_id") != "12" {
		t.Errorf("in_reply_to_id = %q", form.Get("in_reply_to_id"))
	}
	if form.Get("visibility") != "unlisted" {
		t.Errorf("visibility = %q", form.Get("visibility"))
	}
}

func TestStatusActions(t *testing.T) {
	rs := newRecordingServer(t, `{"id":"5","favourited":true,"reblogged":true,"bookmarked":true}`)
	c := testClient(rs.srv)
	ctx := context.Background()
	actions := []struct {
		call func() (*Status, error)
		path string
	}{
		{func() (*Status, error) { return c.Favourite(ctx, "5") }, "/api/v1/statuses/5/favourite"},
		{func() (*Status, error) { return c.Unfavourite(ctx, "5") }, "/api/v1/statuses/5/unfavourite"},
		{func() (*Status, error) { return c.Boost(ctx, "5") }, "/api/v1/statuses/5/reblog"},
		{func() (*Status, error) { return c.Unboost(ctx, "5") }, "/api/v1/statuses/5/unreblog"},
		{func() (*Status, error) { return c.Bookmark(ctx, "5") }, "/api/v1/statuses/5/bookmark"},
		{func() (*Status, error) { return c.Unbookmark(ctx, "5") }, "/api/v1/statuses/5/unbookmark"},
	}
	for _, a := range actions {
		if _, err := a.call(); err != nil {
			t.Fatalf("%s: %v", a.path, err)
		}
		if rs.lastPath != a.path || rs.lastMethod != http.MethodPost {
			t.Errorf("got %s %s, want POST %s", rs.lastMethod, rs.lastPath, a.path)
		}
	}
}

func TestAccountActions(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := testClient(rs.srv)
	ctx := context.Background()
	checks := []struct {
		call func() error
		path string
	}{
		{func() error { return c.Follow(ctx, "9") }, "/api/v1/accounts/9/follow"},
		{func() error { return c.Unfollow(ctx, "9") }, "/api/v1/accounts/9/unfollow"},
		{func() error { return c.Mute(ctx, "9") }, "/api/v1/accounts/9/mute"},
		{func() error { return c.Block(ctx, "9") }, "/api/v1/accounts/9/block"},
	}
	for _, ck := range checks {
		if err := ck.call(); err != nil {
			t.Fatalf("%s: %v", ck.path, err)
		}
		if rs.lastPath != ck.path || rs.lastMethod != http.MethodPost {
			t.Errorf("got %s %s, want POST %s", rs.lastMethod, rs.lastPath, ck.path)
		}
	}
}

func TestVotePoll(t *testing.T) {
	rs := newRecordingServer(t, `{"id":"p","voted":true,"options":[{"title":"a","votes_count":1}]}`)
	c := testClient(rs.srv)
	p, err := c.VotePoll(context.Background(), "p1", []int{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Voted {
		t.Error("poll should be voted")
	}
	if rs.lastPath != "/api/v1/polls/p1/votes" || rs.lastMethod != http.MethodPost {
		t.Errorf("got %s %s", rs.lastMethod, rs.lastPath)
	}
	form, _ := url.ParseQuery(rs.lastBody)
	if got := form["choices[]"]; len(got) != 2 || got[0] != "0" || got[1] != "2" {
		t.Errorf("choices = %v", got)
	}
}

func TestSearch(t *testing.T) {
	rs := newRecordingServer(t, `{"accounts":[{"id":"1","acct":"a"}],"statuses":[{"id":"2"}],"hashtags":[{"name":"go"}]}`)
	c := testClient(rs.srv)
	res, err := c.Search(context.Background(), "golang", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accounts) != 1 || len(res.Statuses) != 1 || len(res.Hashtags) != 1 {
		t.Fatalf("unexpected results: %+v", res)
	}
	if rs.lastPath != "/api/v2/search" || rs.lastQuery.Get("q") != "golang" {
		t.Errorf("search request wrong: %s ?%s", rs.lastPath, rs.lastQuery.Encode())
	}
}

func TestPostPoll(t *testing.T) {
	rs := newRecordingServer(t, `{"id":"9","poll":{"id":"p"}}`)
	c := testClient(rs.srv)
	if _, err := c.PostPoll(context.Background(), "Q?", []string{"yes", "no"}, 3600, false, "public"); err != nil {
		t.Fatal(err)
	}
	form, _ := url.ParseQuery(rs.lastBody)
	if opts := form["poll[options][]"]; len(opts) != 2 || opts[0] != "yes" {
		t.Errorf("poll options = %v", opts)
	}
	if form.Get("poll[expires_in]") != "3600" {
		t.Errorf("expires_in = %q", form.Get("poll[expires_in]"))
	}
}

func TestAPIErrorSurfacesMessage(t *testing.T) {
	rs := newRecordingServer(t, `{"error":"Record not found"}`)
	rs.status = 404
	c := testClient(rs.srv)
	_, err := c.Timeline(context.Background(), TimelineHome, 20)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Record not found") {
		t.Errorf("error = %q, want it to contain the API message", err)
	}
}

func TestOAuthFlow(t *testing.T) {
	var regBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps":
			b, _ := io.ReadAll(r.Body)
			regBody = string(b)
			io.WriteString(w, `{"client_id":"CID","client_secret":"CSEC"}`)
		case "/oauth/token":
			io.WriteString(w, `{"access_token":"ATOK","token_type":"Bearer"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New("example.social", "")
	c.base = srv.URL

	cid, csec, err := c.RegisterApp(context.Background(), "mastocli", "http://127.0.0.1/cb")
	if err != nil {
		t.Fatal(err)
	}
	if cid != "CID" || csec != "CSEC" {
		t.Fatalf("register returned %q %q", cid, csec)
	}
	form, _ := url.ParseQuery(regBody)
	if form.Get("redirect_uris") != "http://127.0.0.1/cb" {
		t.Errorf("redirect_uris = %q", form.Get("redirect_uris"))
	}
	if form.Get("scopes") != Scopes {
		t.Errorf("scopes = %q, want %q", form.Get("scopes"), Scopes)
	}

	tok, err := c.ExchangeToken(context.Background(), cid, csec, "code123", "http://127.0.0.1/cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "ATOK" {
		t.Errorf("token = %q", tok)
	}
}

func TestAuthorizeURL(t *testing.T) {
	c := New("mastodon.example", "")
	got := c.AuthorizeURL("CID", "http://127.0.0.1/cb")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "mastodon.example" || u.Path != "/oauth/authorize" {
		t.Errorf("unexpected authorize url: %s", got)
	}
	q := u.Query()
	if q.Get("client_id") != "CID" || q.Get("response_type") != "code" || q.Get("scope") != Scopes {
		t.Errorf("authorize query missing fields: %v", q)
	}
}

func TestNewNormalizesInstance(t *testing.T) {
	for _, in := range []string{"https://mastodon.example/", "http://mastodon.example", "mastodon.example"} {
		c := New(in, "")
		if c.Instance() != "mastodon.example" {
			t.Errorf("New(%q).Instance() = %q", in, c.Instance())
		}
		if c.baseURL() != "https://mastodon.example" {
			t.Errorf("New(%q).baseURL() = %q", in, c.baseURL())
		}
	}
}

func TestFixturesParse(t *testing.T) {
	var s Status
	if err := json.Unmarshal([]byte(`{"id":"1","poll":{"id":"p","options":[{"title":"a"}]}}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Poll == nil || len(s.Poll.Options) != 1 {
		t.Fatalf("poll not parsed: %+v", s.Poll)
	}
}
