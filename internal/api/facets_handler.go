package api

import (
	"context"
	"net/http"

	"github.com/KubeRocketCI/krci-audit/internal/services/facets"
)

// facetsQuerier abstracts the facets query capability for testability.
type facetsQuerier interface {
	Query(ctx context.Context, fields []facets.Field) (map[facets.Field]facets.FieldValues, error)
}

// FacetsHandler serves distinct-value lookups over the whitelisted facet fields.
type FacetsHandler struct {
	service facetsQuerier
}

// NewFacetsHandler creates a FacetsHandler backed by the given query capability.
func NewFacetsHandler(service facetsQuerier) *FacetsHandler {
	return &FacetsHandler{service: service}
}

// ListAuditFacets implements api.StrictServerInterface. Defaults to every facet field when
// none are requested; an unknown field is a 400. Only the requested fields are present in the
// response.
func (h *FacetsHandler) ListAuditFacets(
	ctx context.Context,
	request ListAuditFacetsRequestObject,
) (ListAuditFacetsResponseObject, error) {
	requested := facets.AllFields()
	if request.Params.Fields != nil && len(*request.Params.Fields) > 0 {
		fields := make([]facets.Field, 0, len(*request.Params.Fields))
		for _, raw := range *request.Params.Fields {
			f, err := facets.ParseField(string(raw))
			if err != nil {
				return ListAuditFacets400JSONResponse{
					Code:    errorCode(http.StatusBadRequest),
					Message: "invalid facet field: " + string(raw),
				}, nil
			}
			fields = append(fields, f)
		}
		requested = fields
	}

	values, err := h.service.Query(ctx, requested)
	if err != nil {
		code, message := serverErrorResponse(err)
		return ListAuditFacets500JSONResponse{Code: code, Message: message}, nil
	}

	return ListAuditFacets200JSONResponse(toFacetsDTO(values)), nil
}

// toFacetsDTO maps the service result to the generated response DTO. Only requested fields
// (present as keys in values) are set; unrequested fields stay nil (omitted from the JSON).
func toFacetsDTO(values map[facets.Field]facets.FieldValues) AuditFacetsResponse {
	var dto AuditFacetsResponse
	if v, ok := values[facets.FieldNamespace]; ok {
		dto.Namespace = toFacetDTO(v)
	}
	if v, ok := values[facets.FieldKind]; ok {
		dto.Kind = toFacetDTO(v)
	}
	if v, ok := values[facets.FieldActor]; ok {
		dto.Actor = toFacetDTO(v)
	}
	return dto
}

// toFacetDTO maps one field's service result to the generated {values, truncated} DTO.
func toFacetDTO(fv facets.FieldValues) *Facet {
	values := fv.Values
	if values == nil {
		values = []string{}
	}
	return &Facet{Values: values, Truncated: fv.Truncated}
}
