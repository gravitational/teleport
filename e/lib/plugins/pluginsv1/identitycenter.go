package pluginsv1

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// identityCenterNeedsCleanup will return an error if Teleport cluster has resources
// created by the previous installation of Identity Center plugin.
// For the create plugin request, we perform default cleanup. And any resource returned
// by this function  during create plugin request means the cleanup
// terminally failed and requires manual admin intervention.
func (s *Service) identityCenterNeedsCleanup(ctx context.Context) ([]*types.ResourceID, bool, error) {
	active, err := s.isPluginOfTypeActive(ctx, types.PluginTypeAWSIdentityCenter)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	var out []*types.ResourceID

	icResources, err := listAllIdentityCenterResources(ctx, s.authServer.IdentityCenter, s.authServer.ProvisioningStates)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	out = append(out, icResources...)

	icCreatedAccessLists, err := identitycenter.ListICOriginatedAccessLists(ctx, s.authServer.AccessLists)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, r := range icCreatedAccessLists {
		out = append(out, &types.ResourceID{Kind: types.KindAccessList, Name: r.GetMetadata().Name})
	}

	icCreatedRoles, err := identitycenter.ListICOriginatedRoles(ctx, s.authServer.Access)
	if err != nil {
		return nil, active, trace.Wrap(err)
	}
	for _, r := range icCreatedRoles {
		out = append(out, &types.ResourceID{Kind: types.KindRole, Name: r.GetMetadata().Name})
	}

	return out, active, nil
}

// cleanupAWSIdentityCenter deletes resources created by the Identity Center plugin.
func (s *Service) cleanupAWSIdentityCenter(ctx context.Context, deletedPlugin *types.PluginV1) error {
	active, err := s.isPluginOfTypeActive(ctx, types.PluginTypeAWSIdentityCenter)
	if err != nil {
		return trace.Wrap(err)
	}
	if active {
		return trace.CompareFailed("AWS IAM Identity Center plugin is active, can't cleanup")
	}

	s.logger.InfoContext(ctx, "Cleaning up AWS IAM Identity Center plugin resources")
	var errs []error

	// orphaned SAML service provider and AWS OIDC integration is detected early during plugin enrollment
	// so it is skipped if this function is invoked during create plugin request. We also cannot
	// delete SAML service provider and integration while the plugin is still active.
	if deletedPlugin != nil {
		awsIC := deletedPlugin.Spec.GetAwsIc()
		if awsIC == nil {
			return trace.BadParameter("missing PluginAWSICSettings")
		}
		serviceProviderName := awsIC.SamlIdpServiceProviderName
		if err := s.authServer.SAMLIdPServiceProviders.DeleteSAMLIdPServiceProvider(ctx, serviceProviderName); err != nil && !trace.IsNotFound(err) {
			errs = append(errs, trace.Errorf(`Error %v. To manually remove the service provider, run the command: 'tctl rm saml_idp_service_provider/%s'`,
				err.Error(),
				serviceProviderName,
			))
		}

		if err := s.authServer.Integrations.DeleteIntegration(ctx, awsIC.IntegrationName); err != nil && !trace.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	// delete core Identity Center resources created by the import and provisioning service.
	if err := deleteIdentityCenterResources(ctx, s.authServer.IdentityCenter, s.authServer.ProvisioningStates); err != nil {
		errs = append(errs, err)
	}

	// delete access list and its members imported by the plugin.
	if err := deleteIdentityCenterOriginatedAccessLists(ctx, s.authServer.AccessLists); err != nil {
		errs = append(errs, err)
	}

	// delete roles created for permission assignments.
	if err := deleteIdentityCenterOriginatedRoles(ctx, s.authServer.Access); err != nil {
		errs = append(errs, err)
	}

	return trace.NewAggregate(errs...)
}

var identityCenterDeleteResourceKinds = []string{
	types.KindIdentityCenter,
	types.KindSAMLIdPServiceProvider,
	types.KindIntegration,
	types.KindAccessList,
	types.KindRole,
}

func checkIdentityCenterResourceDeleteAccess(authCtx *authz.Context) error {
	for _, k := range identityCenterDeleteResourceKinds {
		if err := authCtx.CheckAccessToKind(k, types.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func listAllIdentityCenterResources(ctx context.Context, icService services.IdentityCenter, provisioningService services.ProvisioningStates) ([]*types.ResourceID, error) {
	var allResources []*types.ResourceID

	accounts := stream.FilterMap(clientutils.Resources(ctx, icService.ListIdentityCenterAccounts2),
		func(a *identitycenterv1.Account) (*types.ResourceID, bool) {
			return &types.ResourceID{Kind: types.KindIdentityCenterAccount, Name: a.GetMetadata().GetName()}, true
		})

	assignments := stream.FilterMap(clientutils.Resources(ctx, icService.ListIdentityCenterAccountAssignments),
		func(a *identitycenterv1.AccountAssignment) (*types.ResourceID, bool) {
			return &types.ResourceID{Kind: types.KindIdentityCenterAccountAssignment, Name: a.GetMetadata().GetName()}, true
		})

	principalAssignments := stream.FilterMap(clientutils.Resources(ctx, icService.ListPrincipalAssignments2),
		func(a *identitycenterv1.PrincipalAssignment) (*types.ResourceID, bool) {
			return &types.ResourceID{Kind: types.KindIdentityCenterPrincipalAssignment, Name: a.GetMetadata().GetName()}, true
		})

	permissionSets := stream.FilterMap(clientutils.Resources(ctx, icService.ListPermissionSets2),
		func(ps *identitycenterv1.PermissionSet) (*types.ResourceID, bool) {
			return &types.ResourceID{Kind: types.KindIdentityCenterPermissionSet, Name: ps.GetMetadata().GetName()}, true
		})

	provisioningStates := stream.FilterMap(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, page string) ([]*provisioningv1.PrincipalState, string, error) {
		return provisioningService.ListProvisioningStates2(ctx, identitycenter.IdentityCenterDownstreamID, pageSize, page)
	}), func(ps *provisioningv1.PrincipalState) (*types.ResourceID, bool) {
		return &types.ResourceID{Kind: types.KindProvisioningPrincipalState, Name: ps.GetMetadata().GetName()}, true
	})

	allResources, err := stream.Collect(stream.Chain(accounts, assignments, principalAssignments, permissionSets, provisioningStates))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return allResources, nil
}

func deleteIdentityCenterResources(ctx context.Context, icService services.IdentityCenter, provisioningService services.ProvisioningStates) error {
	var errs []error
	if err := icService.DeleteAllIdentityCenterAccounts(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := icService.DeleteAllAccountAssignments(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := icService.DeleteAllPrincipalAssignments(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := icService.DeleteAllPermissionSets(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := provisioningService.DeleteDownstreamProvisioningStates(ctx, identitycenter.IdentityCenterDownstreamID); err != nil {
		errs = append(errs, err)
	}

	return trace.NewAggregate(errs...)
}

func deleteIdentityCenterOriginatedAccessLists(ctx context.Context, accessListClient services.AccessLists) error {
	var errs []error
	icCreatedAccessLists, err := identitycenter.ListICOriginatedAccessLists(ctx, accessListClient)
	if err != nil {
		errs = append(errs, err)
	}
	// only collect failed access list names to reduce size of the returned error message.
	var failedDeletionLists []string
	for _, a := range icCreatedAccessLists {
		if err := accessListClient.DeleteAccessList(ctx, a.GetName()); err != nil {
			failedDeletionLists = append(failedDeletionLists, a.GetName())
		}
	}
	if len(failedDeletionLists) > 0 {
		errs = append(errs, fmt.Errorf("failed to delete access lists: %q", strings.Join(failedDeletionLists[:], ",")))
	}

	return trace.NewAggregate(errs...)
}

func deleteIdentityCenterOriginatedRoles(ctx context.Context, roleService services.Access) error {
	var errs []error
	icCreatedRoles, err := identitycenter.ListICOriginatedRoles(ctx, roleService)
	if err != nil {
		errs = append(errs, err)
	}
	// only collect failed roles names to reduce size of the returned error message.
	var failedDeletionRoles []string
	for _, r := range icCreatedRoles {
		if err := roleService.DeleteRole(ctx, r.GetName()); err != nil {
			failedDeletionRoles = append(failedDeletionRoles, r.GetName())
		}
	}
	if len(failedDeletionRoles) > 0 {
		errs = append(errs, fmt.Errorf("failed to delete roles: %q", strings.Join(failedDeletionRoles[:], ",")))
	}

	return trace.NewAggregate(errs...)
}
