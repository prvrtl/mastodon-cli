package mastodon

import "time"

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Acct        string `json:"acct"`
	DisplayName string `json:"display_name"`
	URL         string `json:"url"`
	Bot         bool   `json:"bot"`
}

func (a Account) Name() string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Username
}

type Status struct {
	ID               string    `json:"id"`
	URI              string    `json:"uri"`
	URL              string    `json:"url"`
	CreatedAt        time.Time `json:"created_at"`
	Content          string    `json:"content"`
	Account          Account   `json:"account"`
	RepliesCount     int       `json:"replies_count"`
	ReblogsCount     int       `json:"reblogs_count"`
	FavouritesCount  int       `json:"favourites_count"`
	Favourited       bool      `json:"favourited"`
	Reblogged        bool      `json:"reblogged"`
	Bookmarked       bool      `json:"bookmarked"`
	Pinned           bool      `json:"pinned"`
	Sensitive        bool      `json:"sensitive"`
	SpoilerText      string    `json:"spoiler_text"`
	Visibility       string    `json:"visibility"`
	Reblog           *Status   `json:"reblog"`
	InReplyToID      string    `json:"in_reply_to_id"`
	MediaAttachments []Media   `json:"media_attachments"`
	Poll             *Poll     `json:"poll"`
}

type Media struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type Poll struct {
	ID         string       `json:"id"`
	Expired    bool         `json:"expired"`
	Multiple   bool         `json:"multiple"`
	VotesCount int          `json:"votes_count"`
	Voted      bool         `json:"voted"`
	Options    []PollOption `json:"options"`
}

type PollOption struct {
	Title      string `json:"title"`
	VotesCount int    `json:"votes_count"`
}

type Context struct {
	Ancestors   []Status `json:"ancestors"`
	Descendants []Status `json:"descendants"`
}

type Tag struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type SearchResults struct {
	Accounts []Account `json:"accounts"`
	Statuses []Status  `json:"statuses"`
	Hashtags []Tag     `json:"hashtags"`
}

type List struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Account   Account   `json:"account"`
	Status    *Status   `json:"status"`
}

type appRegistration struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}
