package model

import (
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	ID        string
	Title     string
	Agent     string
	Mode      string
	Model     string
	Provider  string
	Project   string
	Directory string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []Message
	Todos     []Todo
	Files     []string
	Tools     []string
	Cost      float64
	TokensIn  int64
	TokensOut int64
	Source    string
	Warnings  []string
}

type Message struct {
	ID        string
	Role      string
	CreatedAt time.Time
	Parts     []Part
}

type Part struct {
	Type string
	Text string
	Meta string
}

type Todo struct {
	Content  string
	Status   string
	Priority string
}

func (s Session) PromptText() string {
	return s.roleText("user")
}

func (s Session) ResponseText() string {
	return s.roleText("assistant")
}

func (s Session) AllText() string {
	var b strings.Builder
	b.WriteString(s.Title)
	b.WriteString(" ")
	b.WriteString(s.ID)
	b.WriteString(" ")
	b.WriteString(s.Agent)
	b.WriteString(" ")
	b.WriteString(s.Model)
	b.WriteString(" ")
	b.WriteString(s.Provider)
	b.WriteString(" ")
	b.WriteString(s.Project)
	b.WriteString(" ")
	b.WriteString(s.Directory)
	for _, msg := range s.Messages {
		b.WriteString(" ")
		b.WriteString(msg.Role)
		for _, p := range msg.Parts {
			b.WriteString(" ")
			b.WriteString(p.Type)
			b.WriteString(" ")
			b.WriteString(p.Text)
			b.WriteString(" ")
			b.WriteString(p.Meta)
		}
	}
	for _, t := range s.Tools {
		b.WriteString(" ")
		b.WriteString(t)
	}
	for _, f := range s.Files {
		b.WriteString(" ")
		b.WriteString(f)
	}
	return b.String()
}

func (s Session) Preview(max int) string {
	if max <= 0 {
		max = 240
	}
	text := strings.TrimSpace(s.PromptText())
	if text == "" {
		text = strings.TrimSpace(s.ResponseText())
	}
	text = oneLine(text)
	if len(text) > max {
		return text[:max-1] + "..."
	}
	return text
}

func (s Session) ProjectName() string {
	if s.Project != "" {
		return s.Project
	}
	if s.Directory != "" {
		return filepath.Base(s.Directory)
	}
	return "unknown"
}

func (s Session) roleText(role string) string {
	var b strings.Builder
	for _, msg := range s.Messages {
		if msg.Role != role {
			continue
		}
		for _, p := range msg.Parts {
			if p.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
