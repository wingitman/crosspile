package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wingitman/crosspile/internal/analytics"
	"github.com/wingitman/crosspile/internal/model"
	"github.com/wingitman/crosspile/internal/search"
	"github.com/wingitman/crosspile/internal/ui"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	if m.updatePrompt || m.reinstallPrompt {
		b.WriteString(m.renderUpdatePopup())
		return b.String()
	}
	switch m.mode {
	case modeOnboarding:
		b.WriteString(m.renderOnboarding())
	case modeLoading:
		b.WriteString("\n" + ui.StyleMuted.Render("  scanning configured work locations...") + "\n")
	case modeError:
		b.WriteString("\n" + ui.StyleError.Render("  "+m.err) + "\n")
	case modeFilterHelp:
		b.WriteString(m.renderFilterHelp())
	case modeAnalytics:
		b.WriteString(m.renderAnalytics())
	case modeRawData:
		b.WriteString(m.renderRawView())
	default:
		b.WriteString(m.renderMain())
	}
	return b.String()
}

func (m Model) renderHeader() string {
	left := ui.StylePrimary.Render("crosspile")
	if m.scanning {
		left += ui.StyleMuted.Render("  scanning")
	} else if len(m.sessions) > 0 {
		left += ui.StyleMuted.Render(fmt.Sprintf("  %d sessions", len(m.sessions)))
	}
	if m.filter != "" {
		left += " " + ui.StyleAccent.Render("[filter]")
	}
	delby := lipgloss.NewStyle().Foreground(ui.ColorBrand1).Bold(true).Render("delby")
	soft := lipgloss.NewStyle().Foreground(ui.ColorBrand2).Bold(true).Render("soft")
	brand := " " + delby + soft + " "
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(brand)
	if pad < 1 {
		pad = 1
	}
	rule := ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width)))
	return left + strings.Repeat(" ", pad) + brand + "\n" + rule
}

func (m Model) renderOnboarding() string {
	path, _ := filepath.Abs(".")
	lines := []string{
		"",
		"  " + ui.StylePrimary.Render("Choose Work Locations"),
		"",
		"  crosspile indexes AI-agent sessions for the projects you work in.",
		"  Enter one or more roots separated by commas or new lines.",
		"",
		"  Example: " + ui.StyleMuted.Render(locationExample()),
		"  Current directory: " + ui.StyleMuted.Render(path),
		"",
		"  " + ui.StyleInputPrompt.Render("locations") + " " + m.locInput.View(),
		"",
		"  " + renderKey(m.keys.confirm) + " save and scan   " + renderKey(m.keys.quit) + " quit",
	}
	if m.status != "" {
		lines = append(lines, "", "  "+ui.StyleError.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderMain() string {
	if m.mode == modeSearch {
		return m.renderSearchMain()
	}
	return m.renderSessionLayout(false)
}

func (m Model) renderSearchMain() string {
	return m.renderSessionLayout(true)
}

func (m Model) renderSessionLayout(searching bool) string {
	listW := m.listWidth()
	detailW := m.width - listW - 1
	contentH := m.listHeight()
	left := m.renderList(listW, contentH)
	right := m.renderDetail(detailW, contentH)
	var b strings.Builder
	for i := 0; i < contentH; i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		b.WriteString(padRight(l, listW))
		b.WriteString(ui.StyleMuted.Render("│"))
		b.WriteString(truncate(r, detailW))
		b.WriteString("\n")
	}
	b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
	if searching {
		b.WriteString(ui.StyleInputPrompt.Render(m.keys.search) + " " + m.filterInput.View() + "\n")
		if summary := searchSummary(m.filter); summary != "" {
			b.WriteString("  " + ui.StyleMuted.Render("Parsed: "+summary) + "\n")
		}
	}
	b.WriteString(m.renderStatus())
	return b.String()
}

func (m Model) renderList(width, height int) []string {
	var lines []string
	title := fmt.Sprintf("  Sessions (%d/%d)", len(m.filtered), len(m.sessions))
	lines = append(lines, ui.StylePrimary.Render(title))
	if len(m.filtered) == 0 {
		lines = append(lines, ui.StyleMuted.Render("  no matching sessions"))
		if summary := searchSummary(m.filter); summary != "" {
			lines = append(lines, ui.StyleMuted.Render("  parsed: "+summary))
		}
		return padLines(lines, height)
	}
	start := m.offset
	end := min(len(m.filtered), start+height-1)
	for i := start; i < end; i++ {
		s := m.filtered[i]
		cursor := "  "
		style := ui.StyleNormal
		if i == m.cursor {
			cursor = "▶ "
			style = ui.StyleSelected
		}
		date := shortTime(s.UpdatedAt)
		label := fmt.Sprintf("%s%-9s %-10s %s", cursor, s.Agent, s.ProjectName(), truncate(s.Title, width-27))
		line := style.Render(truncate(label, width-10)) + " " + ui.StyleMuted.Render(date)
		lines = append(lines, line)
	}
	return padLines(lines, height)
}

func (m Model) renderDetail(width, height int) []string {
	all := m.renderDetailLines(width)
	maxScroll := len(all) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	start := m.detailScroll
	end := min(len(all), start+height)
	return padLines(all[start:end], height)
}

func (m Model) renderDetailLines(width int) []string {
	s := m.selected()
	if s == nil {
		return []string{ui.StyleMuted.Render("  Select a session")}
	}
	var lines []string
	lines = append(lines, ui.StylePrimary.Render("  "+truncate(s.Title, width-2)))
	lines = append(lines, "  "+ui.StyleMuted.Render(fmt.Sprintf("%s  %s  %s", s.Agent, s.ID, formatTime(s.UpdatedAt))))
	meta := []string{s.ProjectName()}
	if s.Mode != "" {
		meta = append(meta, "mode:"+s.Mode)
	}
	if s.Model != "" {
		meta = append(meta, "model:"+s.Model)
	}
	if s.Provider != "" {
		meta = append(meta, "provider:"+s.Provider)
	}
	lines = append(lines, "  "+ui.StyleMuted.Render(strings.Join(meta, "  ")))
	lines = append(lines, "  "+ui.StyleMuted.Render(s.Directory))
	if len(s.Tools) > 0 {
		lines = append(lines, "  "+ui.StyleAccent.Render("tools ")+ui.StyleMuted.Render(strings.Join(limitStrings(s.Tools, 6), ", ")))
	}
	if s.Cost > 0 || s.TokensIn > 0 || s.TokensOut > 0 {
		lines = append(lines, "  "+ui.StyleMuted.Render(fmt.Sprintf("cost $%.4f  tokens:%d in:%d out:%d reasoning:%d", s.Cost, s.TotalTokens(), s.TokensIn, s.TokensOut, s.TokensReasoning)))
	}
	lines = append(lines, "")
	lines = appendTranscript(lines, *s, width, 1<<30)
	return lines
}

func appendTranscript(lines []string, s model.Session, width, remaining int) []string {
	for _, msg := range s.Messages {
		if remaining <= 0 {
			break
		}
		role := msg.Role
		style := ui.StyleMuted
		if role == "user" {
			style = ui.StyleAccent
		} else if role == "assistant" {
			style = ui.StylePrimary
		}
		lines = append(lines, "  "+style.Render(strings.ToUpper(role)))
		remaining--
		for _, p := range msg.Parts {
			if remaining <= 0 {
				break
			}
			text := strings.TrimSpace(p.Text)
			if text == "" && p.Meta != "" {
				text = p.Meta
			}
			for _, line := range wrapText(text, width-4) {
				if remaining <= 0 {
					break
				}
				lines = append(lines, "  "+line)
				remaining--
			}
		}
		if remaining > 0 {
			lines = append(lines, "")
			remaining--
		}
	}
	return lines
}

func (m Model) renderStatus() string {
	parts := m.contextHints()
	if m.lastScan.IsZero() {
		parts = append(parts, ui.StyleMuted.Render("not scanned"))
	} else {
		parts = append(parts, ui.StyleMuted.Render("scanned "+m.lastScan.Format("15:04:05")))
	}
	if len(m.warnings) > 0 {
		parts = append(parts, ui.StyleError.Render(fmt.Sprintf("%d warning(s)", len(m.warnings))))
	}
	if m.status != "" {
		parts = append(parts, ui.StyleMuted.Render(m.status))
	}
	out := " " + strings.Join(parts, ui.StyleMuted.Render("  "))
	if warnings := m.renderWarnings(); warnings != "" {
		out += "\n" + warnings
	}
	return out
}

func (m Model) renderWarnings() string {
	if len(m.warnings) == 0 {
		return ""
	}
	lines := []string{" " + ui.StyleError.Render("warnings:")}
	limit := min(len(m.warnings), 3)
	for _, warning := range m.warnings[:limit] {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			warning = "unknown warning"
		}
		lines = append(lines, "  "+ui.StyleMuted.Render(truncate(warning, max(10, m.width-4))))
	}
	if len(m.warnings) > limit {
		lines = append(lines, "  "+ui.StyleMuted.Render(fmt.Sprintf("+%d more warning(s)", len(m.warnings)-limit)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) contextHints() []string {
	switch m.mode {
	case modeSearch:
		return []string{
			renderKey(m.keys.confirm) + " apply",
			renderKey(m.keys.back) + " back",
			renderKey(m.keys.filterHelp) + " help",
			ui.StyleMuted.Render("from:Mar04 to:May17 agent: model: tokens:>10000"),
		}
	case modeFilterHelp:
		return []string{renderKey(m.keys.back) + " back", renderKey(m.keys.search) + " filter", renderKey(m.keys.quit) + " quit"}
	case modeAnalytics:
		return []string{renderKey(m.keys.back) + " back", renderKey(m.keys.analyticsFocus) + " fields/metrics", renderKey(m.keys.analyticsView) + " view", renderKey(m.keys.analyticsBucket) + " bucket", renderKey(m.keys.up) + "/" + renderKey(m.keys.down) + " select", renderKey(m.keys.confirm) + " toggle metric"}
	case modeRawData:
		return []string{renderKey(m.keys.back) + " back", renderKey(m.keys.exportCSV) + " csv", renderKey(m.keys.exportJSON) + " json", renderKey(m.keys.confirm) + " open row", renderKey(m.keys.openDocument) + " edit cell", renderKey(m.keys.rawNextTable) + " table", renderKey(m.keys.pageUp) + "/" + renderKey(m.keys.pageDown) + " page"}
	case modeLoading:
		return []string{renderKey(m.keys.quit) + " quit", ui.StyleMuted.Render("scanning")}
	case modeError:
		return []string{renderKey(m.keys.reload) + " retry", renderKey(m.keys.openConfig) + " config", renderKey(m.keys.quit) + " quit"}
	default:
		return []string{
			renderKey(m.keys.up) + "/" + renderKey(m.keys.down) + " move",
			renderKey(m.keys.search) + " filter",
			renderKey(m.keys.filterHelp) + " help",
			renderKey(m.keys.analytics) + " analytics",
			renderKey(m.keys.reload) + " reload",
			renderKey(m.keys.openDocument) + " open",
			renderKey(m.keys.rawView) + " raw",
			renderKey(m.keys.detailUp) + "/" + renderKey(m.keys.detailDown) + " doc",
			renderKey(m.keys.openConfig) + " config",
			renderKey(m.keys.checkUpdate) + " update",
			renderKey(m.keys.quit) + " quit",
		}
	}
}

func (m Model) renderUpdatePopup() string {
	title := "Update Available"
	if m.reinstallPrompt {
		title = "Reinstall Required"
	}
	message := m.updateMessage
	if message == "" {
		message = "A newer version is available. Pull latest changes and reinstall now?"
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.StyleAccent.Render("  " + title))
	b.WriteString("\n\n")
	b.WriteString("  " + message + "\n")
	if m.updateLatest != "" {
		b.WriteString("  " + ui.StyleMuted.Render("Latest: "+m.updateLatest) + "\n")
	}
	changes := compactStrings(m.updateChanges)
	if len(changes) > 0 {
		b.WriteString("\n  " + ui.StylePrimary.Render("Recent changes:") + "\n")
		for _, change := range changes {
			b.WriteString("  " + ui.StyleMuted.Render("- "+change) + "\n")
		}
	}
	noStyle := ui.StyleNormal
	yesStyle := ui.StyleNormal
	if m.updateChoice == 0 {
		noStyle = ui.StyleSelected
	} else {
		yesStyle = ui.StyleSelected
	}
	b.WriteString("\n  ")
	b.WriteString(noStyle.Render("  No  "))
	b.WriteString("   ")
	b.WriteString(yesStyle.Render("  Yes  "))
	b.WriteString("\n\n  ")
	b.WriteString(renderKey(m.keys.left) + "/" + renderKey(m.keys.right) + " choose  ")
	b.WriteString(renderKey(m.keys.confirm) + " confirm  ")
	b.WriteString(renderKey(m.keys.back) + " cancel")
	return ui.StyleBorder.Width(min(76, max(40, m.width-6))).Render(b.String())
}

func renderKey(k string) string {
	return ui.StyleStatusKey.Render("[" + k + "]")
}

func (m Model) renderFilterHelp() string {
	lines := []string{
		"",
		"  " + ui.StylePrimary.Render("Filter Help"),
		"",
		"  Dates are inclusive. Month/day uses the current year.",
		"  from:Mar04           updated on or after Mar 04",
		"  to:2026-03-31        updated on or before Mar 31",
		"  date:Mar04           exact day",
		"  updated:Mar04..May17 updated range",
		"  created:7d           created in the last 7 days",
		"",
		"  Metadata filters can be repeated. Repeats are OR; different fields are AND.",
		"  agent:opencode       agent or mode includes opencode",
		"  project:crosspile    project/directory includes crosspile",
		"  location:Work        configured location name/path",
		"  model:gpt provider:openai mode:build context:/Work",
		"  tool:bash skill:omarchy file:main.go source:sqlite",
		"",
		"  Text and numeric filters:",
		"  q:\"build a TUI\"      user prompt text",
		"  a:error             assistant response text",
		"  tokens:>10000      total tokens",
		"  cost:<0.25         total cost",
		"  -agent:claude      exclude matches",
		"",
		"  Example:",
		"  " + ui.StyleMuted.Render(`from:Mar04 to:May17 agent:opencode project:crosspile model:gpt tokens:>10000 q:"raw db"`),
		"",
		"  " + renderKey(m.keys.back) + " back",
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderAnalytics() string {
	if len(m.analyticsMetrics) == 0 {
		m.analyticsMetrics = analytics.DefaultSelectedMetrics()
	}
	dimension := analytics.Dimensions[m.analyticsDimension]
	bucket := analytics.Buckets[m.analyticsBucket]
	rows := analytics.Build(m.filtered, dimension, bucket)
	total := analytics.Totals(m.filtered)
	selected := selectedAnalyticsMetrics(m.analyticsMetrics)
	var b strings.Builder
	b.WriteString("\n  " + ui.StylePrimary.Render("Analytics") + ui.StyleMuted.Render(fmt.Sprintf("  %d filtered sessions  dimension:%s  bucket:%s  view:%s", len(m.filtered), dimension, bucket, analyticsViewName(m.analyticsView))) + "\n")
	if m.filter != "" {
		b.WriteString("  " + ui.StyleMuted.Render("filter: "+m.filter) + "\n")
		if summary := searchSummary(m.filter); summary != "" {
			b.WriteString("  " + ui.StyleMuted.Render("parsed: "+summary) + "\n")
		}
	}
	b.WriteString("\n")
	if m.analyticsView != analyticsViewTable {
		b.WriteString(m.renderAnalyticsGraph(rows, selected))
		b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
		b.WriteString("  " + ui.StylePrimary.Render("Total") + "  ")
		for _, metric := range selected {
			b.WriteString(metric.String() + ":" + analytics.Value(total, metric) + "  ")
		}
		b.WriteString("\n")
		b.WriteString(m.renderStatus())
		return b.String()
	}
	metricW := 18
	b.WriteString("  " + ui.StyleAccent.Render("Group By") + "\n")
	for i, dimensionOption := range analytics.Dimensions {
		cursor := "  "
		style := ui.StyleNormal
		if i == m.analyticsDimension {
			cursor = "● "
		}
		if m.analyticsFocus == 0 && i == m.analyticsDimension {
			style = ui.StyleSelected
		}
		b.WriteString("  " + style.Render(cursor+dimensionOption.String()) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + ui.StyleAccent.Render("Metrics") + "\n")
	for i, metric := range analytics.Metrics {
		cursor := "  "
		style := ui.StyleNormal
		if m.analyticsFocus == 1 && i == m.analyticsMetricCursor {
			cursor = "▶ "
			style = ui.StyleSelected
		}
		check := "[ ]"
		if i < len(m.analyticsMetrics) && m.analyticsMetrics[i] {
			check = "[x]"
		}
		b.WriteString("  " + style.Render(cursor+check+" "+metric.String()) + "\n")
	}
	b.WriteString("\n")
	keyW := max(16, m.width-len(selected)*metricW-4)
	if keyW > 34 {
		keyW = 34
	}
	b.WriteString("  " + ui.StyleAccent.Render(padRight(dimension.String(), keyW)))
	for _, metric := range selected {
		b.WriteString(ui.StyleAccent.Render(padRight(metric.String(), metricW)))
	}
	b.WriteString("\n")
	limit := min(len(rows), max(4, m.height-22))
	for i := 0; i < limit; i++ {
		row := rows[i]
		b.WriteString("  " + padRight(truncate(row.Key, keyW-1), keyW))
		for _, metric := range selected {
			b.WriteString(padRight(analytics.Value(row, metric), metricW))
		}
		b.WriteString("\n")
	}
	b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
	b.WriteString("  " + ui.StylePrimary.Render("Total") + "  ")
	for _, metric := range selected {
		b.WriteString(metric.String() + ":" + analytics.Value(total, metric) + "  ")
	}
	b.WriteString("\n")
	b.WriteString(m.renderStatus())
	return b.String()
}

const (
	analyticsViewTable = iota
	analyticsViewBar
	analyticsViewTimeline
	analyticsViewSparkline
	analyticsViewCount
)

func analyticsViewName(v int) string {
	switch v {
	case analyticsViewTable:
		return "table"
	case analyticsViewBar:
		return "bar"
	case analyticsViewTimeline:
		return "timeline"
	case analyticsViewSparkline:
		return "sparkline"
	default:
		return "unknown"
	}
}

func (m Model) renderAnalyticsGraph(rows []analytics.Row, selected []analytics.Metric) string {
	metric := analytics.Metrics[m.analyticsMetricCursor]
	if len(selected) > 0 && m.analyticsMetricCursor >= len(analytics.Metrics) {
		metric = selected[0]
	}
	switch m.analyticsView {
	case analyticsViewSparkline:
		return renderSparkline(rows, metric, m.width)
	case analyticsViewTimeline:
		return renderBars(rows, metric, m.width, true)
	default:
		return renderBars(rows, metric, m.width, false)
	}
}

func renderBars(rows []analytics.Row, metric analytics.Metric, width int, forceTimeline bool) string {
	var b strings.Builder
	limit := min(len(rows), 16)
	labelW := 24
	barW := max(8, width-labelW-22)
	maxVal := 0.0
	for i := 0; i < limit; i++ {
		if v := metricNumeric(rows[i], metric); v > maxVal {
			maxVal = v
		}
	}
	b.WriteString("  " + ui.StyleAccent.Render(metric.String()) + "\n")
	for i := 0; i < limit; i++ {
		row := rows[i]
		v := metricNumeric(row, metric)
		filled := 0
		if maxVal > 0 {
			filled = int((v / maxVal) * float64(barW))
		}
		if filled < 0 {
			filled = 0
		}
		label := row.Key
		if forceTimeline && label == "all" {
			label = "timeline"
		}
		b.WriteString("  " + padRight(truncate(label, labelW-1), labelW))
		b.WriteString(ui.StylePrimary.Render(strings.Repeat("█", filled)))
		b.WriteString(ui.StyleMuted.Render(strings.Repeat("░", max(0, barW-filled))))
		b.WriteString("  " + analytics.Value(row, metric) + "\n")
	}
	return b.String()
}

func renderSparkline(rows []analytics.Row, metric analytics.Metric, width int) string {
	chars := []rune("▁▂▃▄▅▆▇█")
	limit := min(len(rows), max(1, width-8))
	maxVal := 0.0
	vals := make([]float64, 0, limit)
	for i := 0; i < limit; i++ {
		v := metricNumeric(rows[i], metric)
		vals = append(vals, v)
		if v > maxVal {
			maxVal = v
		}
	}
	var line strings.Builder
	for _, v := range vals {
		idx := 0
		if maxVal > 0 {
			idx = int((v / maxVal) * float64(len(chars)-1))
		}
		line.WriteRune(chars[idx])
	}
	return "  " + ui.StyleAccent.Render(metric.String()) + "\n  " + ui.StylePrimary.Render(line.String()) + "\n"
}

func metricNumeric(row analytics.Row, metric analytics.Metric) float64 {
	switch metric {
	case analytics.MetricSessions:
		return float64(row.Sessions)
	case analytics.MetricTokens:
		return float64(row.Tokens)
	case analytics.MetricInputTokens:
		return float64(row.InputTokens)
	case analytics.MetricOutputTokens:
		return float64(row.OutputTokens)
	case analytics.MetricReasoningTokens:
		return float64(row.ReasoningTokens)
	case analytics.MetricCost:
		return row.Cost
	case analytics.MetricCostPerToken:
		if row.Tokens == 0 {
			return 0
		}
		return row.Cost / float64(row.Tokens)
	case analytics.MetricTokensPerRequest:
		if row.Sessions == 0 {
			return 0
		}
		return float64(row.Tokens) / float64(row.Sessions)
	case analytics.MetricCostPerRequest:
		if row.Sessions == 0 {
			return 0
		}
		return row.Cost / float64(row.Sessions)
	default:
		return 0
	}
}

func selectedAnalyticsMetrics(selected []bool) []analytics.Metric {
	var out []analytics.Metric
	for i, metric := range analytics.Metrics {
		if i < len(selected) && selected[i] {
			out = append(out, metric)
		}
	}
	if len(out) == 0 {
		out = append(out, analytics.MetricSessions)
	}
	return out
}

func searchSummary(raw string) string {
	parts := search.Summary(raw)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 5 {
		parts = append(parts[:5], fmt.Sprintf("+%d", len(parts)-5))
	}
	return strings.Join(parts, "  ")
}

func (m Model) renderRawView() string {
	if len(m.raw.Tables) == 0 {
		return "\n" + ui.StyleMuted.Render("  no raw data loaded")
	}
	t := m.raw.Tables[m.rawTable]
	var b strings.Builder
	rangeStart := 0
	rangeEnd := 0
	if len(t.Rows) > 0 {
		rangeStart = t.Page*t.PageSize + 1
		rangeEnd = rangeStart + len(t.Rows) - 1
	}
	loading := ""
	if m.rawLoading {
		loading = "  loading"
	}
	b.WriteString("\n  " + ui.StylePrimary.Render("Raw Data") + ui.StyleMuted.Render(fmt.Sprintf("  %s rows %d-%d of %d  page %d/%d%s", t.Name, rangeStart, rangeEnd, t.Total, t.Page+1, t.rawPageCount(), loading)) + "\n")
	visibleCols := m.rawVisibleCols()
	colEnd := min(len(t.Columns), m.rawColOffset+visibleCols)
	cellW := max(12, (m.width-6)/max(1, colEnd-m.rawColOffset))
	b.WriteString("  ")
	for c := m.rawColOffset; c < colEnd; c++ {
		b.WriteString(ui.StyleAccent.Render(padRight(truncate(t.Columns[c], cellW-1), cellW)))
	}
	b.WriteString("\n")
	rowEnd := min(len(t.Rows), m.rawRowOffset+m.rawVisibleRows())
	for r := m.rawRowOffset; r < rowEnd; r++ {
		b.WriteString("  ")
		for c := m.rawColOffset; c < colEnd; c++ {
			cell := ""
			if c < len(t.Rows[r]) {
				cell = rawCellPreview(t.Rows[r][c])
			}
			style := ui.StyleNormal
			if r == m.rawRow && c == m.rawCol {
				style = ui.StyleSelected
			}
			b.WriteString(style.Render(padRight(truncate(cell, cellW-1), cellW)))
		}
		b.WriteString("\n")
	}
	b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
	selected := ""
	detailHint := ""
	if m.rawRow >= 0 && m.rawRow < len(t.Rows) && m.rawCol >= 0 && m.rawCol < len(t.Columns) && m.rawCol < len(t.Rows[m.rawRow]) {
		if m.rawRowOpen {
			selected = rawDetailValue(t.Rows[m.rawRow][m.rawCol])
		} else {
			selected = rawCellPreview(t.Rows[m.rawRow][m.rawCol])
			detailHint = "  " + ui.StyleMuted.Render("("+m.keys.confirm+" to open row, "+m.keys.openDocument+" to edit cell)")
		}
	}
	column := ""
	if m.rawCol >= 0 && m.rawCol < len(t.Columns) {
		column = t.Columns[m.rawCol]
	}
	b.WriteString("  " + ui.StylePrimary.Render(column) + "\n")
	wrapped := wrapText(selected, m.width-4)
	for _, line := range wrapped[:min(len(wrapped), 6)] {
		b.WriteString("  " + line + "\n")
	}
	if detailHint != "" {
		b.WriteString(detailHint + "\n")
	}
	b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
	b.WriteString(m.renderStatus())
	return b.String()
}

func rawDetailValue(s string) string {
	const maxDetail = 12 * 1024
	if len(s) <= maxDetail {
		return s
	}
	return s[:maxDetail] + fmt.Sprintf("\n... (%d bytes hidden in detail view)", len(s)-maxDetail)
}

func padLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		return lines[:height]
	}
	return lines
}

func padRight(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func wrapText(s string, width int) []string {
	if width <= 10 {
		width = 10
	}
	s = strings.ReplaceAll(s, "\r", "")
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			if line == "" {
				line = w
				continue
			}
			if lipgloss.Width(line)+1+lipgloss.Width(w) > width {
				out = append(out, line)
				line = w
			} else {
				line += " " + w
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func shortTime(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	return t.Format("Jan02")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown time"
	}
	return t.Format("2006-01-02 15:04")
}

func limitStrings(vals []string, n int) []string {
	if len(vals) <= n {
		return vals
	}
	out := append([]string(nil), vals[:n]...)
	out = append(out, fmt.Sprintf("+%d", len(vals)-n))
	return out
}

func compactStrings(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, val := range vals {
		val = strings.TrimSpace(val)
		if val != "" {
			out = append(out, val)
		}
	}
	return out
}

func locationExample() string {
	if runtime.GOOS == "windows" {
		return `%USERPROFILE%\Work, D:\Projects`
	}
	return "~/Work, ~/Projects"
}
