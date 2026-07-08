// Package facets serves distinct-value lookups over a small, fixed whitelist of columns
// (namespace, kind, actor) — the value sets used to populate filter dropdowns. It is
// read-only, dry-run excluded, and returns no counts. The column identifier used to build the
// SQL is always one of the whitelist constants below; a client-supplied field name is
// validated against the whitelist and never itself becomes part of the SQL text.
//
// A high-cardinality field is capped at MaxValues: instead of a misleading partial list, the
// response is Truncated with an empty Values list and the client falls back to free text.
package facets

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/KubeRocketCI/krci-audit/internal/store"
)

// MaxValues is the hard cap on distinct values returned per facet field; a field with more is
// reported as Truncated with an empty Values list rather than a misleading partial one.
const MaxValues = 50

// walkLimit is how far queryDistinct walks and how many rows it fetches: one past MaxValues, so
// a result set larger than the cap is detectable in a single query and the CTE walk stays
// bounded (never a full scan) on a high-cardinality column.
const walkLimit = MaxValues + 1

// querier is the subset of pgxpool.Pool the service needs.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Field is a whitelisted facet field name, as it appears in the API surface.
type Field string

// Supported Field values.
const (
	FieldNamespace Field = "namespace"
	FieldKind      Field = "kind"
	FieldActor     Field = "actor"
)

// AllFields is the default set returned when the caller requests no specific fields, in a
// stable, deterministic order.
func AllFields() []Field {
	return []Field{FieldNamespace, FieldKind, FieldActor}
}

// fieldColumns is the whitelist translating an API Field to its audit_events column — the only
// place a Field becomes a SQL identifier, so no caller-supplied string reaches SQL directly.
var fieldColumns = map[Field]string{
	FieldNamespace: "namespace",
	FieldKind:      "kind",
	FieldActor:     "username",
}

// ParseField validates a raw field name against the whitelist and returns the corresponding
// Field, or an error if it is not one of the supported facet fields.
func ParseField(raw string) (Field, error) {
	f := Field(raw)
	if _, ok := fieldColumns[f]; !ok {
		return "", fmt.Errorf("unknown facet field %q", raw)
	}
	return f, nil
}

// Service returns distinct facet values over the audit_events table (dry-run excluded) via
// the least-privilege audit_reader connection.
type Service struct {
	db querier
}

// New creates a Service backed by the given querier (a pgxpool.Pool in production).
func New(db querier) *Service {
	return &Service{db: db}
}

// FieldValues is one field's result: the sorted distinct values, or — when more than MaxValues
// exist — an empty Values list with Truncated true. A partial list is never returned.
type FieldValues struct {
	Values    []string
	Truncated bool
}

// Query returns the sorted, distinct, non-null values (capped at MaxValues) for each requested
// field. The column used in each field's query is looked up from the fixed fieldColumns
// whitelist — fields must already be validated via ParseField, so no unvalidated string ever
// reaches this method.
func (s *Service) Query(ctx context.Context, fields []Field) (map[Field]FieldValues, error) {
	result := make(map[Field]FieldValues, len(fields))

	for _, f := range fields {
		fv, err := s.queryDistinct(ctx, f)
		if err != nil {
			return nil, fmt.Errorf("query facet %q: %w", f, err)
		}
		result[f] = fv
	}

	return result, nil
}

// queryDistinct returns the sorted distinct non-null values of f's column via a loose index
// scan (recursive CTE over the column's partial index) — O(distinct * log N), not a full scan.
// col comes from the fieldColumns whitelist, so interpolating it into the SQL is injection-safe.
//
// It queries store.EventsTable with an inline dry_run = false (not store.RealView) because the
// loose scan needs the partial indexes, which the planner can only use against the base table.
//
// The CTE counter (n) and the outer LIMIT both stop at walkLimit, so len(values) > MaxValues
// means "more than the cap exists" (Truncated) — detected in one query, with a bounded walk.
func (s *Service) queryDistinct(ctx context.Context, f Field) (FieldValues, error) {
	col := fieldColumns[f]
	sql := fmt.Sprintf(`
		WITH RECURSIVE t AS (
			(SELECT %[1]s AS v, 1 AS n FROM %[2]s
			  WHERE dry_run = false AND %[1]s IS NOT NULL
			  ORDER BY %[1]s LIMIT 1)
		  UNION ALL
			SELECT (SELECT %[1]s FROM %[2]s
			         WHERE dry_run = false AND %[1]s > t.v AND %[1]s IS NOT NULL
			         ORDER BY %[1]s LIMIT 1), t.n + 1
			FROM t WHERE t.v IS NOT NULL AND t.n <= %[3]d
		)
		SELECT v FROM t WHERE v IS NOT NULL ORDER BY v LIMIT %[3]d`, col, store.EventsTable, walkLimit)

	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return FieldValues{}, fmt.Errorf("query distinct %s: %w", col, err)
	}

	values, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return FieldValues{}, fmt.Errorf("scan distinct %s: %w", col, err)
	}

	if len(values) > MaxValues {
		return FieldValues{Truncated: true}, nil
	}

	return FieldValues{Values: values}, nil
}
