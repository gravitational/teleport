package scim

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
)

func TestUserList(t *testing.T) {
	enableOktaSCIMEntitlement(t)

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
					require.Equal(t, fmt.Sprintf("test-user-%03d@example.com", expectedID), res.Id)
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
			filter:      `userName eq "test-user-012@example.com"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(1), list.TotalResults)
				require.Len(t, list.Resources, 1)
				require.Equal(t, "test-user-012@example.com", list.Resources[0].Id)
			},
		}, {
			name:        "filtered (empty)",
			filter:      `userName eq "test-user-999@example.com"`,
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
				On("ListUsers", anyContext, anyListUserRequest).
				Run(requireUserListDoesNotRequestSecrets(t)).
				Return(&userspb.ListUsersResponse{Users: users}, nil)

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

func TestUsersListHandlesPagedUsers(t *testing.T) {
	enableOktaSCIMEntitlement(t)

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
		On("ListUsers", anyContext, listUserRequestWithPageToken("")).
		Run(requireUserListDoesNotRequestSecrets(t)).
		Return(&userspb.ListUsersResponse{Users: users[0:5], NextPageToken: "alpha"}, nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("alpha")).
		Return(&userspb.ListUsersResponse{Users: users[5:15], NextPageToken: "bravo"}, nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("bravo")).
		Return(&userspb.ListUsersResponse{Users: users[15:35], NextPageToken: "charlie"}, nil)
	fix.users.
		On("ListUsers", anyContext, listUserRequestWithPageToken("charlie")).
		Return(&userspb.ListUsersResponse{Users: users[35:49], NextPageToken: ""}, nil)

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

func TestUserGet(t *testing.T) {
	enableOktaSCIMEntitlement(t)

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
			expectValue: func(t require.TestingT, obj interface{}, _ ...interface{}) {
				res, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "invalid arg type")
				require.Equal(t, "test-user-012@example.com", res.Id)
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

func TestUserCreate(t *testing.T) {
	const userRevision = "user revision number"

	enableOktaSCIMEntitlement(t)

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

func TestUserUpdate(t *testing.T) {
	enableOktaSCIMEntitlement(t)

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
		On("userToResource", anyContext, anyUser).
		Return(testUserToResource)
	fix.shim.
		On("onUpdatingUser", anyContext, anyUser, anyResource).
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

// anyUser is an argument matcher for testify mocks that matches any user value
var anyUser interface{} = mock.MatchedBy(func(types.User) bool { return true })

// isUserNamed is a testify mock argument matcher that matches any user with any
// user with a given name
func isUserNamed(name string) interface{} {
	return mock.MatchedBy(
		func(u types.User) bool {
			return u.GetName() == name
		})
}

// anyListUserRequest is an argument matcher for testify mocks that matches any
// non-nil ListUsersRequest value.
var anyListUserRequest interface{} = mock.MatchedBy(func(r *userspb.ListUsersRequest) bool { return r != nil })

// listUserRequestWithPageToken is an argument matcher for testify mocks that
// matches any ListUsersRequest with a given page token value
func listUserRequestWithPageToken(token string) interface{} {
	return mock.MatchedBy(func(r *userspb.ListUsersRequest) bool { return r.PageToken == token })
}

// requireUserListDoesNotRequestSecrets returns a function that can be used with
// `mock.Run()` to assert that the supplied ListUsersRequest does not request
// user secrets.
func requireUserListDoesNotRequestSecrets(t *testing.T) func(mock.Arguments) {
	return func(args mock.Arguments) {
		r := getResultAs[*userspb.ListUsersRequest](args, 1)
		require.False(t, r.WithSecrets, "User list request must not request secrets")
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
