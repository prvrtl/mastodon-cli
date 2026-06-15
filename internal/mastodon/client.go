package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Scopes = "read write follow push"

const RedirectURIOOB = "urn:ietf:wg:oauth:2.0:oob"

type Client struct {
	instance   string
	base       string
	token      string
	http       *http.Client
	streamBase string
}

func New(instance, token string) *Client {
	instance = strings.TrimSpace(instance)
	instance = strings.TrimPrefix(instance, "https://")
	instance = strings.TrimPrefix(instance, "http://")
	instance = strings.TrimSuffix(instance, "/")
	return &Client{
		instance: instance,
		base:     "https://" + instance,
		token:    token,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Instance() string { return c.instance }

func (c *Client) baseURL() string { return c.base }

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, body)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s %s: %w", method, path, err)
	}
	return nil
}

func apiError(status int, body []byte) error {
	var e struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		if e.ErrorDescription != "" {
			return fmt.Errorf("mastodon: %s (%s)", e.Error, e.ErrorDescription)
		}
		return fmt.Errorf("mastodon: %s", e.Error)
	}
	return fmt.Errorf("mastodon: http %d", status)
}

func (c *Client) RegisterApp(ctx context.Context, name, redirectURI string) (clientID, clientSecret string, err error) {
	form := url.Values{
		"client_name":   {name},
		"redirect_uris": {redirectURI},
		"scopes":        {Scopes},
		"website":       {"https://github.com/prvrtl/mastodon-cli"},
	}
	var reg appRegistration
	err = c.do(ctx, http.MethodPost, "/api/v1/apps", strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", &reg)
	if err != nil {
		return "", "", err
	}
	return reg.ClientID, reg.ClientSecret, nil
}

func (c *Client) AuthorizeURL(clientID, redirectURI string) string {
	q := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {Scopes},
	}
	return c.baseURL() + "/oauth/authorize?" + q.Encode()
}

func (c *Client) ExchangeToken(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"code":          {code},
		"scope":         {Scopes},
	}
	var tok tokenResponse
	err := c.do(ctx, http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", &tok)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (c *Client) VerifyCredentials(ctx context.Context) (*Account, error) {
	var a Account
	if err := c.do(ctx, http.MethodGet, "/api/v1/accounts/verify_credentials", nil, "", &a); err != nil {
		return nil, err
	}
	return &a, nil
}

type TimelineKind string

const (
	TimelineHome   TimelineKind = "home"
	TimelinePublic TimelineKind = "public"
	TimelineLocal  TimelineKind = "local"
)

func (c *Client) Timeline(ctx context.Context, kind TimelineKind, limit int) ([]Status, error) {
	var path string
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	switch kind {
	case TimelineHome:
		path = "/api/v1/timelines/home"
	case TimelineLocal:
		path = "/api/v1/timelines/public"
		q.Set("local", "true")
	default:
		path = "/api/v1/timelines/public"
	}
	var statuses []Status
	if err := c.do(ctx, http.MethodGet, path+"?"+q.Encode(), nil, "", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) Lists(ctx context.Context) ([]List, error) {
	var ls []List
	if err := c.do(ctx, http.MethodGet, "/api/v1/lists", nil, "", &ls); err != nil {
		return nil, err
	}
	return ls, nil
}

func (c *Client) ListTimeline(ctx context.Context, listID string, limit int) ([]Status, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	path := "/api/v1/timelines/list/" + url.PathEscape(listID) + "?" + q.Encode()
	var statuses []Status
	if err := c.do(ctx, http.MethodGet, path, nil, "", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) TagTimeline(ctx context.Context, tag string, limit int) ([]Status, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	path := "/api/v1/timelines/tag/" + url.PathEscape(strings.TrimPrefix(tag, "#")) + "?" + q.Encode()
	var statuses []Status
	if err := c.do(ctx, http.MethodGet, path, nil, "", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) LookupAccount(ctx context.Context, acct string) (*Account, error) {
	q := url.Values{"acct": {strings.TrimPrefix(acct, "@")}}
	var a Account
	if err := c.do(ctx, http.MethodGet, "/api/v1/accounts/lookup?"+q.Encode(), nil, "", &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (c *Client) AccountStatuses(ctx context.Context, accountID, sinceID string, limit int) ([]Status, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}, "exclude_replies": {"true"}}
	if sinceID != "" {
		q.Set("since_id", sinceID)
	}
	path := fmt.Sprintf("/api/v1/accounts/%s/statuses?%s", url.PathEscape(accountID), q.Encode())
	var statuses []Status
	if err := c.do(ctx, http.MethodGet, path, nil, "", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) Bookmarks(ctx context.Context, limit int) ([]Status, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	var statuses []Status
	if err := c.do(ctx, http.MethodGet, "/api/v1/bookmarks?"+q.Encode(), nil, "", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) StatusContext(ctx context.Context, id string) (*Context, error) {
	var cx Context
	path := fmt.Sprintf("/api/v1/statuses/%s/context", url.PathEscape(id))
	if err := c.do(ctx, http.MethodGet, path, nil, "", &cx); err != nil {
		return nil, err
	}
	return &cx, nil
}

func (c *Client) Notifications(ctx context.Context, limit int) ([]Notification, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	var ns []Notification
	if err := c.do(ctx, http.MethodGet, "/api/v1/notifications?"+q.Encode(), nil, "", &ns); err != nil {
		return nil, err
	}
	return ns, nil
}

func (c *Client) PostStatus(ctx context.Context, text, inReplyToID, visibility string) (*Status, error) {
	form := url.Values{"status": {text}}
	if inReplyToID != "" {
		form.Set("in_reply_to_id", inReplyToID)
	}
	if visibility != "" {
		form.Set("visibility", visibility)
	}
	var s Status
	err := c.do(ctx, http.MethodPost, "/api/v1/statuses", strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) PostPoll(ctx context.Context, text string, options []string, expiresIn int, multiple bool, visibility string) (*Status, error) {
	form := url.Values{"status": {text}}
	for _, o := range options {
		form.Add("poll[options][]", o)
	}
	form.Set("poll[expires_in]", fmt.Sprint(expiresIn))
	if multiple {
		form.Set("poll[multiple]", "true")
	}
	if visibility != "" {
		form.Set("visibility", visibility)
	}
	var s Status
	err := c.do(ctx, http.MethodPost, "/api/v1/statuses", strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) Favourite(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "favourite")
}

func (c *Client) Unfavourite(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "unfavourite")
}

func (c *Client) Boost(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "reblog")
}

func (c *Client) Bookmark(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "bookmark")
}

func (c *Client) Unbookmark(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "unbookmark")
}

func (c *Client) Unboost(ctx context.Context, id string) (*Status, error) {
	return c.statusAction(ctx, id, "unreblog")
}

func (c *Client) statusAction(ctx context.Context, id, action string) (*Status, error) {
	var s Status
	path := fmt.Sprintf("/api/v1/statuses/%s/%s", url.PathEscape(id), action)
	if err := c.do(ctx, http.MethodPost, path, nil, "", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) Follow(ctx context.Context, accountID string) error {
	return c.accountAction(ctx, accountID, "follow")
}

func (c *Client) Unfollow(ctx context.Context, accountID string) error {
	return c.accountAction(ctx, accountID, "unfollow")
}

func (c *Client) Mute(ctx context.Context, accountID string) error {
	return c.accountAction(ctx, accountID, "mute")
}

func (c *Client) Block(ctx context.Context, accountID string) error {
	return c.accountAction(ctx, accountID, "block")
}

func (c *Client) accountAction(ctx context.Context, accountID, action string) error {
	path := fmt.Sprintf("/api/v1/accounts/%s/%s", url.PathEscape(accountID), action)
	return c.do(ctx, http.MethodPost, path, nil, "", nil)
}

func (c *Client) VotePoll(ctx context.Context, pollID string, choices []int) (*Poll, error) {
	form := url.Values{}
	for _, ch := range choices {
		form.Add("choices[]", fmt.Sprint(ch))
	}
	var p Poll
	path := fmt.Sprintf("/api/v1/polls/%s/votes", url.PathEscape(pollID))
	if err := c.do(ctx, http.MethodPost, path, strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) Search(ctx context.Context, q string, limit int) (*SearchResults, error) {
	query := url.Values{
		"q":       {q},
		"resolve": {"true"},
		"limit":   {fmt.Sprint(limit)},
	}
	var res SearchResults
	if err := c.do(ctx, http.MethodGet, "/api/v2/search?"+query.Encode(), nil, "", &res); err != nil {
		return nil, err
	}
	return &res, nil
}
