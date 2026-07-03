// Package initiator answers "who created this Kubernetes object?" — the first, primary
// capability of the krci-audit read API. It runs one small parameterized query over the
// dry-run-excluded read view for the object's CREATE event and returns the acting username.
package initiator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/store"
)

// querier is the subset of pgxpool.Pool the service needs, so handlers/tests can substitute
// a fake without a live database.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Result is the outcome of an initiator lookup. Found is false (with zero other fields) when
// the object has no CREATE audit event, so a consumer can fall back to "unknown" — this is
// not treated as an error.
type Result struct {
	Found     bool
	Actor     string
	Operation models.Operation
	Timestamp time.Time
}

// Service resolves object creators over the audit_events_real view (dry-run previews already
// excluded) via the least-privilege audit_reader connection.
type Service struct {
	db querier
}

// New creates a Service backed by the given querier (a pgxpool.Pool in production).
func New(db querier) *Service {
	return &Service{db: db}
}

// The CREATE event is the initiator. object_uid is the stable correlation key (CREATE carries
// no resourceVersion). Ordered ASC + LIMIT 1 so a (theoretical) re-CREATE returns the first.
const initiatorQueryTemplate = `SELECT username, operation, received_at FROM %s
		 WHERE %s AND operation = '%s'
		 ORDER BY received_at ASC LIMIT 1`

var (
	byUIDQuery = fmt.Sprintf(initiatorQueryTemplate,
		store.RealView, "object_uid = $1", models.OperationCreate)

	byObjectQuery = fmt.Sprintf(initiatorQueryTemplate,
		store.RealView, "kind = $1 AND namespace = $2 AND name = $3", models.OperationCreate)
)

// ByObjectUID resolves the creator by the object's metadata.uid (→ object_uid).
func (s *Service) ByObjectUID(ctx context.Context, uid string) (Result, error) {
	return s.scan(s.db.QueryRow(ctx, byUIDQuery, uid))
}

// ByObject resolves the creator by kind + namespace + name.
func (s *Service) ByObject(ctx context.Context, kind, namespace, name string) (Result, error) {
	return s.scan(s.db.QueryRow(ctx, byObjectQuery, kind, namespace, name))
}

func (s *Service) scan(row pgx.Row) (Result, error) {
	var r Result
	if err := row.Scan(&r.Actor, &r.Operation, &r.Timestamp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{Found: false}, nil
		}
		return Result{}, fmt.Errorf("query initiator: %w", err)
	}
	r.Found = true
	return r, nil
}
