package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/krci-audit/internal/services/facets"
)

// stubFacets records the fields it received and returns a preconfigured result.
type stubFacets struct {
	called    bool
	gotFields []facets.Field
	values    map[facets.Field]facets.FieldValues
	err       error
}

func (s *stubFacets) Query(_ context.Context, fields []facets.Field) (map[facets.Field]facets.FieldValues, error) {
	s.called = true
	s.gotFields = fields
	return s.values, s.err
}

func TestListAuditFacetsDefaultsToAllFields(t *testing.T) {
	stub := &stubFacets{values: map[facets.Field]facets.FieldValues{
		facets.FieldNamespace: {Values: []string{"edp", "krci"}},
		facets.FieldKind:      {Values: []string{"Codebase", "PipelineRun"}},
		facets.FieldActor:     {Values: []string{"alice", "bob"}},
	}}
	h := NewFacetsHandler(stub)

	resp, err := h.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{})
	require.NoError(t, err)

	ok, is := resp.(ListAuditFacets200JSONResponse)
	require.True(t, is)

	require.ElementsMatch(t, facets.AllFields(), stub.gotFields, "no fields requested defaults to all")
	require.NotNil(t, ok.Namespace)
	require.Equal(t, []string{"edp", "krci"}, ok.Namespace.Values)
	require.False(t, ok.Namespace.Truncated)
	require.NotNil(t, ok.Kind)
	require.Equal(t, []string{"Codebase", "PipelineRun"}, ok.Kind.Values)
	require.False(t, ok.Kind.Truncated)
	require.NotNil(t, ok.Actor)
	require.Equal(t, []string{"alice", "bob"}, ok.Actor.Values)
	require.False(t, ok.Actor.Truncated)
}

func TestListAuditFacetsOnlyRequestedFieldsArePresent(t *testing.T) {
	stub := &stubFacets{values: map[facets.Field]facets.FieldValues{
		facets.FieldNamespace: {Values: []string{"edp"}},
	}}
	h := NewFacetsHandler(stub)

	requested := FacetFieldsParam{"namespace"}
	resp, err := h.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{
		Params: ListAuditFacetsParams{Fields: &requested},
	})
	require.NoError(t, err)

	ok, is := resp.(ListAuditFacets200JSONResponse)
	require.True(t, is)

	require.Equal(t, []facets.Field{facets.FieldNamespace}, stub.gotFields)
	require.NotNil(t, ok.Namespace)
	require.Equal(t, []string{"edp"}, ok.Namespace.Values)
	require.Nil(t, ok.Kind, "unrequested field must be omitted")
	require.Nil(t, ok.Actor, "unrequested field must be omitted")
}

func TestListAuditFacetsTruncatedFieldHasEmptyValues(t *testing.T) {
	stub := &stubFacets{values: map[facets.Field]facets.FieldValues{
		facets.FieldActor: {Truncated: true},
	}}
	h := NewFacetsHandler(stub)

	requested := FacetFieldsParam{"actor"}
	resp, err := h.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{
		Params: ListAuditFacetsParams{Fields: &requested},
	})
	require.NoError(t, err)

	ok, is := resp.(ListAuditFacets200JSONResponse)
	require.True(t, is)

	require.NotNil(t, ok.Actor)
	require.True(t, ok.Actor.Truncated)
	require.Empty(t, ok.Actor.Values, "truncated field must not return a partial list")
}

func TestListAuditFacetsRejectsUnknownField(t *testing.T) {
	stub := &stubFacets{}
	h := NewFacetsHandler(stub)

	requested := FacetFieldsParam{"namespace", "bogus"}
	resp, err := h.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{
		Params: ListAuditFacetsParams{Fields: &requested},
	})
	require.NoError(t, err)

	bad, is := resp.(ListAuditFacets400JSONResponse)
	require.True(t, is)
	require.Equal(t, "400", bad.Code)
	require.False(t, stub.called, "service must not be queried on invalid input")
}

func TestListAuditFacetsServiceErrorIs500(t *testing.T) {
	stub := &stubFacets{err: errors.New("connection reset")}
	h := NewFacetsHandler(stub)

	resp, err := h.ListAuditFacets(context.Background(), ListAuditFacetsRequestObject{})
	require.NoError(t, err)

	bad, is := resp.(ListAuditFacets500JSONResponse)
	require.True(t, is)
	require.Equal(t, "500", bad.Code)
	require.NotContains(t, bad.Message, "connection reset", "internal error detail must not reach the client")
}
