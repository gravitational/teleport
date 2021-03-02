package auth

import (
	"context"

	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
)

// cloudWithRoles extends OSS auth server API with enterprise-specific features
type cloudWithRoles struct {
	plugin *Plugin
}

// ListBillingCycles lists billing cycles
func (ac *cloudWithRoles) ListBillingCycles(ctx context.Context, req *v1.EmptyRequest) (*v1.ListBillingCyclesResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbList)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.ListBillingCycles(ctx, req)
}

// RemoveCard removes a credit card from tenant account
func (ac *cloudWithRoles) RemoveCard(ctx context.Context, req *v1.RemoveCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbDelete)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.RemoveCard(ctx, req)
}

// UpdateCard updates a tenant credit card
func (ac *cloudWithRoles) UpdateCard(ctx context.Context, req *v1.UpdateCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbUpdate)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.UpdateCard(ctx, req)
}

// UpdateAccount updates account information
func (ac *cloudWithRoles) UpdateAccount(ctx context.Context, req *v1.UpdateAccountRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbUpdate)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.UpdateAccount(ctx, req)
}

// AddCreditCard adds a new credit card
func (ac *cloudWithRoles) AddCard(ctx context.Context, req *v1.AddCardRequest) (*v1.EmptyResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbCreate)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.AddCard(ctx, req)
}

// GetBillingInformation returns billing information
func (ac *cloudWithRoles) GetBillingInformation(ctx context.Context, req *v1.EmptyRequest) (*v1.GetBillingInformationResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbRead)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.GetBillingInformation(ctx, req)
}

// SubmitUsageReports submits usage report for processing
func (ac *cloudWithRoles) SubmitUsageReports(ctx context.Context, req *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error) {
	return nil, trace.NotImplemented("SubmitUsageReports cannot be called via Auth Service.")
}

// ListInvoices lists tenant invoices
func (ac *cloudWithRoles) ListInvoices(ctx context.Context, req *v1.EmptyRequest) (*v1.ListInvoicesResponse, error) {
	err := ac.action(ctx, defaults.Namespace, services.KindBilling, services.VerbList)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return ac.plugin.cloudClient.ListInvoices(ctx, req)
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
