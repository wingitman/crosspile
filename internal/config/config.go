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
	Updates   Updates    `toml:"updates"`
	Apps      Apps       `toml:"apps"`
	Output    Output     `toml:"output"`
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

type Updates struct {
	CheckOnStartup bool   `toml:"check_on_startup"`
	AutoPrompt     bool   `toml:"auto_prompt"`
	RemoteURL      string `toml:"remote_url"`
}

type Apps struct {
	Editor string `toml:"editor"`
}

type Output struct {
	ExportDir string `toml:"export_dir"`
}

type Keybinds struct {
	Up              string `toml:"up"`
	Down            string `toml:"down"`
	Left            string `toml:"left"`
	Right           string `toml:"right"`
	Confirm         string `toml:"confirm"`
	Back            string `toml:"back"`
	PageUp          string `toml:"page_up"`
	PageDown        string `toml:"page_down"`
	Search          string `toml:"search"`
	ClearSearch     string `toml:"clear_search"`
	Reload          string `toml:"reload"`
	OpenConfig      string `toml:"open_config"`
	CheckUpdate     string `toml:"check_update"`
	OpenDocument    string `toml:"open_document"`
	RawView         string `toml:"raw_view"`
	RawNextTable    string `toml:"raw_next_table"`
	RawPrevTable    string `toml:"raw_prev_table"`
	ExportCSV       string `toml:"export_csv"`
	ExportJSON      string `toml:"export_json"`
	FilterHelp      string `toml:"filter_help"`
	Analytics       string `toml:"analytics"`
	AnalyticsNext   string `toml:"analytics_next"`
	AnalyticsPrev   string `toml:"analytics_prev"`
	AnalyticsBucket string `toml:"analytics_bucket"`
	AnalyticsView   string `toml:"analytics_view"`
	AnalyticsFocus  string `toml:"analytics_focus"`
	DetailUp        string `toml:"detail_up"`
	DetailDown      string `toml:"detail_down"`
	DetailPageUp    string `toml:"detail_page_up"`
	DetailPageDown  string `toml:"detail_page_down"`
	DetailTop       string `toml:"detail_top"`
	DetailBottom    string `toml:"detail_bottom"`
	Quit            string `toml:"quit"`
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
		Updates: Updates{
			CheckOnStartup: true,
			AutoPrompt:     true,
			RemoteURL:      "https://github.com/wingitman/crosspile.git",
		},
		Keybinds: Keybinds{
			Up:              "up",
			Down:            "down",
			Left:            "left",
			Right:           "right",
			Confirm:         "enter",
			Back:            "esc",
			PageUp:          "pgup",
			PageDown:        "pgdown",
			Search:          "/",
			ClearSearch:     "esc",
			Reload:          "r",
			OpenConfig:      "o",
			CheckUpdate:     "u",
			OpenDocument:    "e",
			RawView:         "R",
			RawNextTable:    "tab",
			RawPrevTable:    "shift+tab",
			ExportCSV:       "c",
			ExportJSON:      "J",
			FilterHelp:      "?",
			Analytics:       "A",
			AnalyticsNext:   "right",
			AnalyticsPrev:   "left",
			AnalyticsBucket: "b",
			AnalyticsView:   "v",
			AnalyticsFocus:  "tab",
			DetailUp:        "k",
			DetailDown:      "j",
			DetailPageUp:    "ctrl+u",
			DetailPageDown:  "ctrl+d",
			DetailTop:       "g",
			DetailBottom:    "G",
			Quit:            "q",
		},
	}
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "delbysoft"), nil
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

func ResetDefault() error {
	return Write("", Default())
}

func ResolveEditor(cfgEditor string) string {
	candidates := ResolveEditorWithFallbacks(cfgEditor)
	if len(candidates) == 0 {
		return "vi"
	}
	return candidates[0]
}

func ResolveEditorWithFallbacks(cfgEditor string) []string {
	var candidates []string
	if cfgEditor != "" {
		candidates = append(candidates, cfgEditor)
	}
	if e := os.Getenv("EDITOR"); e != "" {
		candidates = append(candidates, e)
	}
	if v := os.Getenv("VISUAL"); v != "" {
		candidates = append(candidates, v)
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "notepad")
	} else {
		candidates = append(candidates, "vim", "nano", "vi")
	}
	return candidates
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

	migrate := needsMigration(path)
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	if migrate {
		applyMigrationDefaults(path, cfg)
	}
	applyDefaults(cfg)
	for i := range cfg.Locations {
		cfg.Locations[i].Path = cleanUserPath(cfg.Locations[i].Path)
	}
	if migrate {
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
	b.WriteString("# Use one [[locations]] block per root. First-run setup also accepts commas or new lines.\n")
	if len(cfg.Locations) == 0 {
		b.WriteString("# [[locations]]\n")
		b.WriteString("# name = \"Work\"\n")
		b.WriteString("# path = \"~/Work\"\n\n")
		b.WriteString("# [[locations]]\n")
		b.WriteString("# name = \"Projects\"\n")
		b.WriteString("# path = \"~/Projects\"\n\n")
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
	b.WriteString("[updates]\n")
	b.WriteString(fmt.Sprintf("check_on_startup = %t\n", cfg.Updates.CheckOnStartup))
	b.WriteString(fmt.Sprintf("auto_prompt      = %t\n", cfg.Updates.AutoPrompt))
	b.WriteString("remote_url       = " + quote(cfg.Updates.RemoteURL) + "\n\n")
	b.WriteString("[apps]\n")
	b.WriteString("editor = " + quote(cfg.Apps.Editor) + "\n\n")
	b.WriteString("[output]\n")
	b.WriteString("export_dir = " + quote(cfg.Output.ExportDir) + "\n\n")
	b.WriteString("[keybinds]\n")
	b.WriteString("up           = " + quote(cfg.Keybinds.Up) + "\n")
	b.WriteString("down         = " + quote(cfg.Keybinds.Down) + "\n")
	b.WriteString("left         = " + quote(cfg.Keybinds.Left) + "\n")
	b.WriteString("right        = " + quote(cfg.Keybinds.Right) + "\n")
	b.WriteString("confirm      = " + quote(cfg.Keybinds.Confirm) + "\n")
	b.WriteString("back         = " + quote(cfg.Keybinds.Back) + "\n")
	b.WriteString("page_up      = " + quote(cfg.Keybinds.PageUp) + "\n")
	b.WriteString("page_down    = " + quote(cfg.Keybinds.PageDown) + "\n")
	b.WriteString("search       = " + quote(cfg.Keybinds.Search) + "\n")
	b.WriteString("clear_search = " + quote(cfg.Keybinds.ClearSearch) + "\n")
	b.WriteString("reload       = " + quote(cfg.Keybinds.Reload) + "\n")
	b.WriteString("open_config  = " + quote(cfg.Keybinds.OpenConfig) + "\n")
	b.WriteString("check_update = " + quote(cfg.Keybinds.CheckUpdate) + "\n")
	b.WriteString("open_document = " + quote(cfg.Keybinds.OpenDocument) + "\n")
	b.WriteString("raw_view      = " + quote(cfg.Keybinds.RawView) + "\n")
	b.WriteString("raw_next_table = " + quote(cfg.Keybinds.RawNextTable) + "\n")
	b.WriteString("raw_prev_table = " + quote(cfg.Keybinds.RawPrevTable) + "\n")
	b.WriteString("export_csv    = " + quote(cfg.Keybinds.ExportCSV) + "\n")
	b.WriteString("export_json   = " + quote(cfg.Keybinds.ExportJSON) + "\n")
	b.WriteString("filter_help   = " + quote(cfg.Keybinds.FilterHelp) + "\n")
	b.WriteString("analytics     = " + quote(cfg.Keybinds.Analytics) + "\n")
	b.WriteString("analytics_next = " + quote(cfg.Keybinds.AnalyticsNext) + "\n")
	b.WriteString("analytics_prev = " + quote(cfg.Keybinds.AnalyticsPrev) + "\n")
	b.WriteString("analytics_bucket = " + quote(cfg.Keybinds.AnalyticsBucket) + "\n")
	b.WriteString("analytics_view = " + quote(cfg.Keybinds.AnalyticsView) + "\n")
	b.WriteString("analytics_focus = " + quote(cfg.Keybinds.AnalyticsFocus) + "\n")
	b.WriteString("detail_up     = " + quote(cfg.Keybinds.DetailUp) + "\n")
	b.WriteString("detail_down   = " + quote(cfg.Keybinds.DetailDown) + "\n")
	b.WriteString("detail_page_up = " + quote(cfg.Keybinds.DetailPageUp) + "\n")
	b.WriteString("detail_page_down = " + quote(cfg.Keybinds.DetailPageDown) + "\n")
	b.WriteString("detail_top    = " + quote(cfg.Keybinds.DetailTop) + "\n")
	b.WriteString("detail_bottom = " + quote(cfg.Keybinds.DetailBottom) + "\n")
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
	if cfg.Updates.RemoteURL == "" {
		cfg.Updates.RemoteURL = d.Updates.RemoteURL
	}
	if cfg.Keybinds.Up == "" {
		cfg.Keybinds.Up = d.Keybinds.Up
	}
	if cfg.Keybinds.Down == "" {
		cfg.Keybinds.Down = d.Keybinds.Down
	}
	if cfg.Keybinds.Left == "" {
		cfg.Keybinds.Left = d.Keybinds.Left
	}
	if cfg.Keybinds.Right == "" {
		cfg.Keybinds.Right = d.Keybinds.Right
	}
	if cfg.Keybinds.Confirm == "" {
		cfg.Keybinds.Confirm = d.Keybinds.Confirm
	}
	if cfg.Keybinds.Back == "" {
		cfg.Keybinds.Back = d.Keybinds.Back
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
	if cfg.Keybinds.CheckUpdate == "" {
		cfg.Keybinds.CheckUpdate = d.Keybinds.CheckUpdate
	}
	if cfg.Keybinds.OpenDocument == "" {
		cfg.Keybinds.OpenDocument = d.Keybinds.OpenDocument
	}
	if cfg.Keybinds.RawView == "" {
		cfg.Keybinds.RawView = d.Keybinds.RawView
	}
	if cfg.Keybinds.RawNextTable == "" {
		cfg.Keybinds.RawNextTable = d.Keybinds.RawNextTable
	}
	if cfg.Keybinds.RawPrevTable == "" {
		cfg.Keybinds.RawPrevTable = d.Keybinds.RawPrevTable
	}
	if cfg.Keybinds.ExportCSV == "" {
		cfg.Keybinds.ExportCSV = d.Keybinds.ExportCSV
	}
	if cfg.Keybinds.ExportJSON == "" {
		cfg.Keybinds.ExportJSON = d.Keybinds.ExportJSON
	}
	if cfg.Keybinds.FilterHelp == "" {
		cfg.Keybinds.FilterHelp = d.Keybinds.FilterHelp
	}
	if cfg.Keybinds.Analytics == "" {
		cfg.Keybinds.Analytics = d.Keybinds.Analytics
	}
	if cfg.Keybinds.AnalyticsNext == "" {
		cfg.Keybinds.AnalyticsNext = d.Keybinds.AnalyticsNext
	}
	if cfg.Keybinds.AnalyticsPrev == "" {
		cfg.Keybinds.AnalyticsPrev = d.Keybinds.AnalyticsPrev
	}
	if cfg.Keybinds.AnalyticsBucket == "" {
		cfg.Keybinds.AnalyticsBucket = d.Keybinds.AnalyticsBucket
	}
	if cfg.Keybinds.AnalyticsView == "" {
		cfg.Keybinds.AnalyticsView = d.Keybinds.AnalyticsView
	}
	if cfg.Keybinds.AnalyticsFocus == "" {
		cfg.Keybinds.AnalyticsFocus = d.Keybinds.AnalyticsFocus
	}
	if cfg.Keybinds.DetailUp == "" {
		cfg.Keybinds.DetailUp = d.Keybinds.DetailUp
	}
	if cfg.Keybinds.DetailDown == "" {
		cfg.Keybinds.DetailDown = d.Keybinds.DetailDown
	}
	if cfg.Keybinds.DetailPageUp == "" {
		cfg.Keybinds.DetailPageUp = d.Keybinds.DetailPageUp
	}
	if cfg.Keybinds.DetailPageDown == "" {
		cfg.Keybinds.DetailPageDown = d.Keybinds.DetailPageDown
	}
	if cfg.Keybinds.DetailTop == "" {
		cfg.Keybinds.DetailTop = d.Keybinds.DetailTop
	}
	if cfg.Keybinds.DetailBottom == "" {
		cfg.Keybinds.DetailBottom = d.Keybinds.DetailBottom
	}
	if cfg.Keybinds.Quit == "" {
		cfg.Keybinds.Quit = d.Keybinds.Quit
	}
}

func applyMigrationDefaults(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(data)
	d := Default()
	if !strings.Contains(s, "check_on_startup") {
		cfg.Updates.CheckOnStartup = d.Updates.CheckOnStartup
	}
	if !strings.Contains(s, "auto_prompt") {
		cfg.Updates.AutoPrompt = d.Updates.AutoPrompt
	}
}

func needsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(data)
	for _, required := range []string{"[agents]", "[display]", "[updates]", "[apps]", "[output]", "[keybinds]", "preview_lines", "open_config", "check_update", "open_document", "raw_view", "raw_next_table", "export_csv", "filter_help", "analytics", "analytics_view", "analytics_focus", "detail_down", "confirm", "remote_url"} {
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
