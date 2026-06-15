package mastodon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type StreamEventType string

const (
	EventUpdate       StreamEventType = "update"
	EventNotification StreamEventType = "notification"
	EventDelete       StreamEventType = "delete"
	EventError        StreamEventType = "error"
)

type StreamEvent struct {
	Type         StreamEventType
	Status       *Status
	Notification *Notification
	DeletedID    string
	Err          error
}

type StreamSpec struct {
	Path  string
	Query url.Values
}

func StreamHome() StreamSpec   { return StreamSpec{Path: "user"} }
func StreamPublic() StreamSpec { return StreamSpec{Path: "public"} }
func StreamLocal() StreamSpec  { return StreamSpec{Path: "public/local"} }
func StreamList(id string) StreamSpec {
	return StreamSpec{Path: "list", Query: url.Values{"list": {id}}}
}

func StreamHashtag(tag string) StreamSpec {
	return StreamSpec{Path: "hashtag", Query: url.Values{"tag": {strings.TrimPrefix(tag, "#")}}}
}

func (c *Client) Stream(ctx context.Context, spec StreamSpec) <-chan StreamEvent {
	out := make(chan StreamEvent, 32)
	base := c.streamBaseURL(ctx)
	go func() {
		defer close(out)
		backoff := time.Second
		for ctx.Err() == nil {
			err := c.streamOnce(ctx, base, spec, out)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				select {
				case out <- StreamEvent{Type: EventError, Err: err}:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}()
	return out
}

func (c *Client) streamBaseURL(ctx context.Context) string {
	if c.streamBase != "" {
		return c.streamBase
	}
	c.streamBase = c.baseURL()
	var info struct {
		URLs struct {
			StreamingAPI string `json:"streaming_api"`
		} `json:"urls"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/instance", nil, "", &info); err == nil {
		if u := info.URLs.StreamingAPI; u != "" {
			u = strings.Replace(u, "wss://", "https://", 1)
			u = strings.Replace(u, "ws://", "http://", 1)
			c.streamBase = strings.TrimSuffix(u, "/")
		}
	}
	return c.streamBase
}

func (c *Client) streamOnce(ctx context.Context, base string, spec StreamSpec, out chan<- StreamEvent) error {

	q := url.Values{}
	for k, v := range spec.Query {
		q[k] = v
	}
	q.Set("access_token", c.token)
	u := base + "/api/v1/streaming/" + spec.Path + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.Header.Set("Authorization", "Bearer "+c.token)
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream: http %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventName, data string
	flush := func() {
		if eventName == "" {
			return
		}
		if ev, ok := decodeEvent(eventName, data); ok {
			select {
			case out <- ev:
			case <-ctx.Done():
			}
		}
		eventName, data = "", ""
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, ":"):

		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			chunk := strings.TrimPrefix(line[len("data:"):], " ")
			if data != "" {
				data += "\n"
			}
			data += chunk
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("stream closed")
}

func decodeEvent(name, data string) (StreamEvent, bool) {
	switch StreamEventType(name) {
	case EventUpdate:
		var s Status
		if json.Unmarshal([]byte(data), &s) != nil {
			return StreamEvent{}, false
		}
		return StreamEvent{Type: EventUpdate, Status: &s}, true
	case EventNotification:
		var n Notification
		if json.Unmarshal([]byte(data), &n) != nil {
			return StreamEvent{}, false
		}
		return StreamEvent{Type: EventNotification, Notification: &n}, true
	case EventDelete:
		return StreamEvent{Type: EventDelete, DeletedID: strings.TrimSpace(data)}, true
	default:
		return StreamEvent{}, false
	}
}
