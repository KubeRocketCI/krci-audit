package api

import (
	"context"
	"net/http"

	"github.com/oapi-codegen/nullable"

	"github.com/KubeRocketCI/krci-audit/internal/models"
	"github.com/KubeRocketCI/krci-audit/internal/services/events"
)

// validOperations is the set of Operation values the API accepts as a filter, derived from
// models.AllOperations() (the single source of truth) rather than hand-listed again here.
var validOperations = buildValidOperations()

func buildValidOperations() map[Operation]bool {
	m := make(map[Operation]bool, len(models.AllOperations()))
	for _, op := range models.AllOperations() {
		m[Operation(op)] = true
	}
	return m
}

// eventsQuerier abstracts the audit-events query capability for testability.
type eventsQuerier interface {
	Query(ctx context.Context, f events.Filter) ([]models.AuditEvent, int, error)
}

// EventsHandler serves the general audit-events query.
type EventsHandler struct {
	service eventsQuerier
}

// NewEventsHandler creates an EventsHandler backed by the given query capability.
func NewEventsHandler(service eventsQuerier) *EventsHandler {
	return &EventsHandler{service: service}
}

// ListAuditEvents implements api.StrictServerInterface. Filters map straight to the service;
// pagination is clamped (default 1/20, max perPage 100). No match returns an empty list.
func (h *EventsHandler) ListAuditEvents(
	ctx context.Context,
	request ListAuditEventsRequestObject,
) (ListAuditEventsResponseObject, error) {
	p := request.Params
	if p.Operation != nil && !validOperations[*p.Operation] {
		return ListAuditEvents400JSONResponse{
			Code:    errorCode(http.StatusBadRequest),
			Message: "invalid operation filter",
		}, nil
	}

	if p.From != nil && p.To != nil && p.From.After(*p.To) {
		return ListAuditEvents400JSONResponse{
			Code:    errorCode(http.StatusBadRequest),
			Message: "invalid time range: from must not be after to",
		}, nil
	}

	page, perPage := clampPagination(p.Page, p.PerPage)

	var operation *string
	if p.Operation != nil {
		s := string(*p.Operation)
		operation = &s
	}

	filter := events.Filter{
		Actor:     p.Actor,
		Operation: operation,
		APIGroup:  p.Group,
		Resource:  p.Resource,
		Kind:      p.Kind,
		Namespace: p.Namespace,
		Name:      p.Name,
		ObjectUID: p.ObjectUid,
		From:      p.From,
		To:        p.To,
		Page:      page,
		PerPage:   perPage,
	}

	rows, total, err := h.service.Query(ctx, filter)
	if err != nil {
		code, message := serverErrorResponse(err)
		return ListAuditEvents500JSONResponse{Code: code, Message: message}, nil
	}

	data := make([]AuditEvent, 0, len(rows))
	for _, e := range rows {
		data = append(data, toEventDTO(e))
	}

	return ListAuditEvents200JSONResponse{
		Data: data,
		Pagination: Pagination{
			Total:   total,
			Page:    page,
			PerPage: perPage,
		},
	}, nil
}

// toEventDTO maps a domain AuditEvent to the generated lifted-column DTO.
func toEventDTO(e models.AuditEvent) AuditEvent {
	dto := AuditEvent{
		EventUid:   e.EventUID,
		ReceivedAt: e.ReceivedAt,
		Operation:  Operation(e.Operation),
		ApiGroup:   e.APIGroup,
		ApiVersion: e.APIVersion,
		Resource:   e.Resource,
		Kind:       e.Kind,
		Namespace:  e.Namespace,
		Name:       e.Name,
		Username:   e.Username,
		DryRun:     e.DryRun,
	}
	if e.SubResource != nil {
		dto.SubResource = nullable.NewNullableWithValue(*e.SubResource)
	}
	if e.ObjectUID != nil {
		dto.ObjectUid = nullable.NewNullableWithValue(*e.ObjectUID)
	}
	return dto
}
