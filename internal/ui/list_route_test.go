package ui

import (
	"testing"

	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

func TestSubmitOpenListByNumber(t *testing.T) {
	m := New(nil, &config.Config{})
	m.lists = []mastodon.List{{ID: "12376", Title: "Apex"}, {ID: "12377", Title: "1 TG"}}

	res, cmd := m.submit("/list 1")
	mm := res.(Model)
	if mm.currentListID != "12376" {
		t.Fatalf("currentListID = %q, want 12376", mm.currentListID)
	}
	if cmd == nil {
		t.Fatal("expected a load command, got nil")
	}
}
