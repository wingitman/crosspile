package analytics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

type Dimension int

const (
	DimensionTimeline Dimension = iota
	DimensionAgent
	DimensionModel
	DimensionProject
	DimensionSession
	DimensionLocation
	DimensionProvider
	DimensionMode
	DimensionTool
	DimensionSkill
)

var Dimensions = []Dimension{DimensionTimeline, DimensionAgent, DimensionModel, DimensionProject, DimensionSession, DimensionLocation, DimensionProvider, DimensionMode, DimensionTool, DimensionSkill}

type Bucket int

const (
	BucketDay Bucket = iota
	BucketWeek
	BucketMonth
	BucketAll
)

var Buckets = []Bucket{BucketDay, BucketWeek, BucketMonth, BucketAll}

type Metric int

const (
	MetricSessions Metric = iota
	MetricTokens
	MetricInputTokens
	MetricOutputTokens
	MetricReasoningTokens
	MetricCost
	MetricCostPerToken
	MetricTokensPerRequest
	MetricCostPerRequest
)

var Metrics = []Metric{MetricSessions, MetricTokens, MetricInputTokens, MetricOutputTokens, MetricReasoningTokens, MetricCost, MetricCostPerToken, MetricTokensPerRequest, MetricCostPerRequest}

type Row struct {
	Key             string
	Sessions        int
	Tokens          int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	Cost            float64
}

func (d Dimension) String() string {
	switch d {
	case DimensionTimeline:
		return "timeline"
	case DimensionAgent:
		return "agent"
	case DimensionModel:
		return "model"
	case DimensionProject:
		return "project"
	case DimensionSession:
		return "session"
	case DimensionLocation:
		return "location"
	case DimensionProvider:
		return "provider"
	case DimensionMode:
		return "mode"
	case DimensionTool:
		return "tool"
	case DimensionSkill:
		return "skill"
	default:
		return "unknown"
	}
}

func (b Bucket) String() string {
	switch b {
	case BucketDay:
		return "day"
	case BucketWeek:
		return "week"
	case BucketMonth:
		return "month"
	case BucketAll:
		return "all"
	default:
		return "unknown"
	}
}

func (m Metric) String() string {
	switch m {
	case MetricSessions:
		return "sessions"
	case MetricTokens:
		return "tokens"
	case MetricInputTokens:
		return "input"
	case MetricOutputTokens:
		return "output"
	case MetricReasoningTokens:
		return "reasoning"
	case MetricCost:
		return "cost"
	case MetricCostPerToken:
		return "cost/token"
	case MetricTokensPerRequest:
		return "tokens/request"
	case MetricCostPerRequest:
		return "cost/request"
	default:
		return "unknown"
	}
}

func DefaultSelectedMetrics() []bool {
	selected := make([]bool, len(Metrics))
	for i, m := range Metrics {
		switch m {
		case MetricSessions, MetricTokens, MetricCost:
			selected[i] = true
		}
	}
	return selected
}

func Build(sessions []model.Session, dimension Dimension, bucket Bucket) []Row {
	rows := map[string]*Row{}
	for _, s := range sessions {
		keys := keysFor(s, dimension, bucket)
		for _, key := range keys {
			if key == "" {
				key = "(none)"
			}
			r := rows[key]
			if r == nil {
				r = &Row{Key: key}
				rows[key] = r
			}
			r.Sessions++
			r.Tokens += s.TotalTokens()
			r.InputTokens += s.TokensIn
			r.OutputTokens += s.TokensOut
			r.ReasoningTokens += s.TokensReasoning
			r.Cost += s.Cost
		}
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if dimension == DimensionTimeline {
			return out[i].Key < out[j].Key
		}
		if out[i].Cost == out[j].Cost {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Cost > out[j].Cost
	})
	return out
}

func Totals(sessions []model.Session) Row {
	rows := Build(sessions, DimensionTimeline, BucketAll)
	if len(rows) == 0 {
		return Row{Key: "total"}
	}
	rows[0].Key = "total"
	return rows[0]
}

func Value(row Row, metric Metric) string {
	switch metric {
	case MetricSessions:
		return fmt.Sprintf("%d", row.Sessions)
	case MetricTokens:
		return fmt.Sprintf("%d", row.Tokens)
	case MetricInputTokens:
		return fmt.Sprintf("%d", row.InputTokens)
	case MetricOutputTokens:
		return fmt.Sprintf("%d", row.OutputTokens)
	case MetricReasoningTokens:
		return fmt.Sprintf("%d", row.ReasoningTokens)
	case MetricCost:
		return fmt.Sprintf("$%.4f", row.Cost)
	case MetricCostPerToken:
		if row.Tokens == 0 {
			return "$0"
		}
		return fmt.Sprintf("$%.8f", row.Cost/float64(row.Tokens))
	case MetricTokensPerRequest:
		if row.Sessions == 0 {
			return "0"
		}
		return fmt.Sprintf("%.1f", float64(row.Tokens)/float64(row.Sessions))
	case MetricCostPerRequest:
		if row.Sessions == 0 {
			return "$0"
		}
		return fmt.Sprintf("$%.4f", row.Cost/float64(row.Sessions))
	default:
		return ""
	}
}

func keysFor(s model.Session, d Dimension, b Bucket) []string {
	switch d {
	case DimensionTimeline:
		return []string{bucketKey(s.UpdatedAt, b)}
	case DimensionAgent:
		return []string{s.Agent}
	case DimensionModel:
		return []string{s.Model}
	case DimensionProject:
		return []string{s.ProjectName()}
	case DimensionSession:
		return []string{firstNonEmpty(s.Title, s.ID)}
	case DimensionLocation:
		return []string{firstNonEmpty(s.LocationName, s.LocationPath)}
	case DimensionProvider:
		return []string{s.Provider}
	case DimensionMode:
		return []string{s.Mode}
	case DimensionTool:
		if len(s.Tools) == 0 {
			return []string{"(none)"}
		}
		return s.Tools
	case DimensionSkill:
		if len(s.Skills) == 0 {
			return []string{"(none)"}
		}
		return s.Skills
	default:
		return []string{"unknown"}
	}
}

func bucketKey(t time.Time, b Bucket) string {
	if t.IsZero() {
		return "unknown"
	}
	switch b {
	case BucketDay:
		return t.Format("2006-01-02")
	case BucketWeek:
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	case BucketMonth:
		return t.Format("2006-01")
	case BucketAll:
		return "all"
	default:
		return t.Format("2006-01-02")
	}
}

func firstNonEmpty(vals ...string) string {
	for _, val := range vals {
		if strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}
