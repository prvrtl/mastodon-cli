package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Account struct {
	Instance     string `json:"instance"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AccessToken  string `json:"access_token"`
	Username     string `json:"username"`
}

type Config struct {
	Accounts []Account `json:"accounts"`
	Active   int       `json:"active"`
}

func (c *Config) Current() *Account {
	if c.Active < 0 || c.Active >= len(c.Accounts) {
		return nil
	}
	return &c.Accounts[c.Active]
}

func (c *Config) LoggedIn() bool {
	a := c.Current()
	return a != nil && a.Instance != "" && a.AccessToken != ""
}

func (c *Config) Add(a Account) {
	for i := range c.Accounts {
		if c.Accounts[i].Instance == a.Instance && c.Accounts[i].Username == a.Username {
			c.Accounts[i] = a
			c.Active = i
			return
		}
	}
	c.Accounts = append(c.Accounts, a)
	c.Active = len(c.Accounts) - 1
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mastocli", "config.json"), nil
}

func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var raw struct {
		Accounts     []Account `json:"accounts"`
		Active       int       `json:"active"`
		Instance     string    `json:"instance"`
		ClientID     string    `json:"client_id"`
		ClientSecret string    `json:"client_secret"`
		AccessToken  string    `json:"access_token"`
		Username     string    `json:"username"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	c := &Config{Accounts: raw.Accounts, Active: raw.Active}
	if len(c.Accounts) == 0 && raw.AccessToken != "" {
		c.Accounts = []Account{{
			Instance:     raw.Instance,
			ClientID:     raw.ClientID,
			ClientSecret: raw.ClientSecret,
			AccessToken:  raw.AccessToken,
			Username:     raw.Username,
		}}
		c.Active = 0
	}
	return c, nil
}

func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
