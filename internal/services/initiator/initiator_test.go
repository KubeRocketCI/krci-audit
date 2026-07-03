//go:build integration

// Requires Docker; see internal/store/store_test.go for why this is a build tag, not a
// runtime skip.

package initiator_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/pgtest"
	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

// Initiator lookup returns the CREATE actor by object UID and by kind/namespace/name; an
// un-audited object returns an empty result (found=false), not an error. Covers I-6a.
func TestInitiatorLookup(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	svc := initiator.New(pool)

	created := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	// A CREATE by the human, then a later UPDATE by automation — the initiator is the creator.
	pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
		EventUID: "evt-create", ReceivedAt: created, Operation: "CREATE",
		APIGroup: "tekton.dev", APIVersion: "v1", Resource: "pipelineruns", Kind: "PipelineRun",
		Namespace: "edp", Name: "build-1", ObjectUID: pgtest.Ptr("uid-123"), Username: "dev@example.com",
	})
	pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
		EventUID: "evt-update", ReceivedAt: created.Add(time.Hour), Operation: "UPDATE",
		APIGroup: "tekton.dev", APIVersion: "v1", Resource: "pipelineruns", Kind: "PipelineRun",
		Namespace: "edp", Name: "build-1", ObjectUID: pgtest.Ptr("uid-123"), Username: "system:serviceaccount:edp:tekton",
	})

	t.Run("by object UID", func(t *testing.T) {
		res, err := svc.ByObjectUID(ctx, "uid-123")
		require.NoError(t, err)
		require.True(t, res.Found)
		require.Equal(t, "dev@example.com", res.Actor)
		require.Equal(t, models.OperationCreate, res.Operation)
		require.Equal(t, created, res.Timestamp.UTC())
	})

	t.Run("by kind/namespace/name", func(t *testing.T) {
		res, err := svc.ByObject(ctx, "PipelineRun", "edp", "build-1")
		require.NoError(t, err)
		require.True(t, res.Found)
		require.Equal(t, "dev@example.com", res.Actor)
	})

	t.Run("never audited returns empty, not error", func(t *testing.T) {
		res, err := svc.ByObjectUID(ctx, "does-not-exist")
		require.NoError(t, err)
		require.False(t, res.Found)
		require.Empty(t, res.Actor)
	})
}

// A dry-run CREATE must never be reported as the initiator: the service queries the
// audit_events_real view, which excludes previews. Covers the dry-run branch.
func TestInitiatorExcludesDryRun(t *testing.T) {
	pool := pgtest.New(t)
	ctx := context.Background()
	svc := initiator.New(pool)

	ts := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	pgtest.InsertEvent(t, pool, pgtest.SeedEvent{
		EventUID: "evt-dry", ReceivedAt: ts, Operation: "CREATE", APIGroup: "tekton.dev",
		APIVersion: "v1", Resource: "pipelineruns", Kind: "PipelineRun", Namespace: "edp",
		Name: "preview-1", ObjectUID: pgtest.Ptr("uid-dry"), Username: "dev@example.com", DryRun: true,
	})

	res, err := svc.ByObjectUID(ctx, "uid-dry")
	require.NoError(t, err)
	require.False(t, res.Found, "dry-run preview must not be reported as the creator")
}
