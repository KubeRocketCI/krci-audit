package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/services/events"
)

// stubEvents records the filter it received and returns a preconfigured result.
type stubEvents struct {
	gotFilter events.Filter
	rows      []models.AuditEvent
	total     int
	err       error
}

func (s *stubEvents) Query(_ context.Context, f events.Filter) ([]models.AuditEvent, int, error) {
	s.gotFilter = f
	return s.rows, s.total, s.err
}

func TestListAuditEventsMapsFiltersAndClampsPagination(t *testing.T) {
	sub := "status"
	uid := "obj-1"
	stub := &stubEvents{
		rows: []models.AuditEvent{{
			EventUID: "e1", Operation: models.OperationCreate, Resource: "pipelineruns",
			Kind: "PipelineRun", Namespace: "edp", Name: "b1", Username: "alice",
			SubResource: &sub, ObjectUID: &uid,
		}},
		total: 1,
	}
	h := NewEventsHandler(stub)

	op := Operation("CREATE")
	over := 500
	resp, err := h.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{
		Params: ListAuditEventsParams{
			Actor: ptrTo("alice"), Operation: &op, Namespace: ptrTo("edp"), PerPage: &over,
		},
	})
	require.NoError(t, err)

	ok, is := resp.(ListAuditEvents200JSONResponse)
	require.True(t, is)

	require.NotNil(t, stub.gotFilter.Operation)
	require.Equal(t, "CREATE", *stub.gotFilter.Operation)
	require.Equal(t, "alice", *stub.gotFilter.Actor)
	require.Equal(t, "edp", *stub.gotFilter.Namespace)

	// Pagination clamped: perPage 500 → 100, page defaulted to 1, reflected in both the
	// service call and the response.
	require.Equal(t, 100, stub.gotFilter.PerPage)
	require.Equal(t, 1, stub.gotFilter.Page)
	require.Equal(t, 100, ok.Pagination.PerPage)
	require.Equal(t, 1, ok.Pagination.Page)
	require.Equal(t, 1, ok.Pagination.Total)

	// Nullable columns are surfaced.
	require.Len(t, ok.Data, 1)
	require.Equal(t, "status", ok.Data[0].SubResource.MustGet())
	require.Equal(t, "obj-1", ok.Data[0].ObjectUid.MustGet())
}

func TestListAuditEventsRejectsInvalidOperation(t *testing.T) {
	stub := &stubEvents{}
	h := NewEventsHandler(stub)

	op := Operation("BOGUS")
	resp, err := h.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{
		Params: ListAuditEventsParams{Operation: &op},
	})
	require.NoError(t, err)

	bad, is := resp.(ListAuditEvents400JSONResponse)
	require.True(t, is)
	require.Equal(t, "400", bad.Code)
	require.Zero(t, stub.gotFilter, "service must not be queried on invalid input")
}

func TestListAuditEventsRejectsInvertedTimeRange(t *testing.T) {
	stub := &stubEvents{}
	h := NewEventsHandler(stub)

	from := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	resp, err := h.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{
		Params: ListAuditEventsParams{From: &from, To: &to},
	})
	require.NoError(t, err)

	bad, is := resp.(ListAuditEvents400JSONResponse)
	require.True(t, is)
	require.Equal(t, "400", bad.Code)
	require.Zero(t, stub.gotFilter, "service must not be queried on invalid input")
}

func TestListAuditEventsServiceErrorIs500(t *testing.T) {
	stub := &stubEvents{err: errors.New("connection reset")}
	h := NewEventsHandler(stub)

	resp, err := h.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{})
	require.NoError(t, err)

	bad, is := resp.(ListAuditEvents500JSONResponse)
	require.True(t, is)
	require.Equal(t, "500", bad.Code)
	require.NotContains(t, bad.Message, "connection reset", "internal error detail must not reach the client")
}

func TestListAuditEventsEmptyIsNotError(t *testing.T) {
	h := NewEventsHandler(&stubEvents{rows: nil, total: 0})
	resp, err := h.ListAuditEvents(context.Background(), ListAuditEventsRequestObject{})
	require.NoError(t, err)
	ok, is := resp.(ListAuditEvents200JSONResponse)
	require.True(t, is)
	require.Empty(t, ok.Data)
	require.Zero(t, ok.Pagination.Total)
}

// Read-only surface: the generated router mounts only GET handlers, so a mutating verb on an
// audit path is rejected by chi (405). This proves there is no create/update/delete route at
// the HTTP surface.
func TestReadOnlySurfaceRejectsMutations(t *testing.T) {
	srv := NewServer(
		NewInitiatorHandler(&stubInitiator{}),
		NewEventsHandler(&stubEvents{}),
	)
	router := HandlerFromMux(NewStrictHandlerWithOptions(srv, nil, StrictHTTPServerOptions{}), chi.NewMux())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/audit/events", nil)
		router.ServeHTTP(rec, req)
		require.Equalf(t, http.StatusMethodNotAllowed, rec.Code,
			"%s on /api/v1/audit/events must be rejected (read-only surface)", method)
	}
}
