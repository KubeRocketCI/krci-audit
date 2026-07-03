package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/pgtest"
	"github.com/KubeRocketCI/krci-audit/internal/services/events"
)

// Events query filters by the lifted columns, excludes dry-run (queries audit_events_real),
// and paginates. Covers the events-query scenarios + the actor filter ("My Activity").
func TestEventsQuery(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	svc := events.New(pool)

	base := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	insert := func(uid, ns, user, op string, dry bool, offset time.Duration) {
		pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
			EventUID: uid, ReceivedAt: base.Add(offset), Operation: op, APIGroup: "tekton.dev",
			APIVersion: "v1", Resource: "pipelineruns", Kind: "PipelineRun", Namespace: ns,
			Name: uid, ObjectUID: pgtest.Ptr("obj-" + uid), Username: user, DryRun: dry,
		})
	}
	insert("e1", "edp", "alice", "CREATE", false, 0)
	insert("e2", "edp", "bob", "UPDATE", false, time.Minute)
	insert("e3", "team", "alice", "CREATE", false, 2*time.Minute)
	insert("e4", "edp", "alice", "CREATE", true, 3*time.Minute) // dry-run — must be excluded

	t.Run("filter by namespace", func(t *testing.T) {
		rows, total, err := svc.Query(ctx, events.Filter{Namespace: pgtest.Ptr("edp"), Page: 1, PerPage: 20})
		require.NoError(t, err)
		require.Equal(t, 2, total, "e1 + e2; e4 is dry-run (excluded)")
		require.Len(t, rows, 2)
	})

	t.Run("filter by actor serves activity view", func(t *testing.T) {
		rows, total, err := svc.Query(ctx, events.Filter{Actor: pgtest.Ptr("alice"), Page: 1, PerPage: 20})
		require.NoError(t, err)
		require.Equal(t, 2, total, "alice's real events (e1, e3); dry-run e4 excluded")
		require.Len(t, rows, 2)
		for _, r := range rows {
			require.Equal(t, "alice", r.Username)
		}
	})

	t.Run("most recent first", func(t *testing.T) {
		rows, _, err := svc.Query(ctx, events.Filter{Namespace: pgtest.Ptr("edp"), Page: 1, PerPage: 20})
		require.NoError(t, err)
		require.Equal(t, "e2", rows[0].EventUID, "ORDER BY received_at DESC")
	})

	t.Run("pagination bounds the page", func(t *testing.T) {
		rows, total, err := svc.Query(ctx, events.Filter{Page: 1, PerPage: 2})
		require.NoError(t, err)
		require.Equal(t, 3, total, "three real events total (dry-run excluded)")
		require.Len(t, rows, 2, "page size honoured")
	})

	t.Run("no match returns empty, not error", func(t *testing.T) {
		rows, total, err := svc.Query(ctx, events.Filter{Namespace: pgtest.Ptr("nope"), Page: 1, PerPage: 20})
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, rows)
	})
}
