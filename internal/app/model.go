package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/model"
	"github.com/wingitman/crosspile/internal/scanner"
	"github.com/wingitman/crosspile/internal/search"
)

type scanFn func(context.Context, *config.Config) scanner.Result

type mode int

const (
	modeLoading mode = iota
	modeOnboarding
	modeNormal
	modeSearch
	modeError
)

type scanMsg scanner.Result
type errorMsg string

type Model struct {
	cfg    *config.Config
	scanFn scanFn
	keys   resolvedKeys

	width  int
	height int
	mode   mode

	sessions []model.Session
	filtered []model.Session
	cursor   int
	offset   int

	filterInput textinput.Model
	locInput    textinput.Model
	filter      string

	scanning bool
	lastScan time.Time
	warnings []string
	status   string
	err      string
}

type resolvedKeys struct {
	up, down, pageUp, pageDown, search, clearSearch, reload, openConfig, quit string
}

func resolveKeys(k config.Keybinds) resolvedKeys {
	return resolvedKeys{k.Up, k.Down, k.PageUp, k.PageDown, k.Search, k.ClearSearch, k.Reload, k.OpenConfig, k.Quit}
}

func New(cfg *config.Config, sf scanFn) Model {
	filterInput := textinput.New()
	filterInput.Placeholder = "filter: text agent:opencode project:crosspile from:2026-05-01 q:prompt a:response"
	filterInput.CharLimit = 512

	locInput := textinput.New()
	locInput.Placeholder = "~/Work, ~/Projects"
	locInput.CharLimit = 1024
	locInput.Focus()

	m := Model{cfg: cfg, scanFn: sf, keys: resolveKeys(cfg.Keybinds), filterInput: filterInput, locInput: locInput}
	if len(cfg.Locations) == 0 {
		m.mode = modeOnboarding
	} else {
		m.mode = modeLoading
		m.scanning = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if len(m.cfg.Locations) == 0 {
		return textinput.Blink
	}
	return m.scanCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case scanMsg:
		m.scanning = false
		m.lastScan = msg.Scanned
		m.sessions = msg.Sessions
		m.warnings = msg.Warnings
		m.applyFilter()
		m.mode = modeNormal
		m.status = fmt.Sprintf("scanned %d sessions", len(m.sessions))
	case errorMsg:
		m.scanning = false
		m.mode = modeError
		m.err = string(msg)
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeOnboarding:
			return m.updateOnboarding(key, msg)
		case modeSearch:
			return m.updateSearch(key, msg)
		default:
			return m.updateNormal(key)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) updateOnboarding(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.quit:
		return m, tea.Quit
	case "enter":
		paths := splitLocations(m.locInput.Value())
		if len(paths) == 0 {
			m.status = "enter at least one work location"
			return m, nil
		}
		if err := config.AddLocations(m.cfg, paths); err != nil {
			m.err = err.Error()
			m.mode = modeError
			return m, nil
		}
		m.mode = modeLoading
		m.scanning = true
		return m, m.scanCmd()
	}
	var cmd tea.Cmd
	m.locInput, cmd = m.locInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.clearSearch:
		m.filterInput.Blur()
		m.filter = m.filterInput.Value()
		m.applyFilter()
		m.mode = modeNormal
		return m, nil
	case "enter":
		m.filterInput.Blur()
		m.filter = m.filterInput.Value()
		m.applyFilter()
		m.mode = modeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filter = m.filterInput.Value()
	m.applyFilter()
	return m, cmd
}

func (m Model) updateNormal(key string) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.quit:
		return m, tea.Quit
	case m.keys.search:
		m.mode = modeSearch
		m.filterInput.Focus()
		return m, textinput.Blink
	case m.keys.clearSearch:
		if m.filter != "" {
			m.filter = ""
			m.filterInput.SetValue("")
			m.applyFilter()
		}
	case m.keys.reload:
		m.scanning = true
		m.status = "scanning..."
		return m, m.scanCmd()
	case m.keys.openConfig:
		return m, openConfigCmd()
	case m.keys.up:
		m.move(-1)
	case m.keys.down:
		m.move(1)
	case m.keys.pageUp:
		m.move(-10)
	case m.keys.pageDown:
		m.move(10)
	}
	return m, nil
}

func (m Model) scanCmd() tea.Cmd {
	cfg := *m.cfg
	return func() tea.Msg {
		res := m.scanFn(context.Background(), &cfg)
		return scanMsg(res)
	}
}

func (m *Model) applyFilter() {
	m.filtered = search.Filter(m.sessions, m.filter)
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m *Model) move(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	height := m.listHeight()
	if height <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+height {
		m.offset = m.cursor - height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func openConfigCmd() tea.Cmd {
	path, err := config.ConfigPath()
	if err != nil {
		return func() tea.Msg { return errorMsg(err.Error()) }
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		switch runtime.GOOS {
		case "windows":
			editor = "notepad"
		default:
			editor = "vi"
		}
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errorMsg(err.Error())
		}
		return nil
	})
}

func splitLocations(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' })
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (m Model) selected() *model.Session {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

func (m Model) listHeight() int {
	return max(1, m.height-8)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:min(len(s), width)]
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+3 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
