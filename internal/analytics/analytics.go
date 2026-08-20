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

type Period = Bucket

const (
	PeriodDay   = BucketDay
	PeriodWeek  = BucketWeek
	PeriodMonth = BucketMonth
	PeriodAll   = BucketAll
)

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

// FieldRole describes how a field participates in a pivot.
type FieldRole int

const (
	RoleFilter FieldRole = iota
	RoleRow
	RoleColumn
	RoleValue
)

const (
	FieldRoleFilter = RoleFilter
	FieldRoleRow    = RoleRow
	FieldRoleColumn = RoleColumn
	FieldRoleValue  = RoleValue
)

// Field assigns a dimension or metric to a pivot role. Metrics are used only
// with RoleValue; dimensions are used with the other roles.
type Field struct {
	Dimension Dimension
	Metric    Metric
	Role      FieldRole
	Values    []string
}

type Filter struct {
	Dimension Dimension
	Values    []string
}

// PivotConfig controls the dimensions, metrics, and time grouping of a pivot.
// Fields is an optional equivalent to the explicit role slices. Explicit
// slices take precedence when supplied.
type PivotConfig struct {
	Filters     []Filter
	Rows        []Dimension
	Columns     []Dimension
	Values      []Metric
	Period      Bucket
	Granularity Bucket
	Fields      []Field
}

type Config = PivotConfig

type PivotHeader struct {
	Key    string
	Values []string
}

type Cell struct {
	Row
	Values map[Metric]float64
}

type PivotTable struct {
	Config        PivotConfig
	Rows          []PivotHeader
	Columns       []PivotHeader
	RowHeaders    []PivotHeader
	ColumnHeaders []PivotHeader
	Cells         [][]Cell
	RowTotals     []Row
	ColumnTotals  []Row
	GrandTotal    Row
}

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
			if out[i].Tokens == out[j].Tokens {
				return out[i].Key < out[j].Key
			}
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Cost > out[j].Cost
	})
	return out
}

// BuildPivot aggregates sessions across every combination of configured row
// and column dimensions. Tool and skill dimensions intentionally return one
// value per item, so a session may contribute to more than one cell.
func BuildPivot(sessions []model.Session, config PivotConfig) PivotTable {
	config = normalizeConfig(config)
	table := PivotTable{Config: config}

	rowKeys := map[string][]string{}
	columnKeys := map[string][]string{}
	rows := map[string]*Row{}
	columns := map[string]*Row{}
	cells := map[string]*Cell{}
	grand := Row{Key: "grand total"}

	for _, session := range sessions {
		if !matchesFilters(session, config) {
			continue
		}
		rowValues := dimensionCombinations(session, config.Rows, config.Period)
		columnValues := dimensionCombinations(session, config.Columns, config.Period)
		for _, row := range rowValues {
			for _, column := range columnValues {
				rowKey := joinKey(row)
				columnKey := joinKey(column)
				if _, ok := rowKeys[rowKey]; !ok {
					rowKeys[rowKey] = row
				}
				if _, ok := columnKeys[columnKey]; !ok {
					columnKeys[columnKey] = column
				}
				addSession(rows, rowKey, session)
				addSession(columns, columnKey, session)
				cellKey := rowKey + "\x1e" + columnKey
				if cells[cellKey] == nil {
					cells[cellKey] = &Cell{Row: Row{Key: cellKey}}
				}
				addToRow(&cells[cellKey].Row, session)
			}
		}
		addToRow(&grand, session)
	}

	table.Rows = sortedHeaders(rowKeys)
	table.Columns = sortedHeaders(columnKeys)
	table.RowHeaders = table.Rows
	table.ColumnHeaders = table.Columns
	table.Cells = make([][]Cell, len(table.Rows))
	for r, row := range table.Rows {
		table.Cells[r] = make([]Cell, len(table.Columns))
		for c, column := range table.Columns {
			key := row.Key + "\x1e" + column.Key
			if cell := cells[key]; cell != nil {
				table.Cells[r][c] = *cell
			} else {
				table.Cells[r][c] = Cell{Row: Row{Key: key}}
			}
			table.Cells[r][c].Values = metricValues(table.Cells[r][c].Row, config.Values)
		}
	}
	for _, header := range table.Rows {
		table.RowTotals = append(table.RowTotals, *rows[header.Key])
	}
	for _, header := range table.Columns {
		table.ColumnTotals = append(table.ColumnTotals, *columns[header.Key])
	}
	table.GrandTotal = grand
	for i := range table.RowTotals {
		table.RowTotals[i].Key = table.Rows[i].Key
	}
	for i := range table.ColumnTotals {
		table.ColumnTotals[i].Key = table.Columns[i].Key
	}
	return table
}

func normalizeConfig(config PivotConfig) PivotConfig {
	if len(config.Fields) > 0 {
		useFilters := len(config.Filters) == 0
		useRows := len(config.Rows) == 0
		useColumns := len(config.Columns) == 0
		useValues := len(config.Values) == 0
		for _, field := range config.Fields {
			switch field.Role {
			case RoleFilter:
				if useFilters {
					config.Filters = append(config.Filters, Filter{Dimension: field.Dimension, Values: field.Values})
				}
			case RoleRow:
				if useRows {
					config.Rows = append(config.Rows, field.Dimension)
				}
			case RoleColumn:
				if useColumns {
					config.Columns = append(config.Columns, field.Dimension)
				}
			case RoleValue:
				if useValues {
					config.Values = append(config.Values, field.Metric)
				}
			}
		}
	}
	if config.Granularity != BucketDay {
		config.Period = config.Granularity
	}
	return config
}

func matchesFilters(session model.Session, config PivotConfig) bool {
	for _, filter := range config.Filters {
		if len(filter.Values) == 0 {
			continue
		}
		values := dimensionValues(session, filter.Dimension, config.Period)
		matched := false
		for _, value := range values {
			for _, wanted := range filter.Values {
				if normalizeKey(value) == normalizeKey(wanted) {
					matched = true
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func dimensionCombinations(session model.Session, dimensions []Dimension, bucket Bucket) [][]string {
	combinations := [][]string{{}}
	for _, dimension := range dimensions {
		values := dimensionValues(session, dimension, bucket)
		next := make([][]string, 0, len(combinations)*len(values))
		for _, combination := range combinations {
			for _, value := range values {
				next = append(next, append(append([]string(nil), combination...), normalizeKey(value)))
			}
		}
		combinations = next
	}
	return combinations
}

func dimensionValues(session model.Session, dimension Dimension, bucket Bucket) []string {
	return keysFor(session, dimension, bucket)
}

func normalizeKey(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(none)"
	}
	return value
}

func joinKey(values []string) string { return strings.Join(values, "\x1f") }

func sortedHeaders(keys map[string][]string) []PivotHeader {
	out := make([]PivotHeader, 0, len(keys))
	for key, values := range keys {
		out = append(out, PivotHeader{Key: key, Values: append([]string(nil), values...)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func addSession(rows map[string]*Row, key string, session model.Session) {
	if rows[key] == nil {
		rows[key] = &Row{Key: key}
	}
	addToRow(rows[key], session)
}

func addToRow(row *Row, session model.Session) {
	row.Sessions++
	row.Tokens += session.TotalTokens()
	row.InputTokens += session.TokensIn
	row.OutputTokens += session.TokensOut
	row.ReasoningTokens += session.TokensReasoning
	row.Cost += session.Cost
}

func metricValues(row Row, metrics []Metric) map[Metric]float64 {
	values := make(map[Metric]float64, len(metrics))
	for _, metric := range metrics {
		values[metric] = NumericValue(row, metric)
	}
	return values
}

func NumericValue(row Row, metric Metric) float64 {
	switch metric {
	case MetricSessions:
		return float64(row.Sessions)
	case MetricTokens:
		return float64(row.Tokens)
	case MetricInputTokens:
		return float64(row.InputTokens)
	case MetricOutputTokens:
		return float64(row.OutputTokens)
	case MetricReasoningTokens:
		return float64(row.ReasoningTokens)
	case MetricCost:
		return row.Cost
	case MetricCostPerToken:
		if row.Tokens != 0 {
			return row.Cost / float64(row.Tokens)
		}
	case MetricTokensPerRequest:
		if row.Sessions != 0 {
			return float64(row.Tokens) / float64(row.Sessions)
		}
	case MetricCostPerRequest:
		if row.Sessions != 0 {
			return row.Cost / float64(row.Sessions)
		}
	}
	return 0
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
