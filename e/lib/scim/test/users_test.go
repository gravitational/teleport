package test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestUserList(t *testing.T) {
	t.Parallel()
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
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.GetStartIndex())
				require.Equal(t, int32(17), list.GetTotalResults())
				require.Len(t, list.GetResources(), 17)
				for i, res := range list.GetResources() {
					expectedID := i * 3
					require.Equal(t, fmt.Sprintf("test-user-%03d@example.com", expectedID), res.GetId())
				}
			},
		}, {
			name:        "summary count",
			page:        scimpb.Page_builder{StartIndex: 1, Count: 0}.Build(),
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(17), list.GetTotalResults())
				require.Empty(t, list.GetResources())
			},
		}, {
			name:        "filtered (matching)",
			filter:      `userName eq "test-user-012@example.com"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.GetStartIndex())
				require.Equal(t, int32(1), list.GetTotalResults())
				require.Len(t, list.GetResources(), 1)
				require.Equal(t, "test-user-012@example.com", list.GetResources()[0].GetId())
			},
		}, {
			name:        "filtered (empty)",
			filter:      `userName eq "test-user-999@example.com"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.GetStartIndex())
				require.Equal(t, int32(0), list.GetTotalResults())
				require.Empty(t, list.GetResources())
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Given a SCIM service connected to a user database containing some
			// users belonging to a provide, and some not...
			uut, fix := newTestServiceWith(t, &testFixture{
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})

			rigFixtureSetupForSCIMAuth(fix)
			fix.shim.
				On("UserPredicate", anyContext, anyUser).
				Return(isTestPluginUser)
			fix.shim.
				On("UserToResource", anyContext, anyUser).
				Maybe().
				Return(testUserToResource)

			fix.plugins.
				On("GetPlugin", anyContext, testPluginName, withSecrets).
				Return(mkTestPlugin(), nil)

			fix.users.
				On("ListUsers", anyContext, anyListUserRequest).
				Run(requireUserListDoesNotRequestSecrets(t)).
				Return(userspb.ListUsersResponse_builder{Users: users}.Build(), nil)

			// When I attempt to list all of the User resources via SCIM...
			list, err := uut.ListSCIMResources(fix.userCtx, scimpb.ListSCIMResourcesRequest_builder{
				Target: scimpb.RequestTarget_builder{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
				}.Build(),
				Page:   tt.page,
				Filter: tt.filter,
			}.Build())

			tt.expectError(t, err)
			tt.expectValue(t, list)
		})
	}
}

func TestUsersListHandlesPagedUsers(t *testing.T) {
	t.Parallel()
	// Given a SCIM service connected to a user database containing some users
	// belonging to a provide, and some not...
	uut, fix := newTestServiceWith(t, &testFixture{
		modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.OktaSCIM: {Enabled: true},
				},
			},
		},
	})
	defer fix.AssertExpectations(t)

	rigFixtureSetupForSCIMAuth(fix)
	fix.shim.
		On("UserPredicate", anyContext, anyUser).
		Return(isTestPluginUser)
	fix.shim.
		On("UserToResource", anyContext, anyUser).
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
		On("ListUsers", anyContext, listUserRequestWithPageToken("")).
		Run(requireUserListDoesNotRequestSecrets(t)).
		Return(userspb.ListUsersResponse_builder{Users: users[0:5], NextPageToken: "alpha"}.Build(), nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("alpha")).
		Return(userspb.ListUsersResponse_builder{Users: users[5:15], NextPageToken: "bravo"}.Build(), nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("bravo")).
		Return(userspb.ListUsersResponse_builder{Users: users[15:35], NextPageToken: "charlie"}.Build(), nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("charlie")).
		Return(userspb.ListUsersResponse_builder{Users: users[35:49], NextPageToken: ""}.Build(), nil)

	// When I attempt to list a specific user via a filtered list request ...
	list, err := uut.ListSCIMResources(fix.userCtx, scimpb.ListSCIMResourcesRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: testAuthHeader,
			PluginId:      "test",
			ResourceType:  "Users",
		}.Build(),
	}.Build())

	// Expect the operation to succeed
	require.NoError(t, err)

	// Also expect that the operation returned all of the desired results
	require.Equal(t, int32(1), list.GetStartIndex())
	require.Equal(t, int32(17), list.GetTotalResults())
	require.Len(t, list.GetResources(), 17)
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
	result := scimpb.Resource_builder{
		Id:         user.GetName(),
		ExternalId: externalID,
		Meta: scimpb.Meta_builder{
			Created: timestamppb.New(user.GetCreatedBy().Time),
			Version: user.GetRevision(),
		}.Build(),
		Attributes: attribsStruct,
	}.Build()

	return result, nil
}

func resourceToTestUser(_ context.Context, res *scimpb.Resource) (types.User, error) {
	u, err := types.NewUser(res.GetId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.SetStaticLabels(map[string]string{
		testUserLabel:           testUserLabelValue,
		testUserExternalIDlabel: res.GetExternalId(),
	})
	traits := map[string][]string{}
	for k, v := range res.GetAttributes().AsMap() {
		switch typedVal := v.(type) {
		case string:
			traits[k] = []string{typedVal}
		}
	}
	u.SetTraits(traits)

	return u, err
}

func TestUserGet(t *testing.T) {
	t.Parallel()
	// make a list of 50 users, where every 3rd user is one that belongs to the
	// test plugin
	users := mkTestUserList(t, 50, 3)

	lookupUser := func(_ context.Context, username string, _ bool) (types.User, error) {
		for _, u := range users {
			if u.GetName() == username {
				return u, nil
			}
		}
		return nil, trace.NotFound("%s", username)
	}

	testCases := []struct {
		name        string
		username    string
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		{
			name:        "valid user",
			username:    "test-user-012@example.com",
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				res, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "invalid arg type")
				require.Equal(t, "test-user-012@example.com", res.GetId())
			},
		}, {
			name:        "user exists but excluded from SCIM",
			username:    "test-user-011@example.com",
			expectError: requireNotFound,
			expectValue: require.Nil,
		}, {
			name:        "user does not exist",
			username:    "test-user-999@example.com",
			expectError: requireNotFound,
			expectValue: require.Nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Given a SCIM service connected to a user database containing some
			// users belonging to a provide, and some not...
			uut, fix := newTestServiceWith(t, &testFixture{
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			defer fix.AssertExpectations(t)

			rigFixtureSetupForSCIMAuth(fix)
			fix.shim.
				On("UserPredicate", anyContext, anyUser).
				Maybe().
				Return(isTestPluginUser)
			fix.shim.
				On("UserToResource", anyContext, anyUser).
				Maybe().
				Return(testUserToResource)

			fix.plugins.
				On("GetPlugin", anyContext, testPluginName, withSecrets).
				Return(mkTestPlugin(), nil)

			fix.users.
				On("GetUser", anyContext, tt.username, withoutSecrets).
				Return(lookupUser)

			// When I attempt to fetch a User resource via SCIM...
			resource, err := uut.GetSCIMResource(fix.userCtx, scimpb.GetSCIMResourceRequest_builder{
				Target: scimpb.RequestTarget_builder{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
					ResourceId:    tt.username,
				}.Build(),
			}.Build())

			tt.expectError(t, err)
			tt.expectValue(t, resource)
		})
	}
}

func TestUserCreate(t *testing.T) {
	t.Parallel()
	const userRevision = "user revision number"

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
				require.Equal(t, "newUser@example.com", created.GetId())
				require.Equal(t, "1234567890", created.GetExternalId())
				require.Equal(t, userRevision, created.GetMeta().GetVersion())

				expectedTraits := map[string]any{
					"alpha": "0",
					"beta":  "1",
					"gamma": "2",
					"delta": "3",
				}
				require.Equal(t, expectedTraits, created.GetAttributes().AsMap())
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
			uut, fix := newTestServiceWith(t, &testFixture{
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			defer fix.AssertExpectations(t)

			resource := scimpb.Resource_builder{
				Id:         "newUser@example.com",
				ExternalId: "1234567890",
				Schemas:    []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
				Meta: scimpb.Meta_builder{
					ResourceType: "User",
				}.Build(),
				Attributes: must(structpb.NewStruct(map[string]any{
					"alpha": "0",
					"beta":  "1",
					"gamma": "2",
					"delta": "3",
				})),
			}.Build()

			rigFixtureSetupForSCIMAuth(fix)

			fix.shim.
				On("ResourceToUser", anyContext, anyResource).
				Return(resourceToTestUser)

			if tt.expectCreatedEvent {
				fix.shim.
					On("OnCreatedUser", anyContext, anyUser, anyResource).
					Once().
					Return(nil)
				fix.shim.
					On("UserToResource", anyContext, anyUser).
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
			created, err := uut.CreateSCIMResource(fix.userCtx, scimpb.CreateSCIMResourceRequest_builder{
				Target: scimpb.RequestTarget_builder{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Users",
				}.Build(),
				Resource: resource,
			}.Build())

			tt.expectError(t, err)
			tt.expectValue(t, created)
		})
	}
}

func TestUserUpdate(t *testing.T) {
	t.Parallel()
	// Given a SCIM service connected to a user database containing some users
	// belonging to a provide, and some not...
	uut, fix := newTestServiceWith(t, &testFixture{
		modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.OktaSCIM: {Enabled: true},
				},
			},
		},
	})

	oldUser := mkTestUser(t, 7)

	newResource := scimpb.Resource_builder{
		Id:         "User007@example.com",
		ExternalId: "1234567890",
		Schemas:    []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Meta: scimpb.Meta_builder{
			ResourceType: "User",
		}.Build(),
		Attributes: must(structpb.NewStruct(map[string]any{
			"alpha": "0",
			"beta":  "1",
			"gamma": "2",
			"delta": "3",
		})),
	}.Build()

	rigFixtureSetupForSCIMAuth(fix)
	fix.shim.
		On("UserToResource", anyContext, anyUser).
		Return(testUserToResource)
	fix.shim.
		On("OnUpdatingUser", anyContext, anyUser, anyResource).
		Return(func(ctx context.Context, u types.User, r *scimpb.Resource) (types.User, bool, error) {
			user, err := resourceToTestUser(ctx, r)
			return user, true, err
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
	updated, err := uut.UpdateSCIMResource(fix.userCtx, scimpb.UpdateSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: testAuthHeader,
			PluginId:      testPluginName,
			ResourceType:  "Users",
			ResourceId:    oldUser.GetName(),
		}.Build(),
		Resource: newResource,
	}.Build())

	require.NoError(t, err)
	require.Equal(t, "User007@example.com", updated.GetId())

	expectedTraits := map[string]any{
		"alpha": "0",
		"beta":  "1",
		"gamma": "2",
		"delta": "3",
	}
	require.Equal(t, expectedTraits, updated.GetAttributes().AsMap())
}

// anyUser is an argument matcher for testify mocks that matches any user value
var anyUser any = mock.MatchedBy(func(types.User) bool { return true })

// isUserNamed is a testify mock argument matcher that matches any user with any
// user with a given name
func isUserNamed(name string) any {
	return mock.MatchedBy(
		func(u types.User) bool {
			return u.GetName() == name
		})
}

// anyListUserRequest is an argument matcher for testify mocks that matches any
// non-nil ListUsersRequest value.
var anyListUserRequest any = mock.MatchedBy(func(r *userspb.ListUsersRequest) bool { return r != nil })

// listUserRequestWithPageToken is an argument matcher for testify mocks that
// matches any ListUsersRequest with a given page token value
func listUserRequestWithPageToken(token string) any {
	return mock.MatchedBy(func(r *userspb.ListUsersRequest) bool { return r.GetPageToken() == token })
}

// requireUserListDoesNotRequestSecrets returns a function that can be used with
// `mock.Run()` to assert that the supplied ListUsersRequest does not request
// user secrets.
func requireUserListDoesNotRequestSecrets(t *testing.T) func(mock.Arguments) {
	return func(args mock.Arguments) {
		r := getResultAs[*userspb.ListUsersRequest](args, 1)
		require.False(t, r.GetWithSecrets(), "User list request must not request secrets")
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func mkTestUser(t *testing.T, id int) *types.UserV2 {
	user, err := types.NewUser(userName(id))
	require.NoError(t, err, "making test user list should succeed")
	labels := map[string]string{
		testUserExternalIDlabel: strconv.Itoa(id + 1000),
		testUserLabel:           "banana",
	}
	user.SetStaticLabels(labels)
	return user.(*types.UserV2)
}

func mkTestPluginUser(t *testing.T, id int) *types.UserV2 {
	user, err := types.NewUser(userName(id))
	require.NoError(t, err, "making test user list should succeed")
	labels := map[string]string{
		testUserExternalIDlabel: strconv.Itoa(id + 1000),
		testUserLabel:           testUserLabelValue,
	}
	user.SetStaticLabels(labels)
	return user.(*types.UserV2)
}

func mkTestUserList(t *testing.T, n int, spacing int) []*types.UserV2 {
	dst := make([]*types.UserV2, n)
	for i := range dst {
		usr := mkTestUser(t, i)
		if i%spacing == 0 {
			usr = mkTestPluginUser(t, i)
		}

		usr.SetRevision(strconv.Itoa(i % 5))
		usr.SetCreatedBy(types.CreatedBy{
			Time: time.Date(2024, 1, 19, 14, 13, i%60, 0, time.UTC),
		})

		dst[i] = usr
	}
	return dst
}
