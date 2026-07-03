package initiator_test

// Fast, Docker-free unit tests for the initiator service's query construction and result
// mapping (found/not-found/error). The real query execution against Postgres is covered
// separately by the integration suite (initiator_test.go, `make test-integration`); this file
// exercises the same code paths against a fake querier/row, per the package's own doc comment
// ("so handlers/tests can substitute a fake without a live database").

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

// fakeRow implements pgx.Row (just Scan) so ByObjectUID/ByObject can be exercised without a
// database.
type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

// fakeQuerier implements the service's querier interface and records the SQL/args it received.
type fakeQuerier struct {
	row     fakeRow
	gotSQL  string
	gotArgs []any
}

func (q *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.gotSQL, q.gotArgs = sql, args
	return q.row
}

func scanValues(actor string, operation models.Operation, ts time.Time) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*string) = actor
		*dest[1].(*models.Operation) = operation
		*dest[2].(*time.Time) = ts
		return nil
	}
}

func TestByObjectUIDFound(t *testing.T) {
	ts := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	q := &fakeQuerier{row: fakeRow{scan: scanValues("dev@x", "CREATE", ts)}}
	svc := initiator.New(q)

	res, err := svc.ByObjectUID(context.Background(), "uid-1")
	require.NoError(t, err)
	require.True(t, res.Found)
	require.Equal(t, "dev@x", res.Actor)
	require.Equal(t, ts, res.Timestamp)
	require.Contains(t, q.gotSQL, "object_uid = $1")
	require.Equal(t, []any{"uid-1"}, q.gotArgs)
}

func TestByObjectFound(t *testing.T) {
	ts := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	q := &fakeQuerier{row: fakeRow{scan: scanValues("dev@x", "CREATE", ts)}}
	svc := initiator.New(q)

	res, err := svc.ByObject(context.Background(), "PipelineRun", "edp", "b1")
	require.NoError(t, err)
	require.True(t, res.Found)
	require.Contains(t, q.gotSQL, "kind = $1 AND namespace = $2 AND name = $3")
	require.Equal(t, []any{"PipelineRun", "edp", "b1"}, q.gotArgs)
}

func TestByObjectUIDNotFoundIsNotAnError(t *testing.T) {
	q := &fakeQuerier{row: fakeRow{scan: func(_ ...any) error { return pgx.ErrNoRows }}}
	svc := initiator.New(q)

	res, err := svc.ByObjectUID(context.Background(), "missing")
	require.NoError(t, err)
	require.False(t, res.Found)
	require.Zero(t, res.Actor)
}

func TestByObjectUIDScanError(t *testing.T) {
	boom := errors.New("connection reset")
	q := &fakeQuerier{row: fakeRow{scan: func(_ ...any) error { return boom }}}
	svc := initiator.New(q)

	_, err := svc.ByObjectUID(context.Background(), "uid-1")
	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}
