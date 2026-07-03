package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

// stubInitiator records the call it received and returns a preconfigured result.
type stubInitiator struct {
	gotUID                      string
	gotKind, gotNs, gotName     string
	byUIDCalled, byObjectCalled bool

	res initiator.Result
	err error
}

func (s *stubInitiator) ByObjectUID(_ context.Context, uid string) (initiator.Result, error) {
	s.byUIDCalled, s.gotUID = true, uid
	return s.res, s.err
}

func (s *stubInitiator) ByObject(_ context.Context, kind, ns, name string) (initiator.Result, error) {
	s.byObjectCalled, s.gotKind, s.gotNs, s.gotName = true, kind, ns, name
	return s.res, s.err
}

func strptr(s string) *string { return &s }

func TestGetInitiatorByUID(t *testing.T) {
	ts := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	stub := &stubInitiator{res: initiator.Result{Found: true, Actor: "dev@x", Operation: models.OperationCreate, Timestamp: ts}}
	h := NewInitiatorHandler(stub)

	resp, err := h.GetInitiator(context.Background(), GetInitiatorRequestObject{
		Params: GetInitiatorParams{ObjectUid: strptr("uid-1")},
	})
	require.NoError(t, err)

	ok, is := resp.(GetInitiator200JSONResponse)
	require.True(t, is, "expected 200")
	require.True(t, stub.byUIDCalled)
	require.Equal(t, "uid-1", stub.gotUID)
	require.True(t, ok.Found)
	require.Equal(t, "dev@x", *ok.Actor)
	require.Equal(t, Operation("CREATE"), *ok.Operation)
}

func TestGetInitiatorByKindNamespaceName(t *testing.T) {
	stub := &stubInitiator{res: initiator.Result{Found: true, Actor: "dev@x", Operation: models.OperationCreate}}
	h := NewInitiatorHandler(stub)

	resp, err := h.GetInitiator(context.Background(), GetInitiatorRequestObject{
		Params: GetInitiatorParams{Kind: strptr("PipelineRun"), Namespace: strptr("edp"), Name: strptr("b1")},
	})
	require.NoError(t, err)
	_, is := resp.(GetInitiator200JSONResponse)
	require.True(t, is)
	require.True(t, stub.byObjectCalled)
	require.Equal(t, []string{"PipelineRun", "edp", "b1"}, []string{stub.gotKind, stub.gotNs, stub.gotName})
}

func TestGetInitiatorNotFoundIsEmptyNot404(t *testing.T) {
	stub := &stubInitiator{res: initiator.Result{Found: false}}
	h := NewInitiatorHandler(stub)

	resp, err := h.GetInitiator(context.Background(), GetInitiatorRequestObject{
		Params: GetInitiatorParams{ObjectUid: strptr("missing")},
	})
	require.NoError(t, err)
	ok, is := resp.(GetInitiator200JSONResponse)
	require.True(t, is, "un-audited object is a 200 empty result, not an error")
	require.False(t, ok.Found)
	require.Nil(t, ok.Actor)
}

func TestGetInitiatorMissingParamsIs400(t *testing.T) {
	stub := &stubInitiator{}
	h := NewInitiatorHandler(stub)

	// Neither objectUid nor a complete kind/namespace/name triple.
	resp, err := h.GetInitiator(context.Background(), GetInitiatorRequestObject{
		Params: GetInitiatorParams{Kind: strptr("PipelineRun")},
	})
	require.NoError(t, err)
	_, is := resp.(GetInitiator400JSONResponse)
	require.True(t, is, "incomplete identifiers → 400")
	require.False(t, stub.byUIDCalled)
	require.False(t, stub.byObjectCalled)
}
