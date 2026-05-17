package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/model"
)

func (m Model) openSelectedDocumentCmd() tea.Cmd {
	s := m.selected()
	if s == nil {
		return func() tea.Msg { return statusMsg("no AI document selected") }
	}
	path := s.Source
	if s.SourceKind == "sqlite" || path == "" {
		p, err := writeTranscriptTemp(*s)
		if err != nil {
			return func() tea.Msg { return errorMsg(err.Error()) }
		}
		path = p
	}
	editor := config.ResolveEditor(m.cfg.Apps.Editor)
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errorMsg(fmt.Sprintf("editor exited with error: %v", err))
		}
		return editorClosedMsg{}
	})
}

func writeTranscriptTemp(s model.Session) (string, error) {
	dir := filepath.Join(os.TempDir(), "crosspile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := safeFileName(s.ID)
	if name == "" {
		name = "session"
	}
	path := filepath.Join(dir, name+".md")
	return path, os.WriteFile(path, []byte(renderTranscriptMarkdown(s)), 0o644)
}

func renderTranscriptMarkdown(s model.Session) string {
	var b strings.Builder
	b.WriteString("# " + s.Title + "\n\n")
	meta := [][2]string{{"Session", s.ID}, {"Agent", s.Agent}, {"Mode", s.Mode}, {"Project", s.ProjectName()}, {"Location", s.LocationName}, {"Directory", s.Directory}, {"Model", s.Model}, {"Provider", s.Provider}, {"Source", s.Source}}
	for _, m := range meta {
		if m[1] != "" {
			b.WriteString("- **" + m[0] + ":** " + m[1] + "\n")
		}
	}
	if !s.CreatedAt.IsZero() {
		b.WriteString("- **Created:** " + s.CreatedAt.String() + "\n")
	}
	if !s.UpdatedAt.IsZero() {
		b.WriteString("- **Updated:** " + s.UpdatedAt.String() + "\n")
	}
	b.WriteString(fmt.Sprintf("- **Cost:** %.4f\n- **Tokens:** %d total, %d input, %d output, %d reasoning, %d cache read, %d cache write\n", s.Cost, s.TotalTokens(), s.TokensIn, s.TokensOut, s.TokensReasoning, s.TokensCacheRead, s.TokensCacheWrite))
	if len(s.Tools) > 0 {
		b.WriteString("- **Tools:** " + strings.Join(s.Tools, ", ") + "\n")
	}
	if len(s.Skills) > 0 {
		b.WriteString("- **Skills:** " + strings.Join(s.Skills, ", ") + "\n")
	}
	b.WriteString("\n---\n\n")
	for _, msg := range s.Messages {
		b.WriteString("## " + strings.ToUpper(msg.Role) + "\n\n")
		for _, p := range msg.Parts {
			if p.Text != "" {
				b.WriteString(p.Text + "\n\n")
			} else if p.Meta != "" {
				b.WriteString("`" + p.Meta + "`\n\n")
			}
		}
	}
	return b.String()
}

func safeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, s)
}
