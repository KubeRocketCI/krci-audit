package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

// TestServerDelegatesToHandlers pins that Server is a plain delegator: each
// StrictServerInterface method must call through to its capability handler rather than
// re-implementing behaviour. TestReadOnlySurfaceRejectsMutations (events_handler_test.go)
// already exercises the routing side of NewServer/HandlerFromMux; this covers the direct
// method delegation with no HTTP layer involved.
func TestServerDelegatesToHandlers(t *testing.T) {
	initiatorStub := &stubInitiator{res: initiator.Result{Found: true, Actor: "dev@x"}}
	eventsStub := &stubEvents{total: 1}
	facetsStub := &stubFacets{}
	srv := NewServer(NewInitiatorHandler(initiatorStub), NewEventsHandler(eventsStub), NewFacetsHandler(facetsStub))

	_, err := srv.GetInitiator(context.Background(), GetInitiatorRequestObject{
		Params: GetInitiatorParams{ObjectUid: ptrTo("uid-1")},
	})
	require.NoError(t, err)
	require.True(t, initiatorStub.byUIDCalled, "GetInitiator must delegate to the initiator handler")

	_, err = srv.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{})
	require.NoError(t, err)
	require.Equal(t, 1, eventsStub.gotFilter.Page, "ListAuditEvents must delegate to the events handler")

	_, err = srv.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{})
	require.NoError(t, err)
	require.True(t, facetsStub.called, "ListAuditFacets must delegate to the facets handler")
}
