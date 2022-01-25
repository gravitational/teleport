package auth

import (
	"context"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// cloudWithRoles extends OSS auth server API with enterprise-specific features
type cloudWithRoles struct {
	plugin *Plugin
}

// ListBillingCycles lists billing cycles
func (ac *cloudWithRoles) ListBillingCycles(ctx context.Context, req *v1.EmptyRequest) (*v1.ListBillingCyclesResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.ListBillingCycles(ctx, req)
}

// RemoveCard removes a credit card from tenant account
func (ac *cloudWithRoles) RemoveCard(ctx context.Context, req *v1.RemoveCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.RemoveCard(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingCardDelete{
		Metadata: apievents.Metadata{
			Type: events.BillingCardDeleteEvent,
			Code: events.BillingCardDeleteCode,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.emitter.EmitAuditEvent(ctx, event); err != nil {
		ac.plugin.Log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing card delete event.")
	}

	return res, nil
}

// UpdateCard updates a tenant credit card
func (ac *cloudWithRoles) UpdateCard(ctx context.Context, req *v1.UpdateCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.UpdateCard(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingCardCreate{
		Metadata: apievents.Metadata{
			Type: events.BillingCardUpdateEvent,
			Code: events.BillingCardUpdateCode,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.emitter.EmitAuditEvent(ctx, event); err != nil {
		ac.plugin.Log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing card update event.")
	}

	return res, nil
}

// UpdateAccount updates account information
func (ac *cloudWithRoles) UpdateAccount(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.UpdateAccount(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingInformationUpdate{
		Metadata: apievents.Metadata{
			Type: events.BillingInformationUpdateEvent,
			Code: events.BillingInformationUpdateCode,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.emitter.EmitAuditEvent(ctx, event); err != nil {
		ac.plugin.Log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing account update event.")
	}

	return res, nil
}

// AddCreditCard adds a new credit card
func (ac *cloudWithRoles) AddCard(ctx context.Context, req *v1.AddCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.AddCard(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingCardCreate{
		Metadata: apievents.Metadata{
			Type: events.BillingCardCreateEvent,
			Code: events.BillingCardCreateCode,
		},
		UserMetadata: auth.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.emitter.EmitAuditEvent(ctx, event); err != nil {
		ac.plugin.Log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing card create event.")
	}

	return res, nil
}

// GetBillingInformation returns billing information
func (ac *cloudWithRoles) GetBillingInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetBillingInformationResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetBillingInformation(ctx, req)
}

// SubmitUsageReports submits usage report for processing
func (ac *cloudWithRoles) SubmitUsageReports(ctx context.Context, req *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error) {
	return nil, trace.NotImplemented("SubmitUsageReports cannot be called via Auth Service.")
}

// ListInvoices lists tenant invoices
func (ac *cloudWithRoles) ListInvoices(ctx context.Context, req *v1.EmptyRequest) (*v1.ListInvoicesResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.ListInvoices(ctx, req)
}

// SendAccountRecoveryLink sends an email with a recovery link to user.
func (ac *cloudWithRoles) SendAccountRecoveryLink(ctx context.Context, req *v1.SendAccountRecoveryLinkRequest) (*v1.EmptyResponse, error) {
	if err := ac.hasBuiltinProxyRole(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.SendAccountRecoveryLink(ctx, req)
}

// SendAccountLocked sends an email notifying user their account was locked.
func (ac *cloudWithRoles) SendAccountLocked(ctx context.Context, req *v1.SendAccountLockedRequest) (*v1.EmptyResponse, error) {
	if err := ac.hasBuiltinProxyRole(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.SendAccountLocked(ctx, req)
}

// SendAccountRecovered sends an email notifying user their account was successfully recovered.
func (ac *cloudWithRoles) SendAccountRecovered(ctx context.Context, req *v1.SendAccountRecoveredRequest) (*v1.EmptyResponse, error) {
	if err := ac.hasBuiltinProxyRole(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.SendAccountRecovered(ctx, req)
}

func (ac *cloudWithRoles) action(ctx context.Context, namespace, resource, action string) error {
	if ac.plugin.cloudClient == nil {
		return trace.AccessDenied("cloud features are disabled")
	}

	authCtx, err := ac.plugin.authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	return authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		namespace,
		resource,
		action,
		false)
}

// hasBuiltinProxyRole checks if context contains built in role proxy
func (ac *cloudWithRoles) hasBuiltinProxyRole(ctx context.Context) error {
	authCtx, err := ac.plugin.authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	if !auth.HasBuiltinRole(authCtx.Checker, string(types.RoleProxy)) {
		return trace.AccessDenied("this request can be only executed by a proxy")
	}

	return nil
}
