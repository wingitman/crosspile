package app

import (
	"fmt"
	"path/filepath"
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
	switch m.mode {
	case modeOnboarding:
		b.WriteString(m.renderOnboarding())
	case modeLoading:
		b.WriteString("\n" + ui.StyleMuted.Render("  scanning configured work locations...") + "\n")
	case modeError:
		b.WriteString("\n" + ui.StyleError.Render("  "+m.err) + "\n")
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
		"  Enter one or more roots separated by commas.",
		"",
		"  Example: " + ui.StyleMuted.Render("~/Work, ~/Projects"),
		"  Current directory: " + ui.StyleMuted.Render(path),
		"",
		"  " + ui.StyleInputPrompt.Render("locations") + " " + m.locInput.View(),
		"",
		"  " + renderKey("enter") + " save and scan   " + renderKey(m.keys.quit) + " quit",
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
	listW := 42
	if m.width < 100 {
		listW = m.width / 2
	}
	if listW < 30 {
		listW = 30
	}
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
		b.WriteString(ui.StyleInputPrompt.Render("/") + " " + m.filterInput.View() + "\n")
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
	s := m.selected()
	if s == nil {
		return padLines([]string{ui.StyleMuted.Render("  Select a session")}, height)
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
		lines = append(lines, "  "+ui.StyleMuted.Render(fmt.Sprintf("cost $%.4f  tokens in:%d out:%d", s.Cost, s.TokensIn, s.TokensOut)))
	}
	lines = append(lines, "")
	lines = appendTranscript(lines, *s, width, height-len(lines)-1)
	return padLines(lines, height)
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
	parts := []string{
		renderKey(m.keys.search) + " filter",
		renderKey(m.keys.reload) + " reload",
		renderKey(m.keys.openConfig) + " config",
		renderKey(m.keys.quit) + " quit",
	}
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

func renderKey(k string) string {
	return ui.StyleStatusKey.Render("[" + k + "]")
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
