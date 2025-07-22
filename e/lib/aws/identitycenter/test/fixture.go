package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

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
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// Fixture holds resources for constructing and testing an
// IdentityCenter service
type Fixture struct {
	Ctx              context.Context
	Backend          backend.Backend
	Clock            clocki.FakeClock
	Auth             *auth.Server
	SCIMClient       scimsdk.Client
	ICClient         *icsdk.ClientMock
	PluginService    *local.PluginsService
	PluginStatusSink common.StatusSink
	Emitter          *eventstest.ChannelEmitter
	cleanup          func()
}

type CacheArgs struct {
	Started bool
}

type fixtureOptions struct {
	awsState      *icsdk.MockedAWSStateType
	authOptions   []auth.ServerOption
	clock         clockwork.Clock
	getStatusSink func(services.Plugins) common.StatusSink
}

type FixtureOption func(*fixtureOptions)

func WithAuthOption(authOpt auth.ServerOption) FixtureOption {
	return func(fixtureOpts *fixtureOptions) {
		fixtureOpts.authOptions = append(fixtureOpts.authOptions, authOpt)
	}
}

func WithStartedCache(fixtureOpts *fixtureOptions) {
	WithAuthOption(WithCache(CacheArgs{Started: true}))(fixtureOpts)
}

func WithUnstartedCache(fixtureOpts *fixtureOptions) {
	WithAuthOption(WithCache(CacheArgs{Started: false}))(fixtureOpts)
}

func WithCache(args CacheArgs) auth.ServerOption {
	return func(srv *auth.Server) error {
		return authtest.InitAuthCache(authtest.AuthCacheParams{
			AuthServer: srv,
			Unstarted:  !args.Started,
		})
	}
}

func WithClock(clock clockwork.Clock) FixtureOption {
	return func(fixtureOpts *fixtureOptions) {
		fixtureOpts.clock = clock
	}
}

func WithAWSState(state *icsdk.MockedAWSStateType) FixtureOption {
	return func(fixtureOpts *fixtureOptions) {
		fixtureOpts.awsState = state
	}
}

func WithStatusSink(s common.StatusSink) FixtureOption {
	return func(fixtureOpts *fixtureOptions) {
		fixtureOpts.getStatusSink = func(services.Plugins) common.StatusSink { return s }
	}
}

func WithWriteThroughStatusSink(fixtureOpts *fixtureOptions) {
	fixtureOpts.getStatusSink = func(p services.Plugins) common.StatusSink {
		return &writeThroughStatusSink{pluginsService: p}
	}
}

func NewFixture(t *testing.T, opts ...FixtureOption) *Fixture {
	args := fixtureOptions{
		awsState:      nil, // use default mock state by default
		clock:         clockwork.NewFakeClock(),
		getStatusSink: func(services.Plugins) common.StatusSink { return &integration.FakeStatusSink{} },
	}
	for _, optFn := range opts {
		optFn(&args)
	}
	if args.awsState == nil {
		defaultState := icsdk.NewMockedAWSState()
		args.awsState = &defaultState
	}

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})

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

	authOpts := append(args.authOptions, authtest.WithClock(clock))
	auth, err := auth.NewServer(&auth.InitConfig{
		Authority:              authority.New(),
		Backend:                backend,
		ClusterName:            clusterName,
		SkipPeriodicOperations: true,
		VersionStorage:         authtest.NewFakeTeleportVersion(),
	}, authOpts...)
	require.NoError(t, err, "creating Auth server")
	cleanup := sync.OnceFunc(func() { require.NoError(t, auth.Close()) })
	t.Cleanup(cleanup)

	icClient := NewUnifiedMockClient(*args.awsState)

	pluginService := local.NewPluginsService(backend)

	statusSink := args.getStatusSink(auth.Plugins)

	fixture := &Fixture{
		Ctx:              ctx,
		Backend:          backend,
		Clock:            clock,
		Auth:             auth,
		SCIMClient:       icClient.ViaSCIM(),
		ICClient:         icClient.ViaAPI(),
		PluginService:    pluginService,
		PluginStatusSink: statusSink,
		Emitter:          eventstest.NewChannelEmitter(10),
		cleanup:          cleanup,
	}

	return fixture
}

func (f *Fixture) ResetEmitter() {
	f.Emitter = eventstest.NewChannelEmitter(10)
}

type ICOption func(*types.PluginV1)

func WithoutImport(plugin *types.PluginV1) {
	details := plugin.Status.GetAwsIc()
	if details == nil {
		details = &types.PluginAWSICStatusV1{}
		plugin.Status.Details = &types.PluginStatusV1_AwsIc{AwsIc: details}
	}

	details.GroupImportStatus = &types.AWSICGroupImportStatus{
		StatusCode: types.AWSICGroupImportStatusCode_DONE,
	}
}

func WithGroupFilters(filters icfilters.Filters) ICOption {
	return func(plugin *types.PluginV1) {
		settings := plugin.Spec.GetAwsIc()
		settings.GroupSyncFilters = filters
	}
}

// Cleanup manually cleans up the fixture and shuts down any started services.
// For use in situations where can't or don't want to rely on the automatic
// end-of-tes cleanup, for example when the [Fixture] is created in a
// [testing/synctest] bubble.
func (f *Fixture) Cleanup() {
	f.cleanup()
}

func (f *Fixture) CreatePluginResource(t *testing.T, options ...ICOption) {
	createPluginReq := NewPluginV1CreateRequest("test-oidc", "test-saml")

	initialPlugin := types.NewPluginV1(createPluginReq.GetPlugin().GetMetadata(), createPluginReq.GetPlugin().Spec, nil)
	for _, applyOption := range options {
		applyOption(initialPlugin)
	}
	require.NoError(t, f.PluginService.CreatePlugin(context.Background(), initialPlugin))
}

func (f *Fixture) MustGetPluginResource(t *testing.T) *types.PluginV1 {
	p, err := f.GetPluginResource()
	require.NoError(t, err)
	return p
}

func (f *Fixture) GetPluginResource() (*types.PluginV1, error) {
	p, err := f.PluginService.GetPlugin(context.Background(), types.PluginTypeAWSIdentityCenter, false)
	if err != nil {
		return nil, err
	}
	return p.(*types.PluginV1), nil
}

// ICCreatedData defines resource names for each test resource type.
type ICResource struct {
	Accounts             []string
	AccountAssignments   []string
	PrincipalAssignments []string
	ProvisioningStates   []string
	PermissionSets       []string
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
	createICPrincipleAssignment(t, ctx, client.ICService, testData.ICResource.PrincipalAssignments)
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
			newRole.SetSubKind(types.KindIdentityCenter)
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
			Accounts:             []string{"account1", "account2"},
			AccountAssignments:   []string{"assignment1", "assignment2"},
			PrincipalAssignments: []string{"passignment1", "passignment2"},
			ProvisioningStates:   []string{"pstate1", "pstate2"},
			PermissionSets:       []string{"pset1", "pset2"},
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
			RoleARN:  "arn:aws:iam::123456789012:role/DevTeams",
			Audience: commontypes.OriginAWSIdentityCenter,
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
							BaseUrl: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2",
						},
					},
				},
			},
		},
	}
}

// writeThroughStatusSink implements a simple [common.StatusSink] that writes
// back to the IC plugin resource. Attempting to create a production status
// sink here results in a circular import, so we use this lightweight shim
// instead.
type writeThroughStatusSink struct {
	pluginsService services.Plugins
}

// Emit sends the plugin status, applying custom logic for AWS IC plugin.
// If the status detail field is nil, an existing detail status will be applied.
func (s *writeThroughStatusSink) Emit(ctx context.Context, status types.PluginStatus) error {
	return s.pluginsService.SetPluginStatus(ctx, types.PluginTypeAWSIdentityCenter, status)
}
