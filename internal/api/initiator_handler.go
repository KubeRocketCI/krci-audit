package api

import (
	"context"
	"net/http"

	"github.com/KubeRocketCI/krci-audit/internal/services/initiator"
)

// initiatorResolver abstracts the initiator-lookup capability so the handler can be tested
// without a real service
type initiatorResolver interface {
	ByObjectUID(ctx context.Context, uid string) (initiator.Result, error)
	ByObject(ctx context.Context, kind, namespace, name string) (initiator.Result, error)
}

// InitiatorHandler serves "who created this object?" lookups.
type InitiatorHandler struct {
	service initiatorResolver
}

// NewInitiatorHandler creates an InitiatorHandler backed by the given resolver.
func NewInitiatorHandler(service initiatorResolver) *InitiatorHandler {
	return &InitiatorHandler{service: service}
}

// GetInitiator implements api.StrictServerInterface. It accepts either objectUid or the full
// kind+namespace+name triple; anything else is a 400. A never-audited object yields a 200 with
// found=false (not an error), so the caller can fall back to "unknown".
func (h *InitiatorHandler) GetInitiator(
	ctx context.Context,
	request GetInitiatorRequestObject,
) (GetInitiatorResponseObject, error) {
	p := request.Params

	var (
		res initiator.Result
		err error
	)

	switch {
	case p.ObjectUid != nil && *p.ObjectUid != "":
		res, err = h.service.ByObjectUID(ctx, *p.ObjectUid)
	case p.Kind != nil && *p.Kind != "" && p.Namespace != nil && *p.Namespace != "" && p.Name != nil && *p.Name != "":
		res, err = h.service.ByObject(ctx, *p.Kind, *p.Namespace, *p.Name)
	default:
		return GetInitiator400JSONResponse{
			Code:    errorCode(http.StatusBadRequest),
			Message: "provide objectUid, or kind + namespace + name",
		}, nil
	}

	if err != nil {
		code, message := serverErrorResponse(err)
		return GetInitiator500JSONResponse{Code: code, Message: message}, nil
	}

	return GetInitiator200JSONResponse(toInitiatorDTO(res)), nil
}

// toInitiatorDTO maps the service result to the generated response DTO. When Found is false
// the optional fields stay nil (omitted), so the response is simply {"found": false}.
func toInitiatorDTO(r initiator.Result) Initiator {
	dto := Initiator{Found: r.Found}
	if r.Found {
		actor := r.Actor
		op := Operation(r.Operation)
		ts := r.Timestamp
		dto.Actor = &actor
		dto.Operation = &op
		dto.Timestamp = &ts
	}
	return dto
}
