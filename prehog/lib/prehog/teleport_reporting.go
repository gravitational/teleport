package prehog

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/rs/zerolog/log"

	prehogv1 "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
	prehogv1c "github.com/gravitational/prehog/gen/proto/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/prehog/lib/authn"
)

var _ prehogv1c.TeleportReportingServiceHandler = (*Handler)(nil)

// SubmitEvent implements TeleportReportingServiceHandler
func (h *Handler) SubmitEvent(ctx context.Context, req *connect.Request[prehogv1.SubmitEventRequest]) (*connect.Response[prehogv1.SubmitEventResponse], error) {
	lic, ok := authn.LicenseFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	event, err := encodeSubmitEvent(lic, req.Msg)
	if err != nil {
		return nil, err
	}

	dur, err := h.client.Emit(ctx, event)
	log.Err(err).Str("event", string(event.Event)).Dur("elapsed", dur).Msg("SubmitEvent Emit")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, nil)
	}

	return &connect.Response[prehogv1.SubmitEventResponse]{}, nil
}
