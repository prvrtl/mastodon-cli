package mastodon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamSpecHelpers(t *testing.T) {
	if StreamHome().Path != "user" {
		t.Error("home should map to user stream")
	}
	if StreamPublic().Path != "public" {
		t.Error("public path wrong")
	}
	if StreamLocal().Path != "public/local" {
		t.Error("local path wrong")
	}
	if s := StreamList("42"); s.Path != "list" || s.Query.Get("list") != "42" {
		t.Errorf("list spec wrong: %+v", s)
	}
	if s := StreamHashtag("#golang"); s.Path != "hashtag" || s.Query.Get("tag") != "golang" {
		t.Errorf("hashtag spec wrong (hash should be stripped): %+v", s)
	}
}

func TestStreamEndToEnd(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("access_token")
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("server does not support flushing")
			return
		}

		io.WriteString(w, ":thump\n\n")
		io.WriteString(w, "event: update\ndata: {\"id\":\"1\",\"account\":{\"acct\":\"a\"}}\n\n")
		fl.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New("example.social", "tok")
	c.streamBase = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	ch := c.Stream(ctx, StreamHome())

	select {
	case ev := <-ch:
		if ev.Type != EventUpdate || ev.Status == nil || ev.Status.ID != "1" {
			t.Fatalf("unexpected first event: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stream event")
	}
	if gotToken != "tok" {
		t.Errorf("access_token query = %q, want tok", gotToken)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream channel did not close after cancel")
	}
}

func TestStreamBaseURLResolution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"urls":{"streaming_api":"wss://stream.example"}}`)
	}))
	defer srv.Close()
	c := New("example.social", "tok")
	c.base = srv.URL
	if got := c.streamBaseURL(context.Background()); got != "https://stream.example" {
		t.Errorf("streamBaseURL = %q, want https://stream.example", got)
	}

	if got := c.streamBaseURL(context.Background()); got != "https://stream.example" {
		t.Errorf("cached streamBaseURL = %q", got)
	}
}

func TestStreamBaseURLFallsBackToBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	c := New("example.social", "tok")
	c.base = srv.URL
	if got := c.streamBaseURL(context.Background()); got != srv.URL {
		t.Errorf("streamBaseURL fallback = %q, want %q", got, srv.URL)
	}
}

func TestDecodeEvent(t *testing.T) {
	ev, ok := decodeEvent("update", `{"id":"1","content":"<p>hi</p>","account":{"acct":"a"}}`)
	if !ok || ev.Type != EventUpdate || ev.Status == nil || ev.Status.ID != "1" {
		t.Fatalf("update decode failed: %+v ok=%v", ev, ok)
	}

	ev, ok = decodeEvent("notification", `{"id":"2","type":"follow","account":{"acct":"b"}}`)
	if !ok || ev.Type != EventNotification || ev.Notification == nil || ev.Notification.Type != "follow" {
		t.Fatalf("notification decode failed: %+v ok=%v", ev, ok)
	}

	ev, ok = decodeEvent("delete", "123")
	if !ok || ev.Type != EventDelete || ev.DeletedID != "123" {
		t.Fatalf("delete decode failed: %+v ok=%v", ev, ok)
	}

	if _, ok := decodeEvent("filters_changed", ""); ok {
		t.Fatal("expected unknown event to be dropped")
	}
}
