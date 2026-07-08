package facets

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// ParseField validates against the fixed whitelist and maps every valid Field to its
// underlying column, so no client-supplied string can ever become a SQL identifier.
func TestParseField(t *testing.T) {
	t.Run("valid fields map to whitelisted columns", func(t *testing.T) {
		cases := map[string]struct {
			field Field
			col   string
		}{
			"namespace": {FieldNamespace, "namespace"},
			"kind":      {FieldKind, "kind"},
			"actor":     {FieldActor, "username"},
		}
		for raw, want := range cases {
			f, err := ParseField(raw)
			require.NoError(t, err)
			require.Equal(t, want.field, f)
			require.Equal(t, want.col, fieldColumns[f])
		}
	})

	t.Run("unknown field is rejected", func(t *testing.T) {
		_, err := ParseField("username")
		require.Error(t, err, "username is the column, not the API field name (actor)")

		_, err = ParseField("'; DROP TABLE audit_events; --")
		require.Error(t, err)

		_, err = ParseField("")
		require.Error(t, err)
	})
}

func TestAllFields(t *testing.T) {
	require.Equal(t, []Field{FieldNamespace, FieldKind, FieldActor}, AllFields())
	for _, f := range AllFields() {
		_, ok := fieldColumns[f]
		require.True(t, ok, "every default field must be in the whitelist")
	}
}

// MaxValues is the contractual cap; pinning it guards against an accidental change silently
// altering the truncation threshold.
func TestMaxValues(t *testing.T) {
	require.Equal(t, 50, MaxValues)
}

// errQuerier fails every query, exercising the DB-error path without a real database (the
// integration test's healthy Postgres can't reach it).
type errQuerier struct{ err error }

func (q errQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, q.err
}

func TestQueryWrapsDBError(t *testing.T) {
	svc := New(errQuerier{err: errors.New("connection refused")})

	_, err := svc.Query(context.Background(), AllFields())
	require.ErrorContains(t, err, "connection refused")
}
