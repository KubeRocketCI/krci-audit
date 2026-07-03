package events

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/pgtest"
)

// selectCols must track models.LiftedColumns() minus the explicit API-excluded set, so a new
// lifted column added to the model is either surfaced here or explicitly denylisted — never
// silently dropped.
func TestSelectColsTracksLiftedColumns(t *testing.T) {
	got := strings.Split(selectCols, ", ")

	var want []string
	for _, c := range models.LiftedColumns() {
		if !apiExcludedColumns[c] {
			want = append(want, c)
		}
	}

	require.Equal(t, want, got)
}

// buildWhere binds every predicate as a positional $N placeholder in filter order, so no user
// input is interpolated into SQL. A nil field is omitted entirely.
func TestBuildWhere(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	t.Run("no predicates", func(t *testing.T) {
		where, args := Filter{}.buildWhere()
		require.Empty(t, where)
		require.Empty(t, args)
	})

	t.Run("actor + operation only", func(t *testing.T) {
		where, args := Filter{Actor: pgtest.Ptr("dev@x"), Operation: pgtest.Ptr("CREATE")}.buildWhere()
		require.Equal(t, " WHERE username = $1 AND operation = $2", where)
		require.Equal(t, []any{"dev@x", "CREATE"}, args)
	})

	t.Run("all predicates in order", func(t *testing.T) {
		f := Filter{
			Actor: pgtest.Ptr("u"), Operation: pgtest.Ptr("UPDATE"), APIGroup: pgtest.Ptr("tekton.dev"),
			Resource: pgtest.Ptr("pipelineruns"), Kind: pgtest.Ptr("PipelineRun"), Namespace: pgtest.Ptr("edp"),
			Name: pgtest.Ptr("b1"), ObjectUID: pgtest.Ptr("uid"), From: &from, To: &to,
		}
		where, args := f.buildWhere()
		require.Equal(t,
			" WHERE username = $1 AND operation = $2 AND api_group = $3 AND resource = $4"+
				" AND kind = $5 AND namespace = $6 AND name = $7 AND object_uid = $8"+
				" AND received_at >= $9 AND received_at < $10", where)
		require.Equal(t, []any{"u", "UPDATE", "tekton.dev", "pipelineruns", "PipelineRun", "edp", "b1", "uid", from, to}, args)
	})
}
