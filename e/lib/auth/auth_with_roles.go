package auth

import (
	"context"
	"errors"
	"io"
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

// GetUsage returns usage information for one or more tenants.
func (ac *cloudWithRoles) GetUsage(ctx context.Context, req *v1.GetUsageRequest) (*v1.GetUsageResponse, error) {
	err := ac.action(ctx, types.KindBilling, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetUsage(ctx, req)
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
		ac.plugin.logger.WarnContext(ctx, "SendTeleportInvite failed", "error", err)
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

func (ac *cloudWithRoles) GetClientIPRestrictions(ctx context.Context, in *v1.GetClientIPRestrictionsRequest) (*v1.GetClientIPRestrictionsResponse, error) {
	if err := ac.action(ctx, types.KindClientIPRestriction, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return ac.plugin.cloudClient.GetClientIPRestrictions(ctx, in)
}

func (ac *cloudWithRoles) GetFile(req *v1.GetFileRequest, srv v1.TenantsService_GetFileServer) error {
	if ac.plugin.cloudClient == nil {
		return trace.AccessDenied("cloud features are disabled")
	}

	clientStream, err := ac.plugin.cloudClient.GetFile(srv.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		chunk, err := clientStream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if err := srv.Send(chunk); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (ac *cloudWithRoles) PutClientIPRestrictions(ctx context.Context, in *v1.PutClientIPRestrictionsRequest) (*v1.PutClientIPRestrictionsResponse, error) {
	if err := ac.action(ctx, types.KindClientIPRestriction, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := ac.plugin.cloudClient.PutClientIPRestrictions(ctx, in)

	if err := ac.emitClientIPRestrictionstAuditEvent(ctx, resp, err); err != nil {
		slog.WarnContext(ctx, "failed to emit client ip restriction update event", "error", err)
	}

	return resp, err
}

func (ac *cloudWithRoles) action(ctx context.Context, resource string, actions ...string) error {
	if ac.plugin.cloudClient == nil {
		return trace.AccessDenied("cloud features are disabled")
	}

	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("access denied")
	}

	var errs []error

	for _, action := range actions {
		if err := authCtx.Checker.CheckAccessToRule(
			&services.Context{User: authCtx.User},
			apidefaults.Namespace,
			resource,
			action); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
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

func (ac *cloudWithRoles) emitClientIPRestrictionstAuditEvent(ctx context.Context, resp *v1.PutClientIPRestrictionsResponse, respErr error) error {
	authCtx, err := ac.plugin.authServer.Authorizer.Authorize(ctx)
	if err != nil {
		return err
	}
	resourceMetadata := apievents.ResourceMetadata{
		Name:      types.KindClientIPRestriction,
		UpdatedBy: authCtx.Identity.GetIdentity().Username,
	}

	var cirStr []string

	status := apievents.Status{
		Success: respErr == nil,
	}
	if respErr != nil {
		status.Error = trace.Unwrap(respErr).Error()
		status.UserMessage = respErr.Error()
	} else {
		// only populate the cidr blocks on success to avoid
		// clogging the logs with invalid blocks
		if resp != nil {
			for _, item := range resp.ClientIpRestrictions {
				cirStr = append(cirStr, item.Cidr)
			}
		}
	}

	return ac.plugin.authServer.Emitter.EmitAuditEvent(ctx, &apievents.ClientIPRestrictionsUpdate{
		Metadata: apievents.Metadata{
			Type: libevents.ClientIPRestrictionsUpdateEvent,
			Code: libevents.ClientIPRestrictionsUpdateCode,
		},
		UserMetadata:         authCtx.GetUserMetadata(),
		ConnectionMetadata:   authz.ConnectionMetadata(ctx),
		Status:               status,
		ResourceMetadata:     resourceMetadata,
		ClientIPRestrictions: cirStr,
	})
}
