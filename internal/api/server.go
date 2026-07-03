package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KubeRocketCI/krci-audit/internal/services/events"
	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

var _ StrictServerInterface = (*Server)(nil)

// Server implements the generated StrictServerInterface by delegating each operation to its
// capability handler.
type Server struct {
	initiatorHandler *InitiatorHandler
	eventsHandler    *EventsHandler
}

// NewServer creates a Server delegating each capability to its handler.
func NewServer(initiatorHandler *InitiatorHandler, eventsHandler *EventsHandler) *Server {
	return &Server{
		initiatorHandler: initiatorHandler,
		eventsHandler:    eventsHandler,
	}
}

// GetInitiator implements StrictServerInterface.
func (s *Server) GetInitiator(
	ctx context.Context,
	request GetInitiatorRequestObject,
) (GetInitiatorResponseObject, error) {
	return s.initiatorHandler.GetInitiator(ctx, request)
}

// ListAuditEvents implements StrictServerInterface.
func (s *Server) ListAuditEvents(
	ctx context.Context,
	request ListAuditEventsRequestObject,
) (ListAuditEventsResponseObject, error) {
	return s.eventsHandler.ListAuditEvents(ctx, request)
}

// BuildHandler wires the capability services (backed by the read pool, connected as
// audit_reader) into their handlers and wraps them in the oapi-codegen strict handler.
func BuildHandler(pool *pgxpool.Pool) ServerInterface {
	server := NewServer(
		NewInitiatorHandler(initiator.New(pool)),
		NewEventsHandler(events.New(pool)),
	)

	return NewStrictHandlerWithOptions(server, []StrictMiddlewareFunc{}, StrictHTTPServerOptions{})
}
