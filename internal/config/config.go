package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Locations []Location `toml:"locations"`
	Agents    Agents     `toml:"agents"`
	Display   Display    `toml:"display"`
	Keybinds  Keybinds   `toml:"keybinds"`
}

type Location struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type Agents struct {
	OpenCode bool `toml:"opencode"`
	Claude   bool `toml:"claude"`
	Generic  bool `toml:"generic"`
}

type Display struct {
	PreviewLines int `toml:"preview_lines"`
}

type Keybinds struct {
	Up          string `toml:"up"`
	Down        string `toml:"down"`
	PageUp      string `toml:"page_up"`
	PageDown    string `toml:"page_down"`
	Search      string `toml:"search"`
	ClearSearch string `toml:"clear_search"`
	Reload      string `toml:"reload"`
	OpenConfig  string `toml:"open_config"`
	Quit        string `toml:"quit"`
}

func Default() *Config {
	return &Config{
		Agents: Agents{
			OpenCode: true,
			Claude:   true,
			Generic:  true,
		},
		Display: Display{
			PreviewLines: 12,
		},
		Keybinds: Keybinds{
			Up:          "up",
			Down:        "down",
			PageUp:      "pgup",
			PageDown:    "pgdown",
			Search:      "/",
			ClearSearch: "esc",
			Reload:      "r",
			OpenConfig:  "o",
			Quit:        "q",
		},
	}
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "delbysoft"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crossfile.toml"), nil
}

func Load() (*Config, error) {
	cfg := Default()
	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := Write(path, cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	} else if err != nil {
		return cfg, err
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	applyDefaults(cfg)
	for i := range cfg.Locations {
		cfg.Locations[i].Path = cleanUserPath(cfg.Locations[i].Path)
	}
	if needsMigration(path) {
		_ = Write(path, cfg)
	}
	return cfg, nil
}

func Write(path string, cfg *Config) error {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(BuildTOML(cfg)), 0o644)
}

func AddLocations(cfg *Config, inputs []string) error {
	seen := map[string]bool{}
	for _, loc := range cfg.Locations {
		seen[loc.Path] = true
	}
	for _, raw := range inputs {
		path := cleanUserPath(raw)
		if path == "" || seen[path] {
			continue
		}
		name := filepath.Base(path)
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = path
		}
		cfg.Locations = append(cfg.Locations, Location{Name: name, Path: path})
		seen[path] = true
	}
	return Write("", cfg)
}

func BuildTOML(cfg *Config) string {
	var b bytes.Buffer
	b.WriteString("# crosspile configuration file\n")
	b.WriteString("# Stores work locations and scan preferences for AI-agent history.\n\n")
	b.WriteString("# Add one block per work root that should be included in results.\n")
	if len(cfg.Locations) == 0 {
		b.WriteString("# [[locations]]\n")
		b.WriteString("# name = \"Work\"\n")
		b.WriteString("# path = \"~/Work\"\n\n")
	} else {
		for _, loc := range cfg.Locations {
			b.WriteString("[[locations]]\n")
			b.WriteString("name = " + quote(loc.Name) + "\n")
			b.WriteString("path = " + quote(loc.Path) + "\n\n")
		}
	}
	b.WriteString("[agents]\n")
	b.WriteString(fmt.Sprintf("opencode = %t\n", cfg.Agents.OpenCode))
	b.WriteString(fmt.Sprintf("claude   = %t\n", cfg.Agents.Claude))
	b.WriteString(fmt.Sprintf("generic  = %t\n\n", cfg.Agents.Generic))
	b.WriteString("[display]\n")
	b.WriteString(fmt.Sprintf("preview_lines = %d\n\n", cfg.Display.PreviewLines))
	b.WriteString("[keybinds]\n")
	b.WriteString("up           = " + quote(cfg.Keybinds.Up) + "\n")
	b.WriteString("down         = " + quote(cfg.Keybinds.Down) + "\n")
	b.WriteString("page_up      = " + quote(cfg.Keybinds.PageUp) + "\n")
	b.WriteString("page_down    = " + quote(cfg.Keybinds.PageDown) + "\n")
	b.WriteString("search       = " + quote(cfg.Keybinds.Search) + "\n")
	b.WriteString("clear_search = " + quote(cfg.Keybinds.ClearSearch) + "\n")
	b.WriteString("reload       = " + quote(cfg.Keybinds.Reload) + "\n")
	b.WriteString("open_config  = " + quote(cfg.Keybinds.OpenConfig) + "\n")
	b.WriteString("quit         = " + quote(cfg.Keybinds.Quit) + "\n")
	return b.String()
}

func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

func applyDefaults(cfg *Config) {
	d := Default()
	if cfg.Display.PreviewLines <= 0 {
		cfg.Display.PreviewLines = d.Display.PreviewLines
	}
	if cfg.Keybinds.Up == "" {
		cfg.Keybinds.Up = d.Keybinds.Up
	}
	if cfg.Keybinds.Down == "" {
		cfg.Keybinds.Down = d.Keybinds.Down
	}
	if cfg.Keybinds.PageUp == "" {
		cfg.Keybinds.PageUp = d.Keybinds.PageUp
	}
	if cfg.Keybinds.PageDown == "" {
		cfg.Keybinds.PageDown = d.Keybinds.PageDown
	}
	if cfg.Keybinds.Search == "" {
		cfg.Keybinds.Search = d.Keybinds.Search
	}
	if cfg.Keybinds.ClearSearch == "" {
		cfg.Keybinds.ClearSearch = d.Keybinds.ClearSearch
	}
	if cfg.Keybinds.Reload == "" {
		cfg.Keybinds.Reload = d.Keybinds.Reload
	}
	if cfg.Keybinds.OpenConfig == "" {
		cfg.Keybinds.OpenConfig = d.Keybinds.OpenConfig
	}
	if cfg.Keybinds.Quit == "" {
		cfg.Keybinds.Quit = d.Keybinds.Quit
	}
}

func needsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(data)
	for _, required := range []string{"[agents]", "[display]", "[keybinds]", "preview_lines", "open_config"} {
		if !strings.Contains(s, required) {
			return true
		}
	}
	return false
}

func cleanUserPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "\"'")
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				path = home
			} else if strings.HasPrefix(path, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(path, "~\\")) {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}
