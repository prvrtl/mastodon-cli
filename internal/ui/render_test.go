package ui

import (
	"strings"
	"testing"

	"github.com/prvrtl/mastocli/internal/config"
	"github.com/prvrtl/mastocli/internal/mastodon"
)

func TestViewRendersChrome(t *testing.T) {
	m := readyModel(t)
	m.statusLine = "ready"
	out := plain(m.View())
	for _, want := range []string{"Mastodon CLI", "me", "inst.example", "ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q", want)
		}
	}
}

func TestViewWithCommandMenu(t *testing.T) {
	m := readyModel(t)
	m.input.SetValue("/")
	out := plain(m.View())
	if !strings.Contains(out, "/home") || !strings.Contains(out, "Commands") {
		t.Errorf("command menu not rendered in view:\n%s", out)
	}
}

func TestViewWithListPicker(t *testing.T) {
	m := readyModel(t)
	m.lists = []mastodon.List{{ID: "1", Title: "Friends"}}
	m.listPickerOpen = true
	out := plain(m.View())
	if !strings.Contains(out, "Select a list") || !strings.Contains(out, "Friends") {
		t.Errorf("list picker not rendered in view:\n%s", out)
	}
}

func TestViewStreamingBadge(t *testing.T) {

	idle := readyModel(t)
	if strings.ContainsAny(plain(idle.View()), "▂▃▄▅▆▇") {
		t.Error("idle header should not animate")
	}
	m := readyModel(t)
	m.streaming = true
	if !strings.ContainsAny(plain(m.View()), "▂▃▄▅▆▇") {
		t.Error("live header should show the equaliser animation")
	}
}

func TestInitReturnsCommand(t *testing.T) {
	m := New(nil, &config.Config{})
	if m.Init() == nil {
		t.Error("Init should return a startup command batch")
	}
}

func TestOpenGuards(t *testing.T) {
	m := readyModel(t)
	if res, _ := m.submit("/open"); !strings.Contains(res.(Model).statusLine, "Usage") {
		t.Error("/open without arg should show usage")
	}
	if res, _ := m.submit("/open 9"); !strings.Contains(res.(Model).statusLine, "No post") {
		t.Error("/open with bad index should report no post")
	}
}

func TestListPickerEmptyFallback(t *testing.T) {

	if !strings.Contains(renderListMenu(nil), "no lists") {
		t.Error("empty list menu should explain there are no lists")
	}
}
