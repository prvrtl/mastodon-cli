package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func withTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	withTempConfigDir(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if cfg.LoggedIn() {
		t.Fatal("empty config should not be logged in")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempConfigDir(t)
	in := &Config{Accounts: []Account{{
		Instance: "mastodon.example", ClientID: "cid", ClientSecret: "secret",
		AccessToken: "tok", Username: "alice",
	}}, Active: 0}
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Accounts) != 1 || *out.Current() != *in.Current() {
		t.Fatalf("round trip mismatch: %+v vs %+v", out, in)
	}
	if !out.LoggedIn() {
		t.Error("loaded config should be logged in")
	}
}

func TestLegacyMigration(t *testing.T) {
	withTempConfigDir(t)
	p, _ := Path()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)

	legacy := `{"instance":"old.example","client_id":"c","client_secret":"s","access_token":"t","username":"bob"}`
	if err := os.WriteFile(p, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.LoggedIn() || cfg.Current().Username != "bob" || cfg.Current().Instance != "old.example" {
		t.Fatalf("legacy config not migrated: %+v", cfg)
	}
}

func TestAddReplacesAndActivates(t *testing.T) {
	c := &Config{}
	c.Add(Account{Instance: "a.example", Username: "x", AccessToken: "1"})
	c.Add(Account{Instance: "b.example", Username: "y", AccessToken: "2"})
	if len(c.Accounts) != 2 || c.Active != 1 || c.Current().Username != "y" {
		t.Fatalf("Add did not activate the new account: %+v", c)
	}

	c.Add(Account{Instance: "a.example", Username: "x", AccessToken: "9"})
	if len(c.Accounts) != 2 || c.Current().AccessToken != "9" {
		t.Fatalf("Add should replace existing account: %+v", c)
	}
}

func TestSavePermissions(t *testing.T) {
	withTempConfigDir(t)
	c := &Config{Accounts: []Account{{Instance: "x", AccessToken: "y"}}}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perm = %o, want 600", perm)
	}

	data, _ := os.ReadFile(p)
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil || raw["accounts"] == nil {
		t.Error("saved config missing accounts array")
	}
}

func TestLoggedIn(t *testing.T) {
	cases := []struct {
		c    Config
		want bool
	}{
		{Config{}, false},
		{Config{Accounts: []Account{{Instance: "x"}}}, false},
		{Config{Accounts: []Account{{AccessToken: "y"}}}, false},
		{Config{Accounts: []Account{{Instance: "x", AccessToken: "y"}}}, true},
	}
	for _, tc := range cases {
		if got := tc.c.LoggedIn(); got != tc.want {
			t.Errorf("LoggedIn(%+v) = %v, want %v", tc.c, got, tc.want)
		}
	}
}
