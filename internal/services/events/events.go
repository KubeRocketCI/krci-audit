// Package events serves the general audit-events query: filter by the common lifted columns,
// paginated. Filtering by actor also serves a per-user activity view, so there is no separate
// endpoint. It is a plain parameterized SQL capability over the dry-run-excluded read
// view — no generic query engine, predicate registry, or DSL.
package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/store"
)

// querier is the subset of pgxpool.Pool the service needs.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Filter is the set of optional predicates over the lifted columns. Nil fields are omitted
// from the WHERE clause. Page/PerPage are expected to be pre-clamped by the caller.
type Filter struct {
	Actor     *string
	Operation *string
	APIGroup  *string
	Resource  *string
	Kind      *string
	Namespace *string
	Name      *string
	ObjectUID *string
	From      *time.Time
	To        *time.Time
	Page      int
	PerPage   int
}

// Service queries audit events over the audit_events_real view (dry-run excluded) via the
// least-privilege audit_reader connection.
type Service struct {
	db querier
}

// New creates a Service backed by the given querier (a pgxpool.Pool in production).
func New(db querier) *Service {
	return &Service{db: db}
}

// apiExcludedColumns lists lifted columns excluded from the API surface, in addition to the
// body columns (object/old_object/raw) models.LiftedColumns() already excludes.
var apiExcludedColumns = map[string]bool{"user_groups": true, "user_extra": true}

// selectCols is derived from models.LiftedColumns() (the single source of truth for the
// searchable column set) minus apiExcludedColumns, so it can never silently drift from the
// model the way a hand-typed parallel list could.
var selectCols = buildSelectCols()

func buildSelectCols() string {
	var cols []string
	for _, c := range models.LiftedColumns() {
		if !apiExcludedColumns[c] {
			cols = append(cols, c)
		}
	}
	return strings.Join(cols, ", ")
}

// buildWhere renders the parameterized WHERE clause (empty string when no predicates) and the
// ordered argument list. Every value is bound as a $N placeholder — no user input is
// interpolated into SQL.
func (f Filter) buildWhere() (string, []any) {
	var (
		conds []string
		args  []any
	)
	add := func(expr string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(expr, len(args)))
	}

	if f.Actor != nil {
		add("username = $%d", *f.Actor)
	}
	if f.Operation != nil {
		add("operation = $%d", *f.Operation)
	}
	if f.APIGroup != nil {
		add("api_group = $%d", *f.APIGroup)
	}
	if f.Resource != nil {
		add("resource = $%d", *f.Resource)
	}
	if f.Kind != nil {
		add("kind = $%d", *f.Kind)
	}
	if f.Namespace != nil {
		add("namespace = $%d", *f.Namespace)
	}
	if f.Name != nil {
		add("name = $%d", *f.Name)
	}
	if f.ObjectUID != nil {
		add("object_uid = $%d", *f.ObjectUID)
	}
	if f.From != nil {
		add("received_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("received_at < $%d", *f.To)
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// eventRow is scanned by column name (via models.AuditEvent's own "db" struct tags), so the
// Scan target list can never drift out of order from selectCols the way a hand-typed
// positional Scan call could.
type eventRow struct {
	models.AuditEvent
	TotalCount int `db:"total_count"`
}

// Query returns the matching events for the given filter (most recent first) plus the total
// count of matches (ignoring pagination), so callers can build a Pagination response. The
// count and the page are read from a single snapshot via count(*) OVER(), rather than two
// separate round trips, so they can't disagree under concurrent inserts.
func (s *Service) Query(ctx context.Context, f Filter) ([]models.AuditEvent, int, error) {
	where, args := f.buildWhere()

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	sql := fmt.Sprintf(
		`SELECT %s, count(*) OVER() AS total_count FROM %s%s ORDER BY received_at DESC LIMIT $%d OFFSET $%d`,
		selectCols, store.RealView, where, limitArg, offsetArg)
	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query events: %w", err)
	}

	rowsScanned, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[eventRow])
	if err != nil {
		return nil, 0, fmt.Errorf("scan events: %w", err)
	}

	var total int
	events := make([]models.AuditEvent, 0, len(rowsScanned))
	for _, r := range rowsScanned {
		events = append(events, r.AuditEvent)
		total = r.TotalCount
	}

	return events, total, nil
}
