package auth

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// cloudWithRoles extends OSS auth server API with enterprise-specific features
type cloudWithRoles struct {
	plugin *Plugin
	v1.UnimplementedTenantsServiceServer
}

// GetBillingInformation returns billing information
func (ac *cloudWithRoles) GetBillingInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetBillingInformationResponse, error) {
	err := ac.action(ctx, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetBillingInformation(ctx, req)
}

// GetBillingSummaryInformation returns the users Billing Summary Information
func (ac *cloudWithRoles) GetBillingSummaryInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetBillingSummaryInformationResponse, error) {
	err := ac.action(ctx, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetBillingSummaryInformation(ctx, req)
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

// SendTeleportInvite emails a cluster invitation link to a user via the Cloud
// API.
func (ac *cloudWithRoles) SendTeleportInvite(ctx context.Context, req *v1.SendTeleportInviteRequest) (*v1.EmptyResponse, error) {
	// Note: we want to inherit the permissions of the normal CreateUser() +
	// CreateResetPasswordToken() flow
	err := ac.action(ctx, types.KindUser, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = ac.action(ctx, types.KindUser, types.VerbUpdate)
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

// ClusterAlertInfo returns information about a cluster that will determine if the Teleport usage reporter should generate a cluster alert
func (ac *cloudWithRoles) ClusterAlertInfo(ctx context.Context, req *v1.EmptyRequest) (*v1.ClusterAlertInfoResponse, error) {
	_, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.ClusterAlertInfo(ctx, req)
}

// GetUpdatedLicense returns the customer's license if it is different from the license
// provided as the mTLS peer certificate.
func (ac *cloudWithRoles) GetUpdatedLicense(ctx context.Context, in *v1.GetUpdatedLicenseRequest) (*v1.GetUpdatedLicenseResponse, error) {
	if _, err := ac.plugin.authServer.Authorizer.Authorize(ctx); err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	return ac.plugin.cloudClient.GetUpdatedLicense(ctx, in)
}

// GetContacts returns a list of the business and security contacts.
func (ac *cloudWithRoles) GetContacts(ctx context.Context, in *v1.EmptyRequest) (*v1.GetContactsResponse, error) {
	if err := ac.action(ctx, types.KindContact, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetContacts(ctx, in)
}

// RemoveContact removes a contact.
func (ac *cloudWithRoles) RemoveContact(ctx context.Context, in *v1.RemoveContactRequest) (*v1.RemoveContactResponse, error) {
	if err := ac.action(ctx, types.KindContact, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.RemoveContact(ctx, in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eventEmail string
	if res.Contact != nil {
		eventEmail = res.Contact.Email
	}
	if err := ac.emitContactAuditEvent(ctx, contactEventTypeDelete, eventEmail, apievents.ContactType(in.ContactType)); err != nil {
		slog.WarnContext(ctx, "failed to emit contact delete event", "error", err)
	}

	return res, nil
}

// CreateContact creates a contact
func (ac *cloudWithRoles) CreateContact(ctx context.Context, in *v1.CreateContactRequest) (*v1.CreateContactResponse, error) {
	if err := ac.action(ctx, types.KindContact, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := ac.plugin.cloudClient.CreateContact(ctx, in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ac.emitContactAuditEvent(ctx, contactEventTypeCreate, in.Email, apievents.ContactType(in.ContactType)); err != nil {
		slog.WarnContext(ctx, "failed to emit contact create event", "error", err)
	}

	return res, nil
}

func (ac *cloudWithRoles) action(ctx context.Context, resource, action string) error {
	if ac.plugin.cloudClient == nil {
		return trace.AccessDenied("cloud features are disabled")
	}

	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	return authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		apidefaults.Namespace,
		resource,
		action)
}

// hasBuiltinProxyRole checks if context contains built in role proxy
func (ac *cloudWithRoles) hasBuiltinProxyRole(ctx context.Context) error {
	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return trace.AccessDenied("this request can be only executed by a proxy")
	}

	return nil
}

type contactEventType uint8

const (
	contactEventTypeCreate = iota
	contactEventTypeDelete
)

// emitContactAuditEvent emits a contact audit event event with metadata from the context.
// Returns an error if emitting the event failed, or if it can't read the user information auth context.
func (ac *cloudWithRoles) emitContactAuditEvent(ctx context.Context, event contactEventType, email string, contactType apievents.ContactType) error {
	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return err
	}
	status := apievents.Status{
		Success: true,
	}
	resourceMetadata := apievents.ResourceMetadata{
		Name:      types.KindContact,
		UpdatedBy: authCtx.Identity.GetIdentity().Username,
	}
	switch event {
	case contactEventTypeCreate:
		return ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, &apievents.ContactCreate{
			Metadata: apievents.Metadata{
				Type: libevents.ContactCreateEvent,
				Code: libevents.ContactCreateCode,
			},
			UserMetadata:       authCtx.GetUserMetadata(),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status:             status,
			ResourceMetadata:   resourceMetadata,
			Email:              email,
			ContactType:        contactType,
		})
	case contactEventTypeDelete:
		return ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, &apievents.ContactDelete{
			Metadata: apievents.Metadata{
				Type: libevents.ContactDeleteEvent,
				Code: libevents.ContactDeleteCode,
			},
			UserMetadata:       authCtx.GetUserMetadata(),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status:             status,
			ResourceMetadata:   resourceMetadata,
			Email:              email,
			ContactType:        contactType,
		})
	default:
		return trace.BadParameter("unknown contact audit event type %d", event)
	}
}
