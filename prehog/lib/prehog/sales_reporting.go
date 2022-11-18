package prehog

import (
	"context"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/rs/zerolog/log"

	prehogv1 "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
	prehogv1c "github.com/gravitational/prehog/gen/proto/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/prehog/lib/authn"
	"github.com/gravitational/prehog/lib/posthog"
)

var _ prehogv1c.SalesReportingServiceHandler = (*Handler)(nil)

// IdentifyAccount implements SalesReportingServiceHandler
func (h *Handler) IdentifyAccount(ctx context.Context, req *connect.Request[prehogv1.IdentifyAccountRequest]) (*connect.Response[prehogv1.IdentifyAccountResponse], error) {
	if !authn.IsCAFromContext(ctx) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	accountID := req.Msg.GetAccountId()
	if accountID == "" {
		return nil, invalidArgument("missing account_id")
	}

	event := &posthog.Event{
		DistinctID: distinctIDPrefix + accountID,
		Event:      posthog.CreateAliasEvent,
		Timestamp:  time.Now().UTC(),
		Set: map[posthog.PersonProperty]any{
			accountIDPersonProperty: accountID,
			isCloudPersonProperty:   req.Msg.GetIsCloud(),
			isTrialProperty:         req.Msg.GetIsTrial(),
		},
	}

	// TODO(espadolini): validate the marketing ID further
	if marketingID := req.Msg.GetMarketingId(); marketingID != "" {
		if strings.HasPrefix(marketingID, distinctIDPrefix) {
			log.Warn().Str("marketing_id", marketingID).Msg("Ignoring impossible marketing_id in AccountIdentify")
		} else {
			event.AddProperty(posthog.AliasProperty, marketingID)
		}
	}

	if licenseName := req.Msg.GetLicenseName(); licenseName != "" {
		event.AddSet(licenseNamePersonProperty, licenseName)
	}

	dur, err := h.client.Emit(ctx, event)
	log.Err(err).Interface("event", event).Dur("elapsed", dur).Msg("IdentifyAccount Emit")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, nil)
	}

	return &connect.Response[prehogv1.IdentifyAccountResponse]{}, nil
}

// UpdateAccount implements SalesReportingServiceHandler
func (h *Handler) UpdateAccount(ctx context.Context, req *connect.Request[prehogv1.UpdateAccountRequest]) (*connect.Response[prehogv1.UpdateAccountResponse], error) {
	if !authn.IsCAFromContext(ctx) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	accountID := req.Msg.GetAccountId()
	if accountID == "" {
		return nil, invalidArgument("missing account_id")
	}

	event := &posthog.Event{
		DistinctID: distinctIDPrefix + accountID,
		Event:      posthog.CreateAliasEvent,
		Timestamp:  time.Now().UTC(),
		Set: map[posthog.PersonProperty]any{
			accountIDPersonProperty: accountID,
			isCloudPersonProperty:   req.Msg.GetIsCloud(),
			isTrialProperty:         req.Msg.GetIsTrial(),
		},
	}

	if oldAccountID := req.Msg.GetOldAccountId(); oldAccountID != "" {
		event.AddProperty(posthog.AliasProperty, oldAccountID)
	}

	if licenseName := req.Msg.GetLicenseName(); licenseName != "" {
		event.AddSet(licenseNamePersonProperty, licenseName)
	}

	dur, err := h.client.Emit(ctx, event)
	log.Err(err).Interface("event", event).Dur("elapsed", dur).Msg("UpdateAccount Emit")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, nil)
	}

	return &connect.Response[prehogv1.UpdateAccountResponse]{}, nil
}

// SubmitSalesEvent implements SalesReportingServiceHandler
func (h *Handler) SubmitSalesEvent(ctx context.Context, req *connect.Request[prehogv1.SubmitSalesEventRequest]) (*connect.Response[prehogv1.SubmitSalesEventResponse], error) {
	if !authn.IsCAFromContext(ctx) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	event, err := encodeSubmitSalesEvent(req.Msg)
	if err != nil {
		return nil, err
	}

	dur, err := h.client.Emit(ctx, event)
	log.Err(err).Str("event", string(event.Event)).Dur("elapsed", dur).Msg("SubmitSalesEvent Emit")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, nil)
	}

	return &connect.Response[prehogv1.SubmitSalesEventResponse]{}, nil
}
