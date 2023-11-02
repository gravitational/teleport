package auth

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// cloudWithRoles extends OSS auth server API with enterprise-specific features
type cloudWithRoles struct {
	plugin *Plugin
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
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
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
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing card update event.")
	}

	return res, nil
}

// AddCard adds a new credit card
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
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
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

// GetBillingSummaryInformation returns the users Billing Summary Information
func (ac *cloudWithRoles) GetBillingSummaryInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetBillingSummaryInformationResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetBillingSummaryInformation(ctx, req)
}

// CancelSubscription cancels the customers subscription
func (ac *cloudWithRoles) CancelSubscription(ctx context.Context, req *v1.EmptyRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.CancelSubscription(ctx, req)
}

// GetPaymentsInvoicesInformation returns users the Payments Invoices Information
func (ac *cloudWithRoles) GetPaymentsInvoicesInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetPaymentsInvoicesInformationResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetPaymentsInvoicesInformation(ctx, req)
}

// GetInvoiceSettingsInformation returns the users Invoice Settings Information
func (ac *cloudWithRoles) GetInvoiceSettingsInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetInvoiceSettingsInformationResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetInvoiceSettingsInformation(ctx, req)
}

// CreateSetupIntent creates a Stripe setup intent and returns the client secret. A Stripe SetupIntent guides the
// process of setting up and saving a customer's payment credentials for future payments. https://stripe.com/docs/api/setup_intents
func (ac *cloudWithRoles) CreateSetupIntent(ctx context.Context, req *v1.EmptyRequest) (*v1.CreateSetupIntentResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ac.plugin.cloudClient.CreateSetupIntent(ctx, req)
}

// SubmitUsageReports submits usage report for processing
func (ac *cloudWithRoles) SubmitUsageReports(ctx context.Context, req *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error) {
	return nil, trace.NotImplemented("SubmitUsageReports cannot be called via Auth Service.")
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

// UpdateAccountUpgradeWindowStartHour updates the start of the account upgrade window for cloud users.
func (ac *cloudWithRoles) UpdateAccountUpgradeWindowStartHour(ctx context.Context, req *v1.UpdateAccountUpgradeWindowStartHourRequest) (*v1.EmptyResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.UpdateAccountUpgradeWindowStartHour(ctx, req)
}

// GetAccountUpgradeWindowStartHour returns the start of the account upgrade window for cloud users.
func (ac *cloudWithRoles) GetAccountUpgradeWindowStartHour(ctx context.Context, req *v1.EmptyRequest) (*v1.GetAccountUpgradeWindowStartHourResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.GetAccountUpgradeWindowStartHour(ctx, req)
}

// GetFeatures returns the features enabled in the Teleport Cloud cluster
func (ac *cloudWithRoles) GetFeatures(ctx context.Context, req *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.GetFeatures(ctx, req)
}

// GetSurveyCompany returns the company survey responses for the account associated with the current user.
func (ac *cloudWithRoles) GetSurveyCompany(ctx context.Context, req *v1.EmptyRequest) (*v1.SurveyCompanyResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.GetSurveyCompany(ctx, req)
}

// SetSurveyResults updates the account object with onboarding survey results
func (ac *cloudWithRoles) SetSurveyResults(ctx context.Context, req *v1.SetSurveyResultsRequest) (*v1.EmptyResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.SetSurveyResults(ctx, req)
}

// UpdateStripeAddress updates account address information
func (ac *cloudWithRoles) UpdateStripeAddress(ctx context.Context, req *v1.StripeBillingAddressRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.UpdateStripeAddress(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingInformationUpdate{
		Metadata: apievents.Metadata{
			Type: events.BillingInformationUpdateEvent,
			Code: events.BillingInformationUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing account update event.")
	}

	return res, nil
}

// UpdateEmail updates email address information
func (ac *cloudWithRoles) UpdateEmail(ctx context.Context, req *v1.UpdateEmailRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.UpdateEmail(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingInformationUpdate{
		Metadata: apievents.Metadata{
			Type: events.BillingInformationUpdateEvent,
			Code: events.BillingInformationUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing account update event.")
	}

	return res, nil
}

// UpdatePurchaseOrderPrefix updates purchase order prefix
func (ac *cloudWithRoles) UpdatePurchaseOrderPrefix(ctx context.Context, req *v1.UpdatePurchaseOrderPrefixRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, apidefaults.Namespace, types.KindBilling, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.UpdatePurchaseOrderPrefix(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.BillingInformationUpdate{
		Metadata: apievents.Metadata{
			Type: events.BillingInformationUpdateEvent,
			Code: events.BillingInformationUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
	}
	if err := ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"user":         event.UserMetadata.User,
			"impersonator": event.UserMetadata.Impersonator,
		}).Warn("Failed to emit billing account update event.")
	}

	return res, nil
}

// SendTeleportInvite emails a cluster invitation link to a user via the Cloud
// API.
func (ac *cloudWithRoles) SendTeleportInvite(ctx context.Context, req *v1.SendTeleportInviteRequest) (*v1.EmptyResponse, error) {
	// Note: we want to inherit the permissions of the normal CreateUser() +
	// CreateResetPasswordToken() flow
	err := ac.action(ctx, apidefaults.Namespace, types.KindUser, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = ac.action(ctx, apidefaults.Namespace, types.KindUser, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.SendTeleportInvite(ctx, req)
	if err != nil {
		ac.plugin.authServer.WithError(err).Warnf("SendTeleportInvite failed")
		return nil, trace.Wrap(err)
	}

	return res, nil
}

func (ac *cloudWithRoles) action(ctx context.Context, namespace, resource, action string) error {
	if ac.plugin.cloudClient == nil {
		return trace.AccessDenied("cloud features are disabled")
	}

	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
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
	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	if !auth.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return trace.AccessDenied("this request can be only executed by a proxy")
	}

	return nil
}
