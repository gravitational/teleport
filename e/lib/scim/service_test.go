package scim

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

type builtinRoleAuthorizer struct{}

func (builtinRoleAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	userI, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if role, ok := userI.(authz.BuiltinRole); ok {
		return authz.ContextForBuiltinRole(role, nil)
	}
	return nil, trace.AccessDenied("nope")
}

type testFixture struct {
	users   mockUserService
	locks   mockLocksService
	plugins mockPluginsService
	creds   mockCredentialsService
	shim    *mockProviderShim
	clock   clockwork.FakeClock
	userCtx context.Context
}

type expectationAsserter interface {
	AssertExpectations(mock.TestingT) bool
}

func (tf *testFixture) AssertExpectations(t *testing.T) {
	for _, m := range []expectationAsserter{&tf.users, &tf.locks, &tf.plugins, &tf.creds, tf.shim} {
		if m != nil {
			m.AssertExpectations(t)
		}
	}
}

func newTestService(t *testing.T) (*Service, *testFixture) {
	fix := &testFixture{
		clock: clockwork.NewFakeClock(),
		userCtx: authz.ContextWithUser(
			context.Background(),
			authz.BuiltinRole{
				Role:     types.RoleProxy,
				Username: string(types.RoleProxy),
			}),
	}

	scimSvc, err := NewService(&Config{
		Authorizer:         builtinRoleAuthorizer{},
		UsersService:       &fix.users,
		LocksService:       &fix.locks,
		PluginsService:     &fix.plugins,
		CredentialsService: &fix.creds,
		Clock:              fix.clock,
	})
	require.NoError(t, err, "creating test harness")

	// patch the services shim factory map to return our mock shim when asked to
	// create one for the test plugin type
	fix.shim = &mockProviderShim{}
	t.Cleanup(func() {
		// No sense in cluttering the output with missed expectations if the
		// test has already failed.
		if t.Failed() {
			return
		}
		fix.AssertExpectations(t)
	})

	scimSvc.shimFactories[testPluginType] = func(context.Context, types.Plugin, *Service) (providerShim, error) {
		return fix.shim, nil
	}
	t.Cleanup(func() { delete(scimSvc.shimFactories, testPluginType) })

	return scimSvc, fix
}

// enableIGS configures the system modules to allow IGS features for tge life of
// the supplied test.
func enableIGS(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: true,
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
	enableIGS(t)
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
func TestSCIMServiceFailsWithoutIGS(t *testing.T) {
	// Explicitly disable IGS
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: false,
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

func isTestPluginUser(_ context.Context, u types.User) bool {
	l, _ := u.GetLabel(testUserLabel)
	return l == testUserLabelValue
}

func testUserToResource(_ context.Context, user types.User) (*scimpb.Resource, error) {
	attribs := map[string]any{}
	for k, vs := range user.GetTraits() {
		attribs[k] = vs[0]
	}

	attribsStruct, err := structpb.NewStruct(attribs)
	if err != nil {
		return nil, err
	}

	externalID, _ := user.GetLabel(testUserExternalIDlabel)
	result := &scimpb.Resource{
		Id:         user.GetName(),
		ExternalId: externalID,
		Meta: &scimpb.Meta{
			Created: timestamppb.New(user.GetCreatedBy().Time),
			Version: user.GetRevision(),
		},
		Attributes: attribsStruct,
	}

	return result, nil
}

func resourceToTestUser(_ context.Context, res *scimpb.Resource) (types.User, error) {
	u, err := types.NewUser(res.Id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.SetStaticLabels(map[string]string{
		testUserLabel:           testUserLabelValue,
		testUserExternalIDlabel: res.ExternalId,
	})
	traits := map[string][]string{}
	for k, v := range res.Attributes.AsMap() {
		switch typedVal := v.(type) {
		case string:
			traits[k] = []string{typedVal}
		}
	}
	u.SetTraits(traits)

	return u, err
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext interface{} = mock.MatchedBy(func(context.Context) bool { return true })

var anyUser interface{} = mock.MatchedBy(func(types.User) bool { return true })

var anyResource interface{} = mock.MatchedBy(func(*scimpb.Resource) bool { return true })

const (
	testPluginName          = "test"
	testPluginOrgUrl        = "https://mystery-machine.mockta.com"
	testSSOConnectorID      = "test-okta-integration"
	testAppID               = "okta-app-id"
	testPluginID            = "some-string-unique-to-the-plugin"
	testAuthHeader          = "some sort of bearer token"
	withSecrets             = true
	withoutSecrets          = false
	testUserExternalIDlabel = "mock-extrenal-id"
	testUserLabel           = "favouriteFruit"
	testUserLabelValue      = "nectarine"

	// doesn't really matter what this value is, as long as it matches the
	// plugin value created by mkTestPlugin() and is unlikely to be something
	// that will have a SCIM integration
	testPluginType = types.PluginTypeJira
)

func TestListUserResources(t *testing.T) {
	enableIGS(t)

	// make a list of 50 users, where every 3rd user is one that belongs to the
	// test plugin
	users := mkTestUserList(t, 50, 3)

	testCases := []struct {
		name        string
		filter      string
		page        *scimpb.Page
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		{
			name:        "all",
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(17), list.TotalResults)
				require.Len(t, list.Resources, 17)
				for i, res := range list.Resources {
					expectedID := i * 3
					require.Equal(t, fmt.Sprintf("User%03d@example.com", expectedID), res.Id)
				}
			},
		}, {
			name:        "summary count",
			page:        &scimpb.Page{StartIndex: 1, Count: 0},
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(17), list.TotalResults)
				require.Empty(t, list.Resources)
			},
		}, {
			name:        "filtered (matching)",
			filter:      `userName eq "User012@example.com"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(1), list.TotalResults)
				require.Len(t, list.Resources, 1)
				require.Equal(t, "User012@example.com", list.Resources[0].Id)
			},
		}, {
			name:        "filtered (empty)",
			filter:      `userName eq "User999@example.com"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(0), list.TotalResults)
				require.Empty(t, list.Resources)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Given a SCIM service connected to a user database containing some
			// users belonging to a provide, and some not...
			uut, fix := newTestService(t)

			fix.shim.
				On("authorizeRequest", anyContext, testAuthHeader).
				Return(nil)
			fix.shim.
				On("userPredicate", anyContext, anyUser).
				Return(isTestPluginUser)
			fix.shim.
				On("userToResource", anyContext, anyUser).
				Maybe().
				Return(testUserToResource)

			fix.plugins.
				On("GetPlugin", anyContext, testPluginName, withSecrets).
				Return(mkTestPlugin(), nil)

			fix.users.
				On("ListUsers", anyContext, mock.AnythingOfType("int"),
					mock.AnythingOfType("string"), withoutSecrets).
				Return(users, "", nil)

			// When I attempt to list all of the User resources via SCIM...
			list, err := uut.ListSCIMResources(fix.userCtx, &scimpb.ListSCIMResourcesRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
				},
				Page:   tt.page,
				Filter: tt.filter,
			})

			tt.expectError(t, err)
			tt.expectValue(t, list)
		})
	}
}

func TestListUsersHandlesPagedUsers(t *testing.T) {
	enableIGS(t)

	// Given a SCIM service connected to a user database containing some users
	// belonging to a provide, and some not...
	uut, fix := newTestService(t)
	defer fix.AssertExpectations(t)

	// configure the provider shim with a basic implementation
	fix.shim.
		On("authorizeRequest", anyContext, testAuthHeader).
		Return(nil)
	fix.shim.
		On("userPredicate", anyContext, anyUser).
		Return(isTestPluginUser)
	fix.shim.
		On("userToResource", anyContext, anyUser).
		Return(testUserToResource)

	fix.plugins.
		On("GetPlugin", anyContext, testPluginName, withSecrets).
		Return(mkTestPlugin(), nil)

	// make a list of 50 users, where every 3rd user is one that belongs to the
	// test plugin
	users := mkTestUserList(t, 50, 3)

	// configure the user mock to deliver the users in several, arbitrarily-
	// sized pages
	fix.users.
		On("ListUsers", anyContext, mock.AnythingOfType("int"), "", withoutSecrets).
		Return(users[0:5], "alpha", nil)
	fix.users.
		On("ListUsers", anyContext, mock.AnythingOfType("int"), "alpha", withoutSecrets).
		Return(users[5:15], "bravo", nil)
	fix.users.
		On("ListUsers", anyContext, mock.AnythingOfType("int"), "bravo", withoutSecrets).
		Return(users[15:35], "charlie", nil)
	fix.users.
		On("ListUsers", anyContext, mock.AnythingOfType("int"), "charlie", withoutSecrets).
		Return(users[35:49], "", nil)

	// When I attempt to list a specific user via a filtered list request ...
	list, err := uut.ListSCIMResources(fix.userCtx, &scimpb.ListSCIMResourcesRequest{
		Target: &scimpb.RequestTarget{
			Authorization: testAuthHeader,
			PluginId:      "test",
			ResourceType:  "Users",
		},
	})

	// Expect the operation to succeed
	require.NoError(t, err)

	// Also expect that the operation returned all of the desired results
	require.Equal(t, int32(1), list.StartIndex)
	require.Equal(t, int32(17), list.TotalResults)
	require.Len(t, list.Resources, 17)
}

func TestGetUser(t *testing.T) {
	enableIGS(t)

	// make a list of 50 users, where every 3rd user is one that belongs to the
	// test plugin
	users := mkTestUserList(t, 50, 3)

	lookupUser := func(_ context.Context, username string, _ bool) (types.User, error) {
		for _, u := range users {
			if u.GetName() == username {
				return u, nil
			}
		}
		return nil, trace.NotFound(username)
	}

	testCases := []struct {
		name        string
		username    string
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		{
			name:        "valid user",
			username:    "User012@example.com",
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				res, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "invalid arg type")
				require.Equal(t, "User012@example.com", res.Id)
			},
		}, {
			name:        "user exists but excluded from SCIM",
			username:    "User011@example.com",
			expectError: requireNotFound,
			expectValue: require.Nil,
		}, {
			name:        "user does not exist",
			username:    "User999@example.com",
			expectError: requireNotFound,
			expectValue: require.Nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Given a SCIM service connected to a user database containing some
			// users belonging to a provide, and some not...
			uut, fix := newTestService(t)
			defer fix.AssertExpectations(t)

			fix.shim.
				On("authorizeRequest", anyContext, testAuthHeader).
				Return(nil)
			fix.shim.
				On("userPredicate", anyContext, anyUser).
				Maybe().
				Return(isTestPluginUser)
			fix.shim.
				On("userToResource", anyContext, anyUser).
				Maybe().
				Return(testUserToResource)

			fix.plugins.
				On("GetPlugin", anyContext, testPluginName, withSecrets).
				Return(mkTestPlugin(), nil)

			fix.users.
				On("GetUser", anyContext, tt.username, withoutSecrets).
				Return(lookupUser)

			// When I attempt to fetch a User resource via SCIM...
			resource, err := uut.GetSCIMResource(fix.userCtx, &scimpb.GetSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
					ResourceId:    tt.username,
				},
			})

			tt.expectError(t, err)
			tt.expectValue(t, resource)
		})
	}
}

func TestCreateUser(t *testing.T) {
	const userRevision = "user revision number"

	enableIGS(t)

	mockCreateUser := func(_ context.Context, u types.User) (types.User, error) {
		u.SetRevision(userRevision)
		u.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
			Time: time.Now(),
			Connector: &types.ConnectorRef{
				ID:   testSSOConnectorID,
				Type: constants.SAML,
			},
		})
		return u, nil
	}

	testCases := []struct {
		name               string
		createUserResponse []any
		expectCreatedEvent bool
		expectError        require.ErrorAssertionFunc
		expectValue        require.ValueAssertionFunc
	}{
		{
			name:               "simple",
			createUserResponse: []any{mockCreateUser},
			expectCreatedEvent: true,
			expectError:        require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				created, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "expected a SCIM resource")
				require.Equal(t, "newUser@example.com", created.Id)
				require.Equal(t, "1234567890", created.ExternalId)
				require.Equal(t, userRevision, created.Meta.Version)

				expectedTraits := map[string]any{
					"alpha": "0",
					"beta":  "1",
					"gamma": "2",
					"delta": "3",
				}
				require.Equal(t, expectedTraits, created.Attributes.AsMap())
			},
		}, {
			name:               "name collision",
			expectCreatedEvent: false,
			createUserResponse: []any{nil, trace.AlreadyExists("user already exists")},
			expectError:        requireAlreadyExists,
			expectValue:        require.Nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Given a SCIM service connected to a user database containing some users
			// belonging to a provide, and some not...
			uut, fix := newTestService(t)
			defer fix.AssertExpectations(t)

			resource := &scimpb.Resource{
				Id:         "newUser@example.com",
				ExternalId: "1234567890",
				Schemas:    []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
				Meta: &scimpb.Meta{
					ResourceType: "User",
				},
				Attributes: must(structpb.NewStruct(map[string]any{
					"alpha": "0",
					"beta":  "1",
					"gamma": "2",
					"delta": "3",
				})),
			}

			fix.shim.
				On("authorizeRequest", anyContext, testAuthHeader).
				Return(nil)
			fix.shim.
				On("resourceToUser", anyContext, anyResource).
				Return(resourceToTestUser)
			fix.shim.
				On("onCreatingUser", anyContext, anyUser, anyResource).
				Once().
				Return(nil)

			if tt.expectCreatedEvent {
				fix.shim.
					On("onCreatedUser", anyContext, anyUser, anyResource).
					Once().
					Return(nil)
				fix.shim.
					On("userToResource", anyContext, anyUser).
					Once().
					Return(testUserToResource)
			}

			fix.users.
				On("CreateUser", anyContext, anyUser).
				Return(tt.createUserResponse...)

			fix.plugins.
				On("GetPlugin", anyContext, testPluginName, withSecrets).
				Return(mkTestPlugin(), nil)

			// When I attempt to create a new user...
			created, err := uut.CreateSCIMResource(fix.userCtx, &scimpb.CreateSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
				},
				Resource: resource,
			})

			tt.expectError(t, err)
			tt.expectValue(t, created)
		})
	}
}

func TestUpdateUser(t *testing.T) {
	enableIGS(t)

	// Given a SCIM service connected to a user database containing some users
	// belonging to a provide, and some not...
	uut, fix := newTestService(t)

	oldUser := mkTestUser(t, 7)

	newResource := &scimpb.Resource{
		Id:         "User007@example.com",
		ExternalId: "1234567890",
		Schemas:    []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Meta: &scimpb.Meta{
			ResourceType: "User",
		},
		Attributes: must(structpb.NewStruct(map[string]any{
			"alpha": "0",
			"beta":  "1",
			"gamma": "2",
			"delta": "3",
		})),
	}

	fix.shim.
		On("authorizeRequest", anyContext, testAuthHeader).
		Return(nil)
	fix.shim.
		On("resourceToUser", anyContext, anyResource).
		Return(resourceToTestUser)
	fix.shim.
		On("userToResource", anyContext, anyUser).
		Return(testUserToResource)
	fix.shim.
		On("onUpdatingUser", anyContext, anyUser, anyResource).
		Return(func(ctx context.Context, u types.User, r *scimpb.Resource) (types.User, error) {
			return fix.shim.resourceToUser(ctx, r)
		})

	fix.users.
		On("GetUser", anyContext, "User007@example.com", withoutSecrets).
		Return(oldUser, nil)
	fix.users.
		On("UpdateUser", anyContext, anyUser).
		Return(func(_ context.Context, u types.User) (types.User, error) {
			return u, nil
		})

	fix.plugins.
		On("GetPlugin", anyContext, testPluginName, withSecrets).
		Return(mkTestPlugin(), nil)

	// When I attempt to create a new user...
	updated, err := uut.UpdateSCIMResource(fix.userCtx, &scimpb.UpdateSCIMResourceRequest{
		Target: &scimpb.RequestTarget{
			Authorization: testAuthHeader,
			PluginId:      testPluginName,
			ResourceType:  "Users",
			ResourceId:    oldUser.GetName(),
		},
		Resource: newResource,
	})

	require.NoError(t, err)
	require.Equal(t, "User007@example.com", updated.Id)

	expectedTraits := map[string]any{
		"alpha": "0",
		"beta":  "1",
		"gamma": "2",
		"delta": "3",
	}
	require.Equal(t, expectedTraits, updated.Attributes.AsMap())
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func mkTestUser(t *testing.T, id int) types.User {
	user, err := types.NewUser(fmt.Sprintf("User%03d@example.com", id))
	require.NoError(t, err, "making test user list should succeed")
	return user
}

func mkTestUserList(t *testing.T, n int, spacing int) []types.User {
	dst := make([]types.User, n)
	for i := 0; i < n; i++ {
		dst[i] = mkTestUser(t, i)
		dst[i].SetRevision(strconv.Itoa(i % 5))
		dst[i].SetCreatedBy(types.CreatedBy{
			Time: time.Date(2024, 1, 19, 14, 13, i%60, 0, time.UTC),
		})

		labels := map[string]string{
			testUserLabel: "banana",
		}
		if i%spacing == 0 {
			labels[testUserExternalIDlabel] = strconv.Itoa(i + 1000)
			labels[testUserLabel] = testUserLabelValue
		}
		dst[i].SetStaticLabels(labels)
	}
	return dst
}

func mkTestPlugin() types.Plugin {
	return &types.PluginV1{
		Kind:    types.KindPlugin,
		SubKind: types.PluginSubkindAccess,
		Version: "abc123",
		Metadata: types.Metadata{
			Name:      testPluginName,
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"test": testPluginName},
		},
		// Spec is deliberately empty, apart from giving the plugin just enough
		// info to determine its type. The SCIM server should not touch *any*
		// plugin-type specific properties of the plugin. Only a type-specific
		// shim should is allowed to do that, and we'll handle that with a mock
		// shim for these tests.
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Jira{},
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

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func requireAlreadyExists(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsAlreadyExists(err), "Expected AlreadyExists, got %s", err)
}
