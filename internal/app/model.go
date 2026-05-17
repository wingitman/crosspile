package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wingitman/crosspile/internal/analytics"
	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/model"
	"github.com/wingitman/crosspile/internal/scanner"
	"github.com/wingitman/crosspile/internal/search"
	"github.com/wingitman/crosspile/internal/updatecheck"
)

type scanFn func(context.Context, *config.Config) scanner.Result

type mode int

const (
	modeLoading mode = iota
	modeOnboarding
	modeNormal
	modeSearch
	modeFilterHelp
	modeAnalytics
	modeRawData
	modeError
)

type scanMsg scanner.Result
type errorMsg string
type configEditorClosedMsg struct{}
type configReloadedMsg struct {
	cfg *config.Config
	err error
}
type configCheckMsg struct {
	modTime time.Time
}
type updateCheckResultMsg struct {
	result updatecheck.Result
}
type statusMsg string
type editorClosedMsg struct{}
type rawCellEditorClosedMsg struct{}
type rawCellEditorReadyMsg struct {
	path string
	err  error
}
type rawLoadedMsg struct {
	session model.Session
	data    rawData
	err     error
}
type rawPageLoadedMsg struct {
	sessionID string
	source    string
	table     string
	data      rawTable
	err       error
}

type Model struct {
	cfg    *config.Config
	scanFn scanFn
	keys   resolvedKeys

	width  int
	height int
	mode   mode

	sessions     []model.Session
	filtered     []model.Session
	cursor       int
	offset       int
	detailScroll int

	filterInput textinput.Model
	locInput    textinput.Model
	filter      string

	scanning bool
	lastScan time.Time
	warnings []string
	status   string
	err      string

	configPath    string
	configModTime time.Time

	updateOrigin    string
	updateRepoDir   string
	currentVersion  string
	updatePrompt    bool
	reinstallPrompt bool
	updateLatest    string
	updateMessage   string
	updateChanges   []string
	updateChoice    int
	updatePull      bool

	raw          rawData
	rawSession   model.Session
	rawLoading   bool
	rawTable     int
	rawRow       int
	rawCol       int
	rawRowOpen   bool
	rawRowOffset int
	rawColOffset int

	analyticsDimension    int
	analyticsBucket       int
	analyticsView         int
	analyticsFocus        int
	analyticsMetricCursor int
	analyticsMetrics      []bool
}

type resolvedKeys struct {
	up, down, left, right, confirm, back, pageUp, pageDown, search, clearSearch, reload, openConfig, checkUpdate, openDocument, rawView, rawNextTable, rawPrevTable, exportCSV, exportJSON, filterHelp, analytics, analyticsNext, analyticsPrev, analyticsBucket, analyticsView, analyticsFocus, detailUp, detailDown, detailPageUp, detailPageDown, detailTop, detailBottom, quit string
}

func resolveKeys(k config.Keybinds) resolvedKeys {
	return resolvedKeys{k.Up, k.Down, k.Left, k.Right, k.Confirm, k.Back, k.PageUp, k.PageDown, k.Search, k.ClearSearch, k.Reload, k.OpenConfig, k.CheckUpdate, k.OpenDocument, k.RawView, k.RawNextTable, k.RawPrevTable, k.ExportCSV, k.ExportJSON, k.FilterHelp, k.Analytics, k.AnalyticsNext, k.AnalyticsPrev, k.AnalyticsBucket, k.AnalyticsView, k.AnalyticsFocus, k.DetailUp, k.DetailDown, k.DetailPageUp, k.DetailPageDown, k.DetailTop, k.DetailBottom, k.Quit}
}

func New(cfg *config.Config, sf scanFn, origin, version, repoDir string) Model {
	filterInput := textinput.New()
	filterInput.Placeholder = "filter: text agent:opencode project:crosspile from:2026-05-01 q:prompt a:response"
	filterInput.CharLimit = 512

	locInput := textinput.New()
	locInput.Placeholder = "~/Work, ~/Projects"
	locInput.CharLimit = 1024
	locInput.Focus()

	updateState := updatecheck.ResolveState(updatecheck.BakedInfo{Origin: origin, Version: version, RepoDir: repoDir})
	configPath, _ := config.ConfigPath()
	m := Model{
		cfg:              cfg,
		scanFn:           sf,
		keys:             resolveKeys(cfg.Keybinds),
		filterInput:      filterInput,
		locInput:         locInput,
		configPath:       configPath,
		updateOrigin:     firstNonEmpty(updateState.Origin, cfg.Updates.RemoteURL),
		updateRepoDir:    updateState.RepoDir,
		currentVersion:   updateState.InstalledCommit,
		analyticsMetrics: analytics.DefaultSelectedMetrics(),
	}
	m.configModTime = fileModTime(configPath)
	if len(cfg.Locations) == 0 {
		m.mode = modeOnboarding
	} else {
		m.mode = modeLoading
		m.scanning = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.configWatchCmd()}
	if m.cfg.Updates.CheckOnStartup {
		cmds = append(cmds, m.checkUpdateCmd())
	}
	if len(m.cfg.Locations) == 0 {
		cmds = append(cmds, textinput.Blink)
		return tea.Batch(cmds...)
	}
	cmds = append(cmds, m.scanCmd())
	return tea.Batch(cmds...)
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
	case statusMsg:
		m.status = string(msg)
	case editorClosedMsg:
		m.status = "editor closed"
	case rawCellEditorClosedMsg:
		m.status = "cell editor closed"
	case rawCellEditorReadyMsg:
		if msg.err != nil {
			m.status = "cell editor failed: " + msg.err.Error()
			return m, nil
		}
		return m, m.openRawCellEditorCmd(msg.path)
	case rawLoadedMsg:
		if msg.session.ID != m.rawSession.ID || msg.session.Source != m.rawSession.Source {
			return m, nil
		}
		m.rawLoading = false
		if msg.err != nil {
			m.status = "raw data failed: " + msg.err.Error()
			return m, nil
		}
		m.raw = msg.data
		m.rawSession = msg.session
		m.rawTable, m.rawRow, m.rawCol, m.rawRowOffset, m.rawColOffset = 0, 0, 0, 0, 0
		m.rawRowOpen = false
		m.mode = modeRawData
	case rawPageLoadedMsg:
		if msg.sessionID != m.rawSession.ID || msg.source != m.rawSession.Source || len(m.raw.Tables) == 0 || m.rawTable >= len(m.raw.Tables) || msg.table != m.raw.Tables[m.rawTable].Name {
			return m, nil
		}
		m.rawLoading = false
		if msg.err != nil {
			m.status = "raw page failed: " + msg.err.Error()
			return m, nil
		}
		m.raw.Tables[m.rawTable] = msg.data
		m.rawRow, m.rawRowOffset = 0, 0
		m.rawRowOpen = false
		if m.rawCol >= len(msg.data.Columns) {
			m.rawCol = len(msg.data.Columns) - 1
		}
		if m.rawCol < 0 {
			m.rawCol = 0
		}
		m.status = fmt.Sprintf("loaded raw page %d/%d", msg.data.Page+1, msg.data.rawPageCount())
	case configEditorClosedMsg:
		return m, m.reloadConfigCmd(true)
	case configReloadedMsg:
		if msg.err != nil {
			m.status = "config reload failed: " + msg.err.Error()
			return m, nil
		}
		oldLocations := locationsSignature(m.cfg)
		oldAgents := agentsSignature(m.cfg)
		m.cfg = msg.cfg
		m.keys = resolveKeys(msg.cfg.Keybinds)
		m.configModTime = fileModTime(m.configPath)
		m.status = "config reloaded"
		if oldLocations != locationsSignature(msg.cfg) || oldAgents != agentsSignature(msg.cfg) {
			m.scanning = true
			return m, m.scanCmd()
		}
	case configCheckMsg:
		cmds = append(cmds, m.configWatchCmd())
		if !msg.modTime.IsZero() && !m.configModTime.IsZero() && msg.modTime.After(m.configModTime) {
			cmds = append(cmds, m.reloadConfigCmd(false))
		} else if m.configModTime.IsZero() {
			m.configModTime = msg.modTime
		}
	case updateCheckResultMsg:
		m = m.applyUpdateResult(msg.result, m.cfg.Updates.AutoPrompt)
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.updatePrompt || m.reinstallPrompt {
			return m.updateUpdatePrompt(key)
		}
		switch m.mode {
		case modeOnboarding:
			return m.updateOnboarding(key, msg)
		case modeSearch:
			return m.updateSearch(key, msg)
		case modeFilterHelp:
			return m.updateFilterHelp(key)
		case modeAnalytics:
			return m.updateAnalytics(key)
		case modeRawData:
			return m.updateRaw(key)
		default:
			return m.updateNormal(key)
		}
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) updateOnboarding(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.quit:
		return m, tea.Quit
	case m.keys.confirm:
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
	case m.keys.clearSearch, m.keys.back:
		m.filterInput.Blur()
		m.filter = m.filterInput.Value()
		m.applyFilter()
		m.mode = modeNormal
		return m, nil
	case m.keys.confirm:
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
	case m.keys.filterHelp:
		m.mode = modeFilterHelp
	case m.keys.analytics:
		m.mode = modeAnalytics
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
		return m, m.openConfigCmd()
	case m.keys.checkUpdate:
		m.status = "checking for updates..."
		return m, m.checkUpdateCmd()
	case m.keys.openDocument:
		return m, m.openSelectedDocumentCmd()
	case m.keys.rawView:
		if s := m.selected(); s != nil {
			m.rawSession = *s
			m.rawLoading = true
			m.status = "loading raw data..."
			return m, loadRawDataCmd(*s)
		}
	case m.keys.up:
		m.move(-1)
	case m.keys.down:
		m.move(1)
	case m.keys.pageUp:
		m.move(-10)
	case m.keys.pageDown:
		m.move(10)
	case m.keys.detailUp:
		m.scrollDetail(-1)
	case m.keys.detailDown:
		m.scrollDetail(1)
	case m.keys.detailPageUp:
		m.scrollDetail(-m.listHeight() / 2)
	case m.keys.detailPageDown:
		m.scrollDetail(m.listHeight() / 2)
	case m.keys.detailTop:
		m.detailScroll = 0
	case m.keys.detailBottom:
		m.detailScroll = 1 << 30
	}
	return m, nil
}

func (m Model) updateFilterHelp(key string) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.back, m.keys.clearSearch, m.keys.filterHelp:
		m.mode = modeNormal
	case m.keys.quit:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateAnalytics(key string) (tea.Model, tea.Cmd) {
	const (
		analyticsFocusDimension = iota
		analyticsFocusMetric
	)
	switch key {
	case m.keys.back, m.keys.clearSearch, m.keys.analytics:
		m.mode = modeNormal
	case m.keys.quit:
		return m, tea.Quit
	case m.keys.analyticsFocus:
		if m.analyticsFocus == analyticsFocusDimension {
			m.analyticsFocus = analyticsFocusMetric
		} else {
			m.analyticsFocus = analyticsFocusDimension
		}
	case m.keys.analyticsNext, m.keys.right:
		m.analyticsDimension = (m.analyticsDimension + 1) % len(analytics.Dimensions)
	case m.keys.analyticsPrev, m.keys.left:
		m.analyticsDimension--
		if m.analyticsDimension < 0 {
			m.analyticsDimension = len(analytics.Dimensions) - 1
		}
	case m.keys.analyticsBucket:
		m.analyticsBucket = (m.analyticsBucket + 1) % len(analytics.Buckets)
	case m.keys.analyticsView:
		m.analyticsView = (m.analyticsView + 1) % analyticsViewCount
	case m.keys.up:
		if m.analyticsFocus == analyticsFocusDimension {
			m.analyticsDimension--
		} else {
			m.analyticsMetricCursor--
		}
	case m.keys.down:
		if m.analyticsFocus == analyticsFocusDimension {
			m.analyticsDimension++
		} else {
			m.analyticsMetricCursor++
		}
	case m.keys.confirm:
		if m.analyticsFocus == analyticsFocusMetric {
			if len(m.analyticsMetrics) == 0 {
				m.analyticsMetrics = analytics.DefaultSelectedMetrics()
			}
			m.analyticsMetrics[m.analyticsMetricCursor] = !m.analyticsMetrics[m.analyticsMetricCursor]
		}
	}
	if m.analyticsDimension < 0 {
		m.analyticsDimension = len(analytics.Dimensions) - 1
	}
	if m.analyticsDimension >= len(analytics.Dimensions) {
		m.analyticsDimension = 0
	}
	if m.analyticsMetricCursor < 0 {
		m.analyticsMetricCursor = 0
	}
	if m.analyticsMetricCursor >= len(analytics.Metrics) {
		m.analyticsMetricCursor = len(analytics.Metrics) - 1
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
	old := m.cursor
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if old != m.cursor {
		m.detailScroll = 0
	}
	m.ensureCursorVisible()
}

func (m *Model) scrollDetail(delta int) {
	m.detailScroll += delta
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
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

func (m Model) openConfigCmd() tea.Cmd {
	path, err := config.ConfigPath()
	if err != nil {
		return func() tea.Msg { return errorMsg(err.Error()) }
	}
	editor := config.ResolveEditor(m.cfg.Apps.Editor)
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errorMsg(err.Error())
		}
		return configEditorClosedMsg{}
	})
}

func (m Model) reloadConfigCmd(forceScan bool) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return configReloadedMsg{err: err}
		}
		if forceScan {
			return configReloadedMsg{cfg: cfg}
		}
		return configReloadedMsg{cfg: cfg}
	}
}

func (m Model) configWatchCmd() tea.Cmd {
	path := m.configPath
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return configCheckMsg{modTime: fileModTime(path)}
	})
}

func (m Model) checkUpdateCmd() tea.Cmd {
	baked := updatecheck.BakedInfo{Origin: firstNonEmpty(m.cfg.Updates.RemoteURL, m.updateOrigin), Version: m.currentVersion, RepoDir: m.updateRepoDir}
	return func() tea.Msg {
		return updateCheckResultMsg{result: updatecheck.StartupCheck(baked)}
	}
}

func (m Model) applyUpdateResult(result updatecheck.Result, prompt bool) Model {
	switch result.Kind {
	case updatecheck.UpToDate:
		m.status = "crosspile is up to date"
	case updatecheck.CheckUnavailable:
		m.status = "update check unavailable"
	case updatecheck.RemoteAhead:
		m.updateLatest = result.State.RemoteCommit
		m.updateMessage = result.Message
		m.updateChanges = result.RecentChanges
		m.updateRepoDir = result.State.RepoDir
		m.updateOrigin = result.State.Origin
		m.updatePull = true
		m.updateChoice = 0
		m.status = "update available " + result.State.RemoteCommit
		if prompt {
			m.updatePrompt = true
		}
	case updatecheck.LocalRepoAhead, updatecheck.MetadataMissing, updatecheck.RepoMissing:
		m.reinstallPrompt = prompt
		m.updateMessage = result.Message
		m.updateChanges = result.RecentChanges
		m.updateRepoDir = result.State.RepoDir
		m.updateOrigin = result.State.Origin
		m.updateChoice = 0
		m.updatePull = false
		m.status = result.Kind.String()
	}
	return m
}

func (m Model) updateUpdatePrompt(key string) (tea.Model, tea.Cmd) {
	switch key {
	case m.keys.left, m.keys.right, m.keys.up, m.keys.down:
		if m.updateChoice == 0 {
			m.updateChoice = 1
		} else {
			m.updateChoice = 0
		}
	case m.keys.confirm:
		if m.updateChoice == 1 {
			m.updatePrompt = false
			m.reinstallPrompt = false
			return m, m.runUpdateCmd(m.updatePull)
		}
		m.updatePrompt = false
		m.reinstallPrompt = false
		m.updateChoice = 0
	case m.keys.back, m.keys.clearSearch, m.keys.quit:
		m.updatePrompt = false
		m.reinstallPrompt = false
		m.updateChoice = 0
	}
	return m, nil
}

func (m Model) runUpdateCmd(pullFirst bool) tea.Cmd {
	repoDir := resolveRepoDir(m.updateRepoDir)
	return func() tea.Msg {
		cmd := buildDetachedUpdateCmd(repoDir, pullFirst, os.Getpid())
		if err := cmd.Run(); err != nil {
			return statusMsg("update failed to start: " + err.Error())
		}
		return tea.Quit()
	}
}

func resolveRepoDir(embedded string) string {
	meta := config.ReadInstallMeta()
	if meta.RepoDir != "" {
		if info, err := os.Stat(meta.RepoDir); err == nil && info.IsDir() {
			return meta.RepoDir
		}
	}
	if embedded != "" {
		if info, err := os.Stat(embedded); err == nil && info.IsDir() {
			return embedded
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Downloads", "crosspile")
}

func buildDetachedUpdateCmd(repoDir string, pullFirst bool, parentPID int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return buildWindowsDetachedUpdateCmd(repoDir, pullFirst, parentPID)
	}
	return buildUnixDetachedUpdateCmd(repoDir, pullFirst, parentPID)
}

func buildWindowsDetachedUpdateCmd(repoDir string, pullFirst bool, parentPID int) *exec.Cmd {
	inner := fmt.Sprintf(`$ErrorActionPreference='Stop'; Write-Host 'Waiting for crosspile to exit before updating...'; Wait-Process -Id %d -ErrorAction SilentlyContinue; `, parentPID)
	if isGitRepo(repoDir) && pullFirst {
		inner += fmt.Sprintf(`$env:GCM_INTERACTIVE='never'; git -C %s pull; $pullCode=$LASTEXITCODE; $env:GCM_INTERACTIVE=$null; if ($pullCode -ne 0) { exit $pullCode }; `, psSingleQuote(repoDir))
	}
	installer := filepath.Join(repoDir, "install.ps1")
	if _, err := os.Stat(installer); err == nil {
		inner += fmt.Sprintf(`& %s; `, psSingleQuote(installer))
	} else {
		inner += fmt.Sprintf(`Write-Host 'Cannot update automatically: install.ps1 was not found at %s.' -ForegroundColor Yellow; `, psSingleQuote(repoDir))
	}
	inner += `Write-Host ''; Write-Host 'Update process finished. You can close this window and reopen crosspile.' -ForegroundColor Green`
	wrapper := fmt.Sprintf(`Start-Process powershell -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-NoExit','-Command',%s)`, psSingleQuote(inner))
	return exec.Command("powershell", "-NoProfile", "-Command", wrapper)
}

func buildUnixDetachedUpdateCmd(repoDir string, pullFirst bool, parentPID int) *exec.Cmd {
	logPath := filepath.Join(os.TempDir(), "crosspile-update.log")
	inner := fmt.Sprintf(`echo "Waiting for crosspile to exit before updating..."; while kill -0 %d 2>/dev/null; do sleep 0.2; done; `, parentPID)
	if isGitRepo(repoDir) && pullFirst {
		inner += fmt.Sprintf(`GCM_INTERACTIVE=never git -C %s pull || exit $?; `, shSingleQuote(repoDir))
	}
	if _, err := os.Stat(filepath.Join(repoDir, "Makefile")); err == nil {
		inner += fmt.Sprintf(`make -C %s install; `, shSingleQuote(repoDir))
	} else {
		inner += fmt.Sprintf(`echo "Cannot update automatically: Makefile was not found at %s."; `, repoDir)
	}
	inner += fmt.Sprintf(`echo "Update process finished. Reopen crosspile."; echo "Log: %s"`, logPath)
	launcher := fmt.Sprintf(`nohup sh -c %s >> %s 2>&1 < /dev/null &`, shSingleQuote(inner), shSingleQuote(logPath))
	return exec.Command("sh", "-c", launcher)
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func psSingleQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func shSingleQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }

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

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.updatePrompt || m.reinstallPrompt || m.mode == modeOnboarding || m.mode == modeSearch {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X < m.listWidth() {
			m.move(-1)
		} else {
			m.scrollDetail(-1)
		}
	case tea.MouseButtonWheelDown:
		if msg.X < m.listWidth() {
			m.move(1)
		} else {
			m.scrollDetail(1)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		idx := m.offset + msg.Y - 3
		if idx >= 0 && idx < len(m.filtered) && msg.X < m.listWidth() {
			m.cursor = idx
			m.ensureCursorVisible()
		}
	}
	return m, nil
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

func (m Model) listWidth() int {
	listW := int(float64(m.width) * 0.40)
	if listW < 36 {
		listW = 36
	}
	if listW > 72 {
		listW = 72
	}
	if m.width-listW < 40 {
		listW = max(24, m.width-40)
	}
	return listW
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func locationsSignature(cfg *config.Config) string {
	var b strings.Builder
	for _, loc := range cfg.Locations {
		b.WriteString(loc.Name)
		b.WriteByte('=')
		b.WriteString(loc.Path)
		b.WriteByte(';')
	}
	return b.String()
}

func agentsSignature(cfg *config.Config) string {
	return fmt.Sprintf("%t:%t:%t", cfg.Agents.OpenCode, cfg.Agents.Claude, cfg.Agents.Generic)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
