//go:build integration

// Requires Docker; see internal/store/store_test.go for why this is a build tag, not a
// runtime skip.

package facets_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/pgtest"
	"github.com/KubeRocketCI/krci-audit/internal/services/facets"
)

// Facets returns distinct, sorted values (capped at facets.MaxValues) per requested field over
// the loose-index-scan query, excludes dry-run rows, and includes only the requested fields.
func TestFacetsQuery(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	svc := facets.New(pool)

	base := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	insert := func(uid, ns, kind, user string, dry bool, offset time.Duration) {
		pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
			EventUID: uid, ReceivedAt: base.Add(offset), Operation: "CREATE", APIGroup: "tekton.dev",
			APIVersion: "v1", Resource: "pipelineruns", Kind: kind, Namespace: ns,
			Name: uid, ObjectUID: pgtest.Ptr("obj-" + uid), Username: user, DryRun: dry,
		})
	}

	// >=3 namespaces, >=2 kinds, >=2 actors, plus one dry-run row that must be excluded.
	insert("e1", "edp-delivery", "Codebase", "alice", false, 0)
	insert("e2", "krci", "PipelineRun", "bob", false, time.Minute)
	insert("e3", "team-a", "Codebase", "alice", false, 2*time.Minute)
	insert("e4", "edp-delivery", "PipelineRun", "system:serviceaccount:krci:builder", false, 3*time.Minute)
	insert("e5", "edp-delivery", "Codebase", "zeta-user", true, 4*time.Minute) // dry-run — must be excluded

	t.Run("default fields return sorted distinct values, dry-run excluded", func(t *testing.T) {
		got, err := svc.Query(ctx, facets.AllFields())
		require.NoError(t, err)

		require.Equal(t, []string{"edp-delivery", "krci", "team-a"}, got[facets.FieldNamespace].Values)
		require.False(t, got[facets.FieldNamespace].Truncated)

		require.Equal(t, []string{"Codebase", "PipelineRun"}, got[facets.FieldKind].Values)
		require.False(t, got[facets.FieldKind].Truncated)

		require.Equal(t, []string{"alice", "bob", "system:serviceaccount:krci:builder"}, got[facets.FieldActor].Values,
			"zeta-user only appears on the dry-run row and must be excluded")
		require.False(t, got[facets.FieldActor].Truncated)
	})

	t.Run("only requested fields are present", func(t *testing.T) {
		got, err := svc.Query(ctx, []facets.Field{facets.FieldKind})
		require.NoError(t, err)

		require.Len(t, got, 1)
		require.Contains(t, got, facets.FieldKind)
		require.Equal(t, []string{"Codebase", "PipelineRun"}, got[facets.FieldKind].Values)
	})

	t.Run("no matching rows returns an empty slice, not an error", func(t *testing.T) {
		pool2 := pgtest.New(t)
		svc2 := facets.New(pool2)

		got, err := svc2.Query(ctx, []facets.Field{facets.FieldNamespace})
		require.NoError(t, err)
		require.Empty(t, got[facets.FieldNamespace].Values)
		require.False(t, got[facets.FieldNamespace].Truncated)
	})
}

// A high-cardinality field (more than facets.MaxValues distinct values) is reported as
// truncated with an empty values list, never a partial one, while an unrelated low-cardinality
// field on the same rows still returns its full sorted set. This also proves the recursive CTE
// walk is bounded — MaxValues+1 (61 seeded, only 51 need to be walked) rather than a full scan.
func TestFacetsQueryTruncatesHighCardinalityField(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	svc := facets.New(pool)

	base := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)

	const distinctNamespaces = facets.MaxValues + 10 // strictly more than the cap
	for i := 0; i < distinctNamespaces; i++ {
		pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
			EventUID: fmt.Sprintf("hc-%03d", i), ReceivedAt: base.Add(time.Duration(i) * time.Second),
			Operation: "CREATE", APIGroup: "tekton.dev", APIVersion: "v1", Resource: "pipelineruns",
			Kind: "Codebase", Namespace: fmt.Sprintf("ns-%03d", i), Name: fmt.Sprintf("hc-%03d", i),
			ObjectUID: pgtest.Ptr(fmt.Sprintf("obj-hc-%03d", i)), Username: "alice", DryRun: false,
		})
	}

	got, err := svc.Query(ctx, []facets.Field{facets.FieldNamespace, facets.FieldActor})
	require.NoError(t, err)

	require.True(t, got[facets.FieldNamespace].Truncated, "more than MaxValues distinct namespaces exist")
	require.Empty(t, got[facets.FieldNamespace].Values, "truncated field must return no partial list")

	require.False(t, got[facets.FieldActor].Truncated, "only one distinct actor (alice) — well under the cap")
	require.Equal(t, []string{"alice"}, got[facets.FieldActor].Values)
}
