package test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	commontypes "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/samlsp"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	samlidptestenv "github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/accesspoint"
	"github.com/gravitational/teleport/lib/auth/authclient"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// Fixture holds resources for constructing and testing an
// IdentityCenter service
type Fixture struct {
	Ctx              context.Context
	Backend          backend.Backend
	Clock            clockwork.FakeClock
	Auth             *auth.Server
	SCIMClient       *scimsdk.ClientMock
	ICClient         *icsdk.ClientMock
	PluginService    *local.PluginsService
	PluginStatusSink common.StatusSink
}

type CacheArgs struct {
	Started bool
}

func initCache(srv *auth.Server, args CacheArgs) error {
	// TODO: See if we can trim the requirements for the cache down a bit.

	svces := srv.Services
	var err error
	srv.Cache, err = accesspoint.NewCache(accesspoint.Config{
		Context:      srv.CloseContext(),
		Setup:        cache.ForAuth,
		CacheName:    []string{teleport.ComponentAuth},
		EventsSystem: true,
		Unstarted:    !args.Started,

		Access:                  svces.Access,
		AccessLists:             svces.AccessLists,
		AccessMonitoringRules:   svces.AccessMonitoringRules,
		AppSession:              svces.Identity,
		Apps:                    svces.Apps,
		ClusterConfig:           svces.ClusterConfiguration,
		AutoUpdateService:       svces.AutoUpdateService,
		CrownJewels:             svces.CrownJewels,
		DatabaseObjects:         svces.DatabaseObjects,
		DatabaseServices:        svces.DatabaseServices,
		Databases:               svces.Databases,
		DiscoveryConfigs:        svces.DiscoveryConfigs,
		DynamicAccess:           svces.DynamicAccessExt,
		Events:                  svces.Events,
		IdentityCenter:          svces.IdentityCenter,
		Integrations:            svces.Integrations,
		KubeWaitingContainers:   svces.KubeWaitingContainer,
		Kubernetes:              svces.Kubernetes,
		Notifications:           svces.Notifications,
		Okta:                    svces.Okta,
		Presence:                svces.PresenceInternal,
		Provisioner:             svces.Provisioner,
		ProvisioningStates:      svces.ProvisioningStates,
		Restrictions:            svces.Restrictions,
		SAMLIdPServiceProviders: svces.SAMLIdPServiceProviders,
		SAMLIdPSession:          svces.Identity,
		SecReports:              svces.SecReports,
		SnowflakeSession:        svces.Identity,
		SPIFFEFederations:       svces.SPIFFEFederations,
		StaticHostUsers:         svces.StaticHostUser,
		Trust:                   svces.TrustInternal,
		UserGroups:              svces.UserGroups,
		UserTasks:               svces.UserTasks,
		UserLoginStates:         svces.UserLoginStates,
		Users:                   svces.Identity,
		WebSession:              svces.Identity.WebSessions(),
		WebToken:                svces.WebTokens(),
		WindowsDesktops:         svces.WindowsDesktops,
		DynamicWindowsDesktops:  svces.DynamicWindowsDesktops,
		PluginStaticCredentials: svces.PluginStaticCredentials,
		GitServers:              svces.GitServers,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func WithCache(args CacheArgs) func(*auth.Server) error {
	return func(srv *auth.Server) error { return initCache(srv, args) }
}

func NewFixture(t *testing.T, opts ...auth.ServerOption) *Fixture {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: t.Name(),
	})
	require.NoError(t, err)

	opts = append(opts, auth.WithClock(clock))
	auth, err := auth.NewServer(&auth.InitConfig{
		Authority:              authority.New(),
		Backend:                backend,
		ClusterName:            clusterName,
		SkipPeriodicOperations: true,
		VersionStorage:         auth.NewFakeTeleportVersion(),
	}, opts...)
	require.NoError(t, err, "creating Auth server")
	t.Cleanup(func() { require.NoError(t, auth.Close()) })

	scimClient := scimsdk.NewSCIMClientMock()
	ICClient := icsdk.NewClientMock(nil /* custom mock data */)

	pluginService := local.NewPluginsService(backend)

	fixture := &Fixture{
		Ctx:              ctx,
		Backend:          backend,
		Clock:            clock,
		Auth:             auth,
		SCIMClient:       scimClient,
		ICClient:         ICClient,
		PluginService:    pluginService,
		PluginStatusSink: &integration.FakeStatusSink{},
	}

	return fixture
}

// ICCreatedData defines resource names for each test resource type.
type ICResource struct {
	Accounts              []string
	AccountAssignments    []string
	PrincipalAsssignments []string
	ProvisioningStates    []string
	PermissionSets        []string
}

// ResourceWithICOrigin defines a resource name that may optionally be
// of Identity Center origin.
type ResourceWithICOrigin struct {
	Name         string
	WithICOrigin bool
}

// CleanupTestClient defines services that is used to test
// plugin delete and resource cleanup.
type CleanupTestClient struct {
	ICService                services.IdentityCenter
	ProvisioningStateService services.ProvisioningStates
	SAMLIdPService           services.SAMLIdPServiceProviders
	IntegrationService       services.Integrations
	AccessListService        services.AccessLists
	RoleService              services.Access
}

// AccessListMembers defines Access List members.
type AccessListMembers struct {
	MemberName     string
	AccessListName string
	IsICOriginated bool
}

// ICDeletionTestData defines all the Identity Center originated
// resources that needs cleanup.
type ICDeletionTestData struct {
	ICResource        ICResource
	AccessLists       []ResourceWithICOrigin
	AccessListMembers []AccessListMembers
	Roles             []ResourceWithICOrigin
}

// CreateICResources creates all the resources that would be created by the Identity Center service.
func CreateICResources(t *testing.T, ctx context.Context, client CleanupTestClient, testData ICDeletionTestData, downstreamID string) {
	t.Helper()

	createICAccount(t, ctx, client.ICService, testData.ICResource.Accounts)
	createICAccountAssignment(t, ctx, client.ICService, testData.ICResource.AccountAssignments)
	createICPrincipleAssignment(t, ctx, client.ICService, testData.ICResource.PrincipalAsssignments)
	createPermissionSets(t, ctx, client.ICService, testData.ICResource.PermissionSets)
	createProvisioningState(t, ctx, client.ProvisioningStateService, testData.ICResource.ProvisioningStates, downstreamID)

	createAccessLists(t, ctx, client.AccessListService, testData.AccessLists)
	createAccessListsMembers(t, ctx, client.AccessListService, testData.AccessListMembers)
	createRoles(t, ctx, client.RoleService, testData.Roles)
}

func createRoles(t *testing.T, ctx context.Context, roleService services.Access, roleRequest []ResourceWithICOrigin) {
	t.Helper()
	for _, r := range roleRequest {
		newRole, err := types.NewRole(r.Name, types.RoleSpecV6{})
		require.NoError(t, err)
		if r.WithICOrigin {
			newRole.SetOrigin(commontypes.OriginAWSIdentityCenter)
		}
		_, err = roleService.UpsertRole(ctx, newRole)
		require.NoError(t, err)
	}
}

func createAccessLists(t *testing.T, ctx context.Context, accessListClient services.AccessLists, accessListRequest []ResourceWithICOrigin) {
	t.Helper()
	for _, a := range accessListRequest {
		acl, err := accesslist.NewAccessList(
			header.Metadata{
				Name: a.Name,
			},
			accesslist.Spec{
				Title:  a.Name,
				Owners: []accesslist.Owner{{Name: "foo"}},
				Grants: accesslist.Grants{
					Roles: []string{"role1"},
				},
			},
		)
		require.NoError(t, err)
		if a.WithICOrigin {
			acl.SetOrigin(commontypes.OriginAWSIdentityCenter)
		}
		_, err = accessListClient.UpsertAccessList(ctx, acl)
		require.NoError(t, err)
	}
}

func createAccessListsMembers(t *testing.T, ctx context.Context, accessListClient services.AccessLists, accessListMembersRequest []AccessListMembers) {
	t.Helper()
	for _, a := range accessListMembersRequest {
		aclMember, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: a.MemberName,
			},
			accesslist.AccessListMemberSpec{
				AccessList: a.AccessListName,
				Name:       a.MemberName,
				Joined:     time.Now(),
				AddedBy:    types.SystemResource,
			},
		)
		require.NoError(t, err)
		_, err = accessListClient.UpsertAccessListMember(ctx, aclMember)
		require.NoError(t, err)
	}
}

func createICAccount(t *testing.T, ctx context.Context, icService services.IdentityCenter, accountNames []string) {
	t.Helper()
	for _, n := range accountNames {
		_, err := icService.CreateIdentityCenterAccount(ctx, services.IdentityCenterAccount{
			Account: &identitycenterv1.Account{
				Kind:     types.KindIdentityCenterAccount,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: n},
				Spec: &identitycenterv1.AccountSpec{
					Id:          "aws-account-id-" + n,
					Arn:         fmt.Sprintf("arn:aws:sso::%s:", n),
					Description: "Test account " + n,
				},
			},
		})
		require.NoError(t, err)
	}
}

func createICAccountAssignment(t *testing.T, ctx context.Context, icService services.IdentityCenter, accountAssignmentNames []string) {
	t.Helper()
	for _, n := range accountAssignmentNames {
		_, err := icService.CreateAccountAssignment(ctx, services.IdentityCenterAccountAssignment{
			AccountAssignment: &identitycenterv1.AccountAssignment{
				Kind:     types.KindIdentityCenterAccountAssignment,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: n},
				Spec: &identitycenterv1.AccountAssignmentSpec{
					Display:     "Some-Permission-set on Some-AWS-account",
					AccountName: "Some Account Name",
					AccountId:   "some account id",
				},
			},
		})
		require.NoError(t, err)
	}
}

func createICPrincipleAssignment(t *testing.T, ctx context.Context, icService services.IdentityCenter, principalAssignmentNames []string) {
	t.Helper()
	for _, n := range principalAssignmentNames {
		_, err := icService.CreatePrincipalAssignment(ctx, &identitycenterv1.PrincipalAssignment{
			Kind:     types.KindIdentityCenterPrincipalAssignment,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: n},
			Spec: &identitycenterv1.PrincipalAssignmentSpec{
				PrincipalType:    identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
				PrincipalId:      n,
				ExternalIdSource: "scim",
				ExternalId:       "some external id",
			},
			Status: &identitycenterv1.PrincipalAssignmentStatus{
				ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			},
		})
		require.NoError(t, err)
	}
}

func createProvisioningState(t *testing.T, ctx context.Context, client services.ProvisioningStates, stateNames []string, downstreamID string) {
	t.Helper()
	for _, n := range stateNames {
		_, err := client.CreateProvisioningState(ctx, &provisioningv1.PrincipalState{
			Kind: types.KindProvisioningPrincipalState,
			Metadata: &headerv1.Metadata{
				Name: "u-" + n,
			},
			Spec: &provisioningv1.PrincipalStateSpec{
				DownstreamId:  downstreamID,
				PrincipalType: provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
				PrincipalId:   n,
			},
			Status: &provisioningv1.PrincipalStateStatus{
				ProvisioningState: provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
			},
		})
		require.NoError(t, err)
	}
}

func createPermissionSets(t *testing.T, ctx context.Context, client services.IdentityCenter, names []string) {
	t.Helper()
	for _, n := range names {
		_, err := client.CreatePermissionSet(ctx, &identitycenterv1.PermissionSet{
			Kind:     types.KindIdentityCenterPermissionSet,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: n},
			Spec: &identitycenterv1.PermissionSetSpec{
				Arn:         fmt.Sprintf("arn:aws:sso:::permissionSet/ic-instance/%s", n),
				Name:        "aws-permission-set-" + n,
				Description: "Test permission set " + n,
			},
		})
		require.NoError(t, err)
	}
}

// CheckCleanupArgs defines argument for CheckAllICResourcesAreConditionallyDeleted.
type CheckCleanupArgs struct {
	SAMLlServiceProviderName    string
	IntegrationName             string
	IsCreateRequest             bool
	TestData                    ICDeletionTestData
	TestClient                  CleanupTestClient
	DownstreamID                string
	ListICOriginatedAccessLists func(ctx context.Context, service services.AccessLists) (map[string]*accesslist.AccessList, error)
	ListICOriginatedRoles       func(ctx context.Context, service services.Access) ([]*types.RoleV6, error)
}

// CheckAllICResourcesAreConditionallyDeleted checks the following:
// IC originated resource, account, account assignment, principal assignment
// - IC originated access list
// - access list members associated with the list
// - IC originated roles
func CheckAllICResourcesAreConditionallyDeleted(t *testing.T, ctx context.Context, args CheckCleanupArgs) {
	if !args.IsCreateRequest {
		sp, err := args.TestClient.SAMLIdPService.GetSAMLIdPServiceProvider(ctx, args.SAMLlServiceProviderName)
		require.ErrorContains(t, err, "doesn't exist")
		require.Nil(t, sp)

		i, err := args.TestClient.IntegrationService.GetIntegration(ctx, args.IntegrationName)
		require.ErrorContains(t, err, "doesn't exist")
		require.Nil(t, i)
	}
	accountFromDB, _, err := args.TestClient.ICService.ListIdentityCenterAccounts(ctx, apidefaults.DefaultChunkSize, &pagination.PageRequestToken{})
	require.NoError(t, err)
	require.Empty(t, accountFromDB)

	accountAssignmentFromDB, _, err := args.TestClient.ICService.ListAccountAssignments(ctx, apidefaults.DefaultChunkSize, &pagination.PageRequestToken{})
	require.NoError(t, err)
	require.Empty(t, accountAssignmentFromDB)

	principalAssignmentFromDB, _, err := args.TestClient.ICService.ListPrincipalAssignments(ctx, apidefaults.DefaultChunkSize, &pagination.PageRequestToken{})
	require.NoError(t, err)
	require.Empty(t, principalAssignmentFromDB)

	permSetsFromDB, _, err := args.TestClient.ICService.ListPermissionSets(ctx, apidefaults.DefaultChunkSize, &pagination.PageRequestToken{})
	require.NoError(t, err)
	require.Empty(t, permSetsFromDB)

	provisioningStateFromDB, _, err := args.TestClient.ProvisioningStateService.ListProvisioningStates(ctx, services.DownstreamID(args.DownstreamID), apidefaults.DefaultChunkSize, &pagination.PageRequestToken{})
	require.NoError(t, err)
	require.Empty(t, provisioningStateFromDB)

	icOriginatedAccessLists, err := args.ListICOriginatedAccessLists(ctx, args.TestClient.AccessListService)
	require.NoError(t, err)
	require.Empty(t, icOriginatedAccessLists)

	for _, am := range args.TestData.AccessListMembers {
		members, _, err := args.TestClient.AccessListService.ListAccessListMembers(ctx, am.AccessListName, apidefaults.DefaultChunkSize, "")
		if am.IsICOriginated {
			// ListAccessListMembers returns an opaque access denied error if access list does not exist.
			if !errors.Is(err, trace.AccessDenied("access denied")) {
				require.NoError(t, err)
			}
			require.Empty(t, icOriginatedAccessLists)
		} else {
			require.NoError(t, err)
			require.NotEmpty(t, members)
		}
	}

	icOriginatedRoles, err := args.ListICOriginatedRoles(ctx, args.TestClient.RoleService)
	require.NoError(t, err)
	require.Empty(t, icOriginatedRoles)

	// check that non-ic originated access list and roles are preserved.
	nonICOriginatedList, err := args.TestClient.AccessListService.GetAccessList(ctx, "nonICOriginatedList")
	require.NoError(t, err)
	require.Equal(t, "nonICOriginatedList", nonICOriginatedList.GetName())

	nonICOriginatedRole, err := args.TestClient.RoleService.GetRole(ctx, "nonICOriginatedRole")
	require.NoError(t, err)
	require.Equal(t, "nonICOriginatedRole", nonICOriginatedRole.GetName())
}

// NewDeletionData returns fake resource names to test Identity Center plugin deletion.
func NewDeletionData() ICDeletionTestData {
	return ICDeletionTestData{
		ICResource: ICResource{
			Accounts:              []string{"account1", "account2"},
			AccountAssignments:    []string{"assignment1", "assignment2"},
			PrincipalAsssignments: []string{"passignment1", "passignment2"},
			ProvisioningStates:    []string{"pstate1", "pstate2"},
			PermissionSets:        []string{"pset1", "pset2"},
		},
		AccessLists: []ResourceWithICOrigin{
			{Name: "nonICOriginatedList", WithICOrigin: false},
			{Name: "alist2", WithICOrigin: true},
			{Name: "alist3", WithICOrigin: true},
		},
		AccessListMembers: []AccessListMembers{
			{MemberName: "member1", AccessListName: "nonICOriginatedList", IsICOriginated: false},
			{MemberName: "member2", AccessListName: "alist2", IsICOriginated: true},
			{MemberName: "member3", AccessListName: "alist3", IsICOriginated: true},
		},
		Roles: []ResourceWithICOrigin{
			{Name: "nonICOriginatedRole", WithICOrigin: false},
			{Name: "role2", WithICOrigin: true},
			{Name: "role3", WithICOrigin: true},
		},
	}
}

// CreateSAMLServiceProvider creates SAML service provider named "ic-saml-sp".
func CreateSAMLServiceProvider(t *testing.T, ctx context.Context, authClient authclient.ClientI, name string) {
	t.Helper()
	sp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: commontypes.OriginAWSIdentityCenter,
			},
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: samlidptestenv.NewTestEntityDescriptor(name, fmt.Sprintf("https://%s/acs", name)),
			Preset:           samlsp.AWSIdentityCenter,
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)
}

// CreateAWSICOIDCIntegration creates OIDC integration.
func CreateAWSOIDCIntegration(t *testing.T, ctx context.Context, authClient authclient.ClientI, name string) {
	t.Helper()
	awsOIDCIg, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: name},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/DevTeams",
			IssuerS3URI: "s3://my-bucket/my-prefix",
			Audience:    commontypes.OriginAWSIdentityCenter,
		},
	)
	require.NoError(t, err)
	_, err = authClient.CreateIntegration(ctx, awsOIDCIg)
	require.NoError(t, err)
}

// NewPluginV1CreateRequest returns a new CreatePluginRequest.
func NewPluginV1CreateRequest(integrationName, samlServiceProviderName string) *pluginspb.CreatePluginRequest {
	return &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: types.PluginTypeAWSIdentityCenter,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_AwsIc{
					AwsIc: &types.PluginAWSICSettings{
						IntegrationName:            integrationName,
						SamlIdpServiceProviderName: samlServiceProviderName,
						Region:                     "ca-central-1",
						Arn:                        "arn:aws:sso:::instance/ssoins-8893885e0d4lllka",
						AccessListDefaultOwners:    []string{"foo"},
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: "https://example.com",
						},
					},
				},
			},
		},
	}
}
