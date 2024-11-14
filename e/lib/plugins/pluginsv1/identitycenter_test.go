package pluginsv1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	ictestenv "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestIdentityCenterResourceCleanup(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	suite := createSuite(t)
	ctx := context.Background()
	icServiceClient := ictestenv.CleanupTestClient{
		ICService:                suite.svc.authServer.IdentityCenter,
		ProvisioningStateService: suite.svc.authServer.ProvisioningStates,
		SAMLIdPService:           suite.svc.authServer.SAMLIdPServiceProviders,
		IntegrationService:       suite.svc.authServer.Integrations,
		AccessListService:        suite.svc.authServer.AccessLists,
		RoleService:              suite.svc.authServer.Access,
	}
	testData := ictestenv.NewDeletionData()
	ictestenv.CreateICResources(t, ctx, icServiceClient, testData, string(identitycenter.IdentityCenterDownstreamID))

	// test that identityCenterNeedsCleanup returns all the resource IDs that were created with
	// CreateICResources func and the ones we care about for Idenitity Center resource cleanup.
	resources, _, err := suite.svc.identityCenterNeedsCleanup(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, resources)
	resourceMap := make(map[string]struct{})
	for _, r := range resources {
		resourceMap[r.Name] = struct{}{}
	}

	for _, r := range testData.AccessLists {
		if r.WithICOrigin {
			_, ok := resourceMap[r.Name]
			require.True(t, ok, r.Name)
		}
	}

	for _, r := range testData.Roles {
		if r.WithICOrigin {
			_, ok := resourceMap[r.Name]
			require.True(t, ok, r.Name)
		}
	}

	for _, r := range testData.ICResource.Accounts {
		_, ok := resourceMap[r]
		require.True(t, ok, r)
	}
	for _, r := range testData.ICResource.PermissionSets {
		_, ok := resourceMap[r]
		require.True(t, ok, r)
	}
	for _, r := range testData.ICResource.PrincipalAsssignments {
		_, ok := resourceMap[r]
		require.True(t, ok, r)
	}
	for _, r := range testData.ICResource.AccountAssignments {
		_, ok := resourceMap[r]
		require.True(t, ok, r)
	}
	for _, r := range testData.ICResource.ProvisioningStates {
		// "u-" is prefixed for user provisioning state
		_, ok := resourceMap["u-"+r]
		require.True(t, ok, r)
	}

	err = suite.svc.cleanupAWSIdentityCenter(ctx, nil)
	require.NoError(t, err)
	resources, _, err = suite.svc.identityCenterNeedsCleanup(context.Background())
	require.NoError(t, err)
	require.Empty(t, resources)
}
