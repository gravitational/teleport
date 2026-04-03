package test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service"
	scimcommon "github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func enableOktaSCIMEntitlement(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OktaSCIM: {Enabled: true},
			},
		},
	})
}

func bindTestFn[A, B any](fn func(context.Context, A) (B, error), val A) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := fn(ctx, val)
		return err
	}
}

// TestSCIMServiceDeniesAccessToNonProxyUser tests that a non-proxy user always
// causes an AccessDenied error without touching any other resources.
func TestSCIMServiceDeniesAccessToNonProxyUser(t *testing.T) {
	enableOktaSCIMEntitlement(t)
	uut, _ := newTestService(t)

	nonProxyCtx := authz.ContextWithUser(
		context.Background(),
		authz.BuiltinRole{
			Role:     types.RoleOkta,
			Username: string(types.RoleOkta),
		})

	testFuncs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{
			name: "list",
			fn:   bindTestFn(uut.ListSCIMResources, &scimpb.ListSCIMResourcesRequest{}),
		}, {
			name: "get",
			fn:   bindTestFn(uut.GetSCIMResource, &scimpb.GetSCIMResourceRequest{}),
		}, {
			name: "create",
			fn:   bindTestFn(uut.CreateSCIMResource, &scimpb.CreateSCIMResourceRequest{}),
		}, {
			name: "update",
			fn:   bindTestFn(uut.UpdateSCIMResource, &scimpb.UpdateSCIMResourceRequest{}),
		}, {
			name: "delete",
			fn:   bindTestFn(uut.DeleteSCIMResource, &scimpb.DeleteSCIMResourceRequest{}),
		},
	}

	for _, f := range testFuncs {
		t.Run(f.name, func(t *testing.T) {
			// Given a context tagged with a non-proxy user, when I attempt to
			// invoke a public method on the SCIM server...
			err := f.fn(nonProxyCtx)

			// Expect that the operation fails with AccessDenied
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err),
				"Expected AccessDenied, got %q", err.Error())

			// Because the behavior of the mock services is to panic if any
			// unexpected calls are made to them, We also (implicitly) test the
			// expectation that the Service did not interact with any *other*
			// services before issuing the AccessDenied error.
		})
	}
}

// TestSCIMServiceFailsWithoutIGS asserts that the SCIM service will refuse to
// serve requests when IGS is disabled
func TestSCIMServiceFailsWithoutEntitlement(t *testing.T) {
	// Explicitly disable IGS
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OktaSCIM: {Enabled: false},
			},
		},
	})

	uut, fix := newTestService(t)

	testFuncs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{
			name: "list",
			fn:   bindTestFn(uut.ListSCIMResources, &scimpb.ListSCIMResourcesRequest{}),
		}, {
			name: "get",
			fn:   bindTestFn(uut.GetSCIMResource, &scimpb.GetSCIMResourceRequest{}),
		}, {
			name: "create",
			fn:   bindTestFn(uut.CreateSCIMResource, &scimpb.CreateSCIMResourceRequest{}),
		}, {
			name: "update",
			fn:   bindTestFn(uut.UpdateSCIMResource, &scimpb.UpdateSCIMResourceRequest{}),
		}, {
			name: "delete",
			fn:   bindTestFn(uut.DeleteSCIMResource, &scimpb.DeleteSCIMResourceRequest{}),
		},
	}

	for _, f := range testFuncs {
		t.Run(f.name, func(t *testing.T) {
			// Given a context tagged with a non-proxy user, when I attempt to
			// invoke a public method on the SCIM server...
			err := f.fn(fix.userCtx)

			// Expect that the operation fails with NotImplemented
			require.Error(t, err)
			require.True(t, trace.IsNotImplemented(err),
				"Expected NotImplemented, got %q", err.Error())

			// Because the behavior of the mock services is to panic if any
			// unexpected calls are made to them, We also (implicitly) test the
			// expectation that the Service did not interact with any *other*
			// services before issuing the NotImplemented error.
		})
	}
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext any = mock.MatchedBy(func(context.Context) bool { return true })

var anyResource any = mock.MatchedBy(func(*scimpb.Resource) bool { return true })

const (
	testPluginName          = "test"
	testPluginOrgUrl        = "https://mystery-machine.mockta.com"
	testSSOConnectorID      = "test-okta-integration"
	testPluginID            = "some-string-unique-to-the-plugin"
	testTokenSecret         = "somesortofbearertoken"
	testAuthHeader          = "Bearer " + testTokenSecret
	withSecrets             = true
	withoutSecrets          = false
	testUserExternalIDlabel = "mock-extrenal-id"
	testUserLabel           = "favouriteFruit"
	testUserLabelValue      = "nectarine"
)

var (
	hashedTokenSecret string
)

func init() {
	scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(testTokenSecret), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	hashedTokenSecret = string(scimTokenHash)
}

func mkTestPlugin() types.Plugin {
	return &types.PluginV1{
		Kind:    types.KindPlugin,
		SubKind: types.PluginSubkindAccess,
		Version: "abc123",
		Metadata: types.Metadata{
			Name:   testPluginName,
			Labels: map[string]string{"test": testPluginName},
		},
		// Spec is deliberately empty, apart from giving the plugin just enough
		// info to determine its type. The SCIM server should not touch *any*
		// plugin-type specific properties of the plugin. Only a type-specific
		// shim should is allowed to do that, and we'll handle that with a mock
		// shim for these tests.
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": testPluginID,
					},
				},
			},
		},
	}
}

func requireNotFound(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func requireAlreadyExists(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsAlreadyExists(err), "Expected AlreadyExists, got %s", err)
}

func requireBadParameter(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsBadParameter(err), "Expected BadParameter, got %s", err)
}

type authMock struct {
}

type mockChecker struct {
	services.AccessChecker
}

func (a *mockChecker) HasRole(role string) bool {
	return true
}

func (a authMock) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker:  &mockChecker{},
		Identity: authz.BuiltinRole{},
	}, nil
}

type pluginMock struct {
	plugin *types.PluginV1
}

func (p pluginMock) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	return p.plugin, nil
}

type userMock struct {
	scimcommon.Users
	users []*types.UserV2
}

func (u *userMock) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	return &userspb.ListUsersResponse{
		Users: u.users,
	}, nil
}

type credMock struct {
	scimcommon.Credentials
	creds []types.PluginStaticCredentials
}

func (c *credMock) GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	return c.creds, nil
}

func TestListSCIMResourcesUserPredicate(t *testing.T) {
	enableOktaSCIMEntitlement(t)
	plugin := &types.PluginV1{
		Metadata: types.Metadata{
			Name: testPluginName,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: testPluginOrgUrl,
					SyncSettings: &types.PluginOktaSyncSettings{
						SyncAccessLists: true,
						SsoConnectorId:  testSSOConnectorID,
					},
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": testPluginID,
					},
				},
			},
		},
	}

	pluginCreds := []types.PluginStaticCredentials{
		&types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: hashedTokenSecret,
				},
			},
		},
	}

	aliceUserCreateByOktaConnector := &types.UserV2{
		Metadata: types.Metadata{Name: "alice@example.com"},
		Spec: types.UserSpecV2{
			CreatedBy: types.CreatedBy{Connector: &types.ConnectorRef{ID: testSSOConnectorID}},
		},
	}

	bobUserCreateByNonOktaConnector := &types.UserV2{
		Metadata: types.Metadata{Name: "bob@example.com"},
		Spec: types.UserSpecV2{
			CreatedBy: types.CreatedBy{Connector: &types.ConnectorRef{ID: "some-other-connector"}},
		},
	}

	richardUserProvidedBySCIM := &types.UserV2{
		Metadata: types.Metadata{
			Name: "richard@example.com",
			Labels: map[string]string{
				teleport.OktaOrgURLLabel: testPluginOrgUrl,
				types.OriginLabel:        types.OriginOkta,
			},
		},
		Spec: types.UserSpecV2{
			CreatedBy: types.CreatedBy{Connector: &types.ConnectorRef{ID: "some-other-connector"}},
		},
	}

	users := []*types.UserV2{
		aliceUserCreateByOktaConnector,
		bobUserCreateByNonOktaConnector,
		richardUserProvidedBySCIM,
	}

	clock := clockwork.NewFakeClock()
	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	sut, err := service.NewService(&scimcommon.Config{
		Authorizer: &authMock{},
		Logger:     logtest.NewLogger(),
		AccessPoint: &serviceTestServices{
			Users:           &userMock{users: users},
			Roles:           &mockRoleService{},
			Identity:        &mockIdentityService{},
			Plugins:         &pluginMock{plugin: plugin},
			Credentials:     &credMock{creds: pluginCreds},
			JWTSigner:       &mockJwtSigner{},
			AccessLists:     &mockAccessListService{},
			Locks:           &mockLocksService{},
			AuthorityGetter: &mockAuthorityGetter{},
			Semaphores:      local.NewPresenceService(bk),
		},
		Backend: &testBackend{
			AccessLists: &mockAccessListService{},
			Assignments: &mockAssignmentsService{},
			Locks:       &mockLocksService{},
		},
		Clock:       clock,
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)

	cmpResourceID := cmp.Comparer(func(x, y *scimpb.Resource) bool {
		return x.Id == y.Id
	})

	t.Run("list user by filter should return SAML originated user", func(t *testing.T) {
		// alice user is SAML ephemeral users SCIM List call should  also list SAML users
		// if SAML from the user object and okta SCIM settings are the same.
		resp, err := sut.ListSCIMResources(ctx, &scimpb.ListSCIMResourcesRequest{
			Target: &scimpb.RequestTarget{
				Authorization: testAuthHeader,
				PluginId:      "okta",
				ResourceType:  "Users",
			},
			Page:   &scimpb.Page{StartIndex: 1, Count: 100},
			Filter: `userName eq "alice@example.com"`,
		})
		require.NoError(t, err)
		require.Len(t, resp.Resources, 1)

		want := []*scimpb.Resource{
			{Id: aliceUserCreateByOktaConnector.GetName()},
		}
		require.Empty(t, cmp.Diff(want, resp.Resources, cmpResourceID))
	})

	t.Run("list SCIM resource should return SCIM and SAML originated users", func(t *testing.T) {
		resp, err := sut.ListSCIMResources(ctx, &scimpb.ListSCIMResourcesRequest{
			Target: &scimpb.RequestTarget{
				Authorization: testAuthHeader,
				PluginId:      "okta",
				ResourceType:  "Users",
			},
			Page: &scimpb.Page{StartIndex: 1, Count: 100},
		})
		require.NoError(t, err)
		require.Len(t, resp.Resources, 2)

		want := []*scimpb.Resource{
			{Id: aliceUserCreateByOktaConnector.GetName()},
			{Id: richardUserProvidedBySCIM.GetName()},
		}
		require.Empty(t, cmp.Diff(want, resp.Resources, cmpResourceID))
	})
}
