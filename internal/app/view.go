package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wingitman/crosspile/internal/model"
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
	return " " + strings.Join(parts, ui.StyleMuted.Render("  "))
}

func (m Model) contextHints() []string {
	switch m.mode {
	case modeSearch:
		return []string{
			renderKey(m.keys.confirm) + " apply",
			renderKey(m.keys.back) + " back",
			ui.StyleMuted.Render("agent: project: sid: model: from: to: q: a:"),
		}
	case modeRawData:
		return []string{renderKey(m.keys.back) + " back", renderKey(m.keys.rawNextTable) + " table", renderKey(m.keys.up) + "/" + renderKey(m.keys.down) + " rows", renderKey(m.keys.left) + "/" + renderKey(m.keys.right) + " cells"}
	case modeLoading:
		return []string{renderKey(m.keys.quit) + " quit", ui.StyleMuted.Render("scanning")}
	case modeError:
		return []string{renderKey(m.keys.reload) + " retry", renderKey(m.keys.openConfig) + " config", renderKey(m.keys.quit) + " quit"}
	default:
		return []string{
			renderKey(m.keys.up) + "/" + renderKey(m.keys.down) + " move",
			renderKey(m.keys.search) + " filter",
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
	noStyle := ui.StyleBorder
	yesStyle := ui.StyleBorder
	if m.updateChoice == 0 {
		noStyle = ui.StyleSelected
	} else {
		yesStyle = ui.StyleSelected
	}
	b.WriteString("\n  ")
	b.WriteString(noStyle.Render(" No "))
	b.WriteString("   ")
	b.WriteString(yesStyle.Render(" Yes "))
	b.WriteString("\n\n  ")
	b.WriteString(renderKey(m.keys.left) + "/" + renderKey(m.keys.right) + " choose  ")
	b.WriteString(renderKey(m.keys.confirm) + " confirm  ")
	b.WriteString(renderKey(m.keys.back) + " cancel")
	return ui.StyleBorder.Width(min(76, max(40, m.width-6))).Render(b.String())
}

func renderKey(k string) string {
	return ui.StyleStatusKey.Render("[" + k + "]")
}

func (m Model) renderRawView() string {
	if len(m.raw.Tables) == 0 {
		return "\n" + ui.StyleMuted.Render("  no raw data loaded")
	}
	t := m.raw.Tables[m.rawTable]
	var b strings.Builder
	b.WriteString("\n  " + ui.StylePrimary.Render("Raw Data") + ui.StyleMuted.Render(fmt.Sprintf("  %s (%d rows)", t.Name, len(t.Rows))) + "\n")
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
				cell = strings.ReplaceAll(t.Rows[r][c], "\n", " ")
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
	if m.rawRow >= 0 && m.rawRow < len(t.Rows) && m.rawCol >= 0 && m.rawCol < len(t.Columns) && m.rawCol < len(t.Rows[m.rawRow]) {
		selected = t.Rows[m.rawRow][m.rawCol]
	}
	b.WriteString("  " + ui.StylePrimary.Render(t.Columns[m.rawCol]) + "\n")
	wrapped := wrapText(selected, m.width-4)
	for _, line := range wrapped[:min(len(wrapped), 6)] {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString(ui.StyleMuted.Render(strings.Repeat("─", max(1, m.width))) + "\n")
	b.WriteString(m.renderStatus())
	return b.String()
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
