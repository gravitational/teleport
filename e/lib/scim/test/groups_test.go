package test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	newACLID          = "test-new-acl"
	newACLDisplayName = "test access list"
)

func TestGroupList(t *testing.T) {
	t.Parallel()
	testAccessLists := mkTestAccessLists(t, 50, 3)

	testCases := []struct {
		name        string
		filter      string
		page        *scimpb.Page
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		// Tests listing all resources, unfiltered and unpaged
		{
			name:        "all",
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(17), list.TotalResults)
				require.Len(t, list.Resources, 17)
				for i, res := range list.Resources {
					expectedID := i * 3
					require.Equal(t, fmt.Sprintf("Access-List-%03d", expectedID), res.Id)
				}
			},
		},
		// Tests counting the matching resources by setting a zero-length page
		// request
		{
			name:        "summary count",
			page:        &scimpb.Page{StartIndex: 1, Count: 0},
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(17), list.TotalResults)
				require.Empty(t, list.Resources)
			},
		},
		// Tests filtering the group list with a SCIM filter expression,
		// returning a single value in a SCIM resource list
		{
			name:        "filtered (matching)",
			filter:      `groupName eq "Access-List-012"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				list, ok := obj.(*scimpb.ResourceList)
				require.True(t, ok, "expected resource list")
				require.Equal(t, int32(1), list.StartIndex)
				require.Equal(t, int32(1), list.TotalResults)
				require.Len(t, list.Resources, 1)
				require.Equal(t, "Access-List-012", list.Resources[0].Id)
			},
		},
		// Tests filtering the group list down to nothing, asserting that the
		// SCIM service can deal with an empty list
		{
			name:        "filtered (empty)",
			filter:      `groupName eq "Access-List-999"`,
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
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
			rigFixtureForGroupTest(fix)

			// Configure the access lists service to return our collection of
			// AccessLists
			fix.accesslists.
				On("ListAccessLists", anyContext, mock.AnythingOfType("int"),
					mock.AnythingOfType("string")).
				Return(testAccessLists, "", nil)

			// When I attempt to list all of the User resources via SCIM...
			list, err := uut.ListSCIMResources(fix.userCtx, &scimpb.ListSCIMResourcesRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Groups",
				},
				Page:   tt.page,
				Filter: tt.filter,
			})

			tt.expectError(t, err)
			tt.expectValue(t, list)
		})
	}
}

func TestGroupListHandlesPagedAccessLists(t *testing.T) {
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
	rigFixtureForGroupTest(fix)

	// Configure the access lists service to return a collection of
	// AccessLists in several, arbitrarily- sized pages
	testAccessLists := mkTestAccessLists(t, 50, 3)
	fix.accesslists.
		On("ListAccessLists", anyContext, mock.AnythingOfType("int"), "").
		Return(testAccessLists[0:6], "alpha", nil)
	fix.accesslists.
		On("ListAccessLists", anyContext, mock.AnythingOfType("int"), "alpha").
		Return(testAccessLists[6:15], "beta", nil)
	fix.accesslists.
		On("ListAccessLists", anyContext, mock.AnythingOfType("int"), "beta").
		Return(testAccessLists[15:35], "gamma", nil)
	fix.accesslists.
		On("ListAccessLists", anyContext, mock.AnythingOfType("int"), "gamma").
		Return(testAccessLists[35:49], "", nil)

	// When I attempt to list a specific user via a filtered list request ...
	list, err := uut.ListSCIMResources(fix.userCtx, &scimpb.ListSCIMResourcesRequest{
		Target: &scimpb.RequestTarget{
			Authorization: testAuthHeader,
			PluginId:      "test",
			ResourceType:  "Groups",
		},
	})

	// Expect the operation to succeed
	require.NoError(t, err)

	// Also expect that the operation returned all of the desired results
	require.Equal(t, int32(1), list.StartIndex)
	require.Equal(t, int32(17), list.TotalResults)
	require.Len(t, list.Resources, 17)
}

func TestGroupGet(t *testing.T) {
	t.Parallel()

	testAccessLists := mkTestAccessLists(t, 10, 2)
	clock := clockwork.NewFakeClock()

	testCases := []struct {
		name                string
		group               string
		getAccessListResult []any
		getMemberListResult []any
		expectError         require.ErrorAssertionFunc
		expectValue         require.ValueAssertionFunc
	}{
		// Tests getting a single group
		{
			name:                "valid",
			group:               testAccessLists[0].GetName(),
			getAccessListResult: []any{testAccessLists[0], nil},
			getMemberListResult: []any{
				mkTestMembers(t, testAccessLists[0], 5, clock), "", nil,
			},
			expectError: require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				r, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "expected resource")

				expectedACL := testAccessLists[0]
				expectedName := expectedACL.GetName()

				require.Equal(t, expectedName, r.Id)
				require.ElementsMatch(t, []string{schema.GroupSchema}, r.Schemas)
				require.Equal(t, "Group", r.Meta.ResourceType)
				require.Equal(t, "/Groups/"+expectedName, r.Meta.Location)
				require.Equal(t, `W/"revision-Access-List-000"`, r.Meta.Version)

				attributes := r.Attributes.AsMap()
				require.Equal(t,
					map[string]any{
						"displayName": expectedACL.Spec.Title,
						"members": []any{
							map[string]any{
								"value":   "test-user-000@example.com",
								"display": "test-user-000@example.com",
							},
							map[string]any{
								"value":   "test-user-001@example.com",
								"display": "test-user-001@example.com",
							},
							map[string]any{
								"value":   "test-user-002@example.com",
								"display": "test-user-002@example.com",
							},
							map[string]any{
								"value":   "test-user-003@example.com",
								"display": "test-user-003@example.com",
							},
							map[string]any{
								"value":   "test-user-004@example.com",
								"display": "test-user-004@example.com",
							},
						},
					},
					attributes)
			},
		},
		// Asserts that requesting a non-existent group returns an error rather
		// than crashing
		{
			name:                "no such group",
			group:               "some group",
			getAccessListResult: []any{nil, trace.NotFound("No such access list")},
			expectError:         requireNotFound,
			expectValue:         require.Nil,
		},
		// Asserts that requesting a existing group that does not belong to the
		// target IdP returns NotFound, rather than just handing the AccessList
		// back
		{
			name:                "group exists but does not belong to plugin",
			group:               testAccessLists[1].GetName(),
			getAccessListResult: []any{testAccessLists[1], nil},
			expectError:         requireNotFound,
			expectValue:         require.Nil,
		},
		// Asserts that the SCIM service can handle a group with an empty member
		// list
		{
			name:                "empty group",
			group:               testAccessLists[2].GetName(),
			getAccessListResult: []any{testAccessLists[2], nil},
			getMemberListResult: []any{[]*accesslist.AccessListMember{}, "", nil},
			expectError:         require.NoError,
			expectValue: func(t require.TestingT, obj any, _ ...any) {
				r, ok := obj.(*scimpb.Resource)
				require.True(t, ok, "expected resource")

				attributes := r.Attributes.AsMap()
				require.Contains(t, attributes, "members")
				require.Empty(t, attributes["members"])
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			uut, fix := newTestServiceWith(t, &testFixture{
				clock: clock,
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			rigFixtureForGroupTest(fix)

			fix.accesslists.
				On("GetAccessList", anyContext, tt.group).
				Return(tt.getAccessListResult...)

			if tt.getMemberListResult != nil {
				fix.accesslists.
					On("ListAccessListMembers", anyContext, tt.group, 0, "").
					Return(tt.getMemberListResult...)
			}

			resource, err := uut.GetSCIMResource(fix.userCtx, &scimpb.GetSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Groups",
					ResourceId:    tt.group,
				},
			})

			tt.expectError(t, err)
			tt.expectValue(t, resource)
		})
	}
}

func TestGroupCreate(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	users := mkTestUserList(t, 10, 1)

	testCases := []struct {
		name                          string
		makeResource                  func(t *testing.T) *scimpb.Resource
		expectACLLookup               bool
		aclLookupResult               []*accesslist.AccessList
		expectACLMemberListing        bool
		expectedUserLookups           []*types.UserV2
		expectCreatingAccessListEvent bool
		expectCreateAccessList        bool
		expectedAccessListID          string
		expectedDisplayName           string
		expectedMemberLookups         []*types.UserV2
		expectedMembers               []*types.UserV2
		expectedRoles                 map[string][]any
		expectError                   require.ErrorAssertionFunc
		expectValue                   require.ValueAssertionFunc
	}{
		// Simplest test case; create an empty access list from an empty
		// group.
		{
			name:                          "empty",
			makeResource:                  mkGroup(newACLDisplayName, 0),
			expectACLLookup:               true,
			expectCreatingAccessListEvent: true,
			expectCreateAccessList:        true,
			expectedAccessListID:          newACLID,
			expectedDisplayName:           newACLDisplayName,
			expectedRoles: map[string][]any{
				"test_access_list-access-okta-acl-role-test-new-acl":   {passThrough[types.Role]},
				"test_access_list-reviewer-okta-acl-role-test-new-acl": {passThrough[types.Role]},
			},
			expectError:     require.NoError,
			expectedMembers: []*types.UserV2{},
		},

		// The Resource ID must be ignored when creating an access list, and a
		// Teleport-supplied one must be used instead
		{
			name: "ID must be ignored",
			makeResource: func(t *testing.T) *scimpb.Resource {
				r := mkGroup(newACLDisplayName, 0)(t)
				r.Id = "DO NOT TRUST THIS VALUE"
				return r
			},
			expectACLLookup: false,
			expectError:     requireBadParameter,
		},

		// Most common case; create a new AccessList with some members
		{
			name:                          "with members",
			makeResource:                  mkGroup(newACLDisplayName, 5),
			expectACLLookup:               true,
			expectCreatingAccessListEvent: true,
			expectCreateAccessList:        true,
			expectedAccessListID:          newACLID,
			expectedDisplayName:           newACLDisplayName,
			expectedMemberLookups:         users[0:5],
			expectedMembers:               users[0:5],
			expectedRoles: map[string][]any{
				"test_access_list-access-okta-acl-role-test-new-acl":   {passThrough[types.Role]},
				"test_access_list-reviewer-okta-acl-role-test-new-acl": {passThrough[types.Role]},
			},
			expectError: require.NoError,
		},

		// In some cases, the SCIM client can "adopt" an existing AccessList. This
		// allows the Okta Sync Service to create the AccessList during a sync and
		// then be updated via SCIM as necessary later on.
		{
			name:                   "adoption",
			makeResource:           mkGroup("Access List #0", 6),
			expectACLLookup:        true,
			aclLookupResult:        mkTestAccessLists(t, 2, 2),
			expectedAccessListID:   "Access-List-000",
			expectedDisplayName:    "Access List #0",
			expectedMemberLookups:  users[0:6],
			expectedMembers:        users[0:6],
			expectACLMemberListing: true,
			expectError:            require.NoError,
		},

		// An access list can only be "adopted" if
		//  1. it's a valid target for the SCIM service, as determined by the
		//     IdP shim access list predicate
		//  2. its Title value matches the SCIM group's "displayName", and
		//  3. there is only exactly one candidate AccessList that matches
		//
		// This test asserts that an adoption failure caused by multiple
		// candidate AccessLists is handled correctly.
		{
			name:            "adoption fails with multiple candidates",
			makeResource:    mkGroup("Access List #0", 6),
			expectACLLookup: true,
			aclLookupResult: []*accesslist.AccessList{
				mkTestAccessList(t, 0, true),
				mkTestAccessList(t, 0, true),
			},
			expectError: requireAlreadyExists,
			expectValue: require.Nil,
		},

		// An access list will not be created if creating its access role would
		// overwrite an existing role
		{
			name:                          "access role overwrite is an error",
			makeResource:                  mkGroup("Access List #0", 6),
			expectACLLookup:               true,
			expectCreatingAccessListEvent: true,
			expectedRoles: map[string][]any{
				"access_list_0-access-okta-acl-role-test-new-acl": {nil, trace.AlreadyExists("That role already exists")},
			},
			expectCreateAccessList: false,
			expectError:            requireAlreadyExists,
			expectValue:            require.Nil,
		},

		// An access list will not be created if creating its reviewer role would
		// overwrite an existing role
		{
			name:                          "reviewer role overwrite is an error",
			makeResource:                  mkGroup("Access List #0", 6),
			expectACLLookup:               true,
			expectCreatingAccessListEvent: true,
			expectedRoles: map[string][]any{
				"access_list_0-access-okta-acl-role-test-new-acl":   {passThrough[types.Role]},
				"access_list_0-reviewer-okta-acl-role-test-new-acl": {nil, trace.AlreadyExists("That role already exists")},
			},
			expectCreateAccessList: false,
			expectError:            requireAlreadyExists,
			expectValue:            require.Nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			uut, fix := newTestServiceWith(t, &testFixture{
				clock: clock,
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			rigFixtureForGroupTest(fix)

			// Configure the AccessLists service with the AccessLists it should
			// return when the group handler is searching for groups to adopt.
			if tt.expectACLLookup {
				fix.accesslists.
					On("ListAccessLists", anyContext, 0, "").
					Return(tt.aclLookupResult, "", nil)
			}

			if tt.expectCreatingAccessListEvent {
				fix.shim.
					On("OnCreatingAccessList", anyContext, anyAccessList).
					Run(func(args mock.Arguments) {
						acl := getResultAs[*accesslist.AccessList](args, 1)
						require.Empty(t, acl.GetName())

						acl.Metadata.Name = newACLID
						acl.Spec.Owners = []accesslist.Owner{{Name: "test-owner"}}
					}).
					Return(nil)
			}

			if tt.expectCreateAccessList {
				fix.accesslists.
					On("UpsertAccessList", anyContext, isAccessList(tt.expectedAccessListID)).
					Run(requireValidAccessList(t)).
					Return(passThrough[*accesslist.AccessList])
			}

			if tt.expectedMembers != nil {
				fix.accesslists.
					On("UpsertAccessListWithMembers", anyContext, isAccessList(tt.expectedAccessListID), anyMemberList).
					Run(requireValidAccessListWithMembers(t, tt.expectedMembers)).
					Return(passThroughAccessListWithMembers)
			}

			for _, u := range tt.expectedMemberLookups {
				fix.users.
					On("GetUser", anyContext, u.GetName(), false).
					Return(u, nil)

				fix.shim.
					On("UserPredicate", anyContext, u).
					Return(isTestPluginUser)

				fix.shim.
					On("OnCreatingAccessListMember", anyContext, isAccessListMember(tt.expectedAccessListID, u.GetName())).
					Return(setAccessListMemberMetadata)
			}

			// Configure the roles service to expect the appropriate role
			// creations (if any)
			for roleName, createCallResult := range tt.expectedRoles {
				fix.roles.
					On("CreateRole", anyContext, isRoleNamed(roleName)).
					Run(requireRoleMetadata(t, roleName)).
					Return(createCallResult...)
			}

			resource := tt.makeResource(t)

			// When I attempt to create a new Group...
			created, err := uut.CreateSCIMResource(fix.userCtx, &scimpb.CreateSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Groups",
				},
				Resource: resource,
			})

			tt.expectError(t, err)

			if tt.expectValue != nil {
				tt.expectValue(t, created)
			}

			if tt.expectedMembers != nil {
				requireGroupResource(t, tt.expectedAccessListID, tt.expectedDisplayName, tt.expectedMembers, created)
			}
		})
	}
}

func TestGroupUpdate(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	users := mkTestUserList(t, 10, 1)

	testCases := []struct {
		name                string
		resourceID          string
		expectedUserLookups map[string]*types.UserV2
		makeResource        func(t *testing.T) *scimpb.Resource
		getACLResult        []any
		existingMemberList  []*accesslist.AccessListMember
		expectedDisplayName string
		expectedMembers     []*types.UserV2
		expectACLUpsert     bool
		expectError         require.ErrorAssertionFunc
		expectValue         require.ValueAssertionFunc
	}{
		{
			name:         "updating a non-existent group is an error",
			resourceID:   newACLID,
			makeResource: setID(newACLID, mkGroup(newACLDisplayName, 5)),
			getACLResult: []any{nil, trace.NotFound("%s", newACLID)},
			expectError:  requireNotFound,
			expectValue:  require.Nil,
		},
		{
			name:         "updating non-SCIM access list is an error",
			resourceID:   newACLID,
			makeResource: setID(newACLID, mkGroup(newACLDisplayName, 5)),
			getACLResult: []any{mkTestAccessListWithName(t, newACLID, newACLDisplayName, false), nil},
			expectError:  requireNotFound,
			expectValue:  require.Nil,
		},
		// Tests changing the AccessList display name without changing any
		// members. Implicitly asserts that no members are changed.
		{
			name:         "change display name",
			resourceID:   newACLID,
			makeResource: setID(newACLID, mkGroup("new-display-name", 2)),
			getACLResult: []any{mkTestAccessListWithName(t, newACLID, "old-display-name", true), nil},
			existingMemberList: []*accesslist.AccessListMember{
				mkTestMember(t, newACLID, 0, clock),
				mkTestMember(t, newACLID, 1, clock),
			},
			expectedUserLookups: utils.FromSlice(users[0:2], getName),
			expectACLUpsert:     true,
			expectedDisplayName: "new-display-name",
			expectedMembers:     users[0:2],
			expectError:         require.NoError,
		},
		{
			name:         "updating",
			resourceID:   newACLID,
			makeResource: setID(newACLID, mkGroup(newACLDisplayName, 5)),
			getACLResult: []any{mkTestAccessListWithName(t, newACLID, newACLDisplayName, true), nil},
			existingMemberList: []*accesslist.AccessListMember{
				mkTestMember(t, newACLID, 1, clock),
				mkTestMember(t, newACLID, 2, clock),
				mkTestMember(t, newACLID, 3, clock),
			},
			expectedDisplayName: newACLDisplayName,
			expectedUserLookups: utils.FromSlice(users[0:5], getName),
			expectACLUpsert:     true,
			expectedMembers:     users[0:5],
			expectError:         require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			uut, fix := newTestServiceWith(t, &testFixture{
				clock: clock,
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			rigFixtureForGroupTest(fix)

			// Configure the shim to convert Group into an access list
			fix.shim.
				On("ResourceToAccessList", anyContext, anyResource).
				Return(resourceToAccessList(tt.resourceID, clock)).
				Maybe()

			fix.assignments.
				On("ListOktaAssignments", anyContext, mock.Anything, "").
				Return([]types.OktaAssignment{}, "", nil).Maybe()

			// Configure the AccessList service to return what we want it to
			// (either an existing access list or a NotFound error)
			fix.accesslists.
				On("GetAccessList", anyContext, tt.resourceID).
				Return(tt.getACLResult...)

			fix.accesslists.
				On("ListAccessListMembers", anyContext, mock.Anything, 0, "").
				Return([]*accesslist.AccessListMember{}, "", nil).Maybe()

			for name, user := range tt.expectedUserLookups {
				fix.users.
					On("GetUser", anyContext, name, false).
					Return(func(_ context.Context, name string, _ bool) (types.User, error) {
						if usr := tt.expectedUserLookups[name]; usr != nil {
							return usr, nil
						}
						return nil, trace.NotFound("User not found %q", name)
					})

				if user != nil {
					fix.shim.
						On("UserPredicate", anyContext, isUserNamed(name)).
						Return(isTestPluginUser)
				}

				if isTestPluginUser(context.Background(), user) {
					fix.shim.
						On("OnCreatingAccessListMember", anyContext, isAccessListMember(tt.resourceID, name)).
						Return(setAccessListMemberMetadata)
				}
			}

			if tt.expectACLUpsert {
				fix.accesslists.
					On("UpsertAccessListWithMembers", anyContext, isAccessList(tt.resourceID), anyMemberList).
					Run(requireValidAccessListWithMembers(t, tt.expectedMembers)).
					Return(passThroughAccessListWithMembers)
			}

			resource := tt.makeResource(t)

			// When I attempt to update a Group...
			created, err := uut.UpdateSCIMResource(fix.userCtx, &scimpb.UpdateSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Groups",
					ResourceId:    tt.resourceID,
				},
				Resource: resource,
			})

			tt.expectError(t, err)

			if tt.expectValue != nil {
				tt.expectValue(t, created)
			}

			if tt.expectedMembers != nil {
				requireGroupResource(t, tt.resourceID, tt.expectedDisplayName,
					tt.expectedMembers, created)
			}
		})
	}
}

func TestGroupDelete(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	testCases := []struct {
		name                   string
		resourceID             string
		getACLResult           []any
		expectACLMemberListing bool
		aclMemberListResult    []*accesslist.AccessListMember
		expectACLDelete        bool
		expectRoleDeleteCalls  map[string]error
		expectedMemberUpdates  []string
		expectError            require.ErrorAssertionFunc
	}{
		{
			name:                  "simple delete",
			resourceID:            newACLID,
			getACLResult:          []any{mkTestAccessListWithName(t, newACLID, newACLDisplayName, true), nil},
			expectACLDelete:       true,
			expectRoleDeleteCalls: map[string]error{"test": nil, "test-reviewer": nil},
			expectError:           require.NoError,
		},
		{
			name:         "deleting a non-existent group is an error",
			resourceID:   newACLID,
			getACLResult: []any{nil, trace.NotFound("%s", newACLID)},
			expectError:  requireNotFound,
		},
		{
			name:         "deleting non-SCIM access list is an error",
			resourceID:   newACLID,
			getACLResult: []any{mkTestAccessListWithName(t, newACLID, newACLDisplayName, false), nil},
			expectError:  requireNotFound,
		},
		{
			name:            "failed role delete is not an error",
			resourceID:      newACLID,
			getACLResult:    []any{mkTestAccessListWithName(t, newACLID, newACLDisplayName, true), nil},
			expectACLDelete: true,
			expectRoleDeleteCalls: map[string]error{
				"test":          trace.NotFound("no such role"),
				"test-reviewer": nil,
			},
			expectError: require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			uut, fix := newTestServiceWith(t, &testFixture{
				clock: clock,
				modules: &modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.OktaSCIM: {Enabled: true},
						},
					},
				},
			})
			rigFixtureForGroupTest(fix)

			// Configure the AccessList service to return what we want it to
			// (either an existing access list or a NotFound error)
			fix.accesslists.
				On("GetAccessList", anyContext, tt.resourceID).
				Return(tt.getACLResult...)

			if tt.expectACLDelete {
				fix.accesslists.
					On("DeleteAccessList", anyContext, tt.resourceID).
					Return(nil)
			}

			for roleName, result := range tt.expectRoleDeleteCalls {
				fix.roles.
					On("DeleteRole", anyContext, roleName).
					Return(result)
			}

			// When I attempt to delete a Group...
			_, err := uut.DeleteSCIMResource(fix.userCtx, &scimpb.DeleteSCIMResourceRequest{
				Target: &scimpb.RequestTarget{
					Authorization: testAuthHeader,
					PluginId:      testPluginName,
					ResourceType:  "Groups",
					ResourceId:    tt.resourceID,
				},
			})

			// Examine the result and make sure its as expected
			tt.expectError(t, err)
		})
	}
}

func mkGroup(displayName string, memberCount int) func(*testing.T) *scimpb.Resource {
	return func(t *testing.T) *scimpb.Resource {
		return makeTestGroupResource(t, displayName, memberCount)
	}
}

func setID(id string, fn func(*testing.T) *scimpb.Resource) func(*testing.T) *scimpb.Resource {
	return func(t *testing.T) *scimpb.Resource {
		r := fn(t)
		r.Id = id
		return r
	}
}

func makeTestGroupResource(t *testing.T, displayName string, memberCount int) *scimpb.Resource {
	members := mkGroupMembers(memberCount)
	untypedMembers := make([]any, len(members))
	for i := range members {
		untypedMembers[i] = members[i]
	}

	resource := &scimpb.Resource{
		Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		Meta: &scimpb.Meta{
			ResourceType: "Group",
		},
		Attributes: must(structpb.NewStruct(map[string]any{
			"displayName": displayName,
			"members":     untypedMembers,
		})),
	}

	return resource
}

func userName(i int) string {
	return fmt.Sprintf("test-user-%03d@example.com", i)
}

func mkGroupMembers(n int) []map[string]any {
	members := make([]map[string]any, n)
	for i := range members {
		members[i] = map[string]any{
			"value":   userName(i),
			"display": fmt.Sprintf("User #%3d", i),
		}
	}
	return members
}

func rigFixtureSetupForSCIMAuth(fix *testFixture) {
	labels := map[string]string{"plugin": "some-string-unique-to-the-plugin"}
	fix.creds.On("GetPluginStaticCredentialsByLabels", anyContext, labels).
		Return([]types.PluginStaticCredentials{
			&types.PluginStaticCredentialsV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "test",
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
		}, nil).Maybe()
}

// rigFixtureForGroupTest configures the fixture mocks for use with all of the
// group tests
func rigFixtureForGroupTest(fix *testFixture) {
	rigFixtureSetupForSCIMAuth(fix)

	fix.shim.
		On("AccessListPredicate", anyContext, anyAccessList).
		Return(isTestPluginAccessList).
		Maybe()
	fix.shim.
		On("GetResourceLabels").
		Return(map[string]string{
			types.OriginLabel: types.OriginConfigFile,
			testUserLabel:     testUserLabelValue,
		}).
		Maybe()

	// Configure plugin lookup to return out mocked out plugin
	fix.plugins.
		On("GetPlugin", anyContext, testPluginName, withSecrets).
		Return(mkTestPlugin(), nil)
}

// anyAccessList is a testify Mock argument matcher that matches any supplied
// Teleport AccessList
var anyAccessList any = mock.MatchedBy(
	func(*accesslist.AccessList) bool { return true })

// anyMemberList is a testify Mock argument matcher that matches any supplied
// slice of supplied Teleport AccessListMembers
var anyMemberList any = mock.MatchedBy(
	func([]*accesslist.AccessListMember) bool { return true })

// isAccessLis is a testify Mock argument matcher that recognizes a
// Teleport AccessList record with a given name
func isAccessList(name string) any {
	return mock.MatchedBy(func(acl *accesslist.AccessList) bool {
		// uncomment for useful test debug output:
		// fmt.Printf(">>>> Checking if supplied %s == expected %s\n", acl.GetName(), name)
		return acl.GetName() == name
	})
}

// isAccessListMember is a testify Mock argument matcher that recognizes a
// Teleport AccessListMember record for a given AccessList + Username pair
func isAccessListMember(acl, user string) any {
	return mock.MatchedBy(func(m *accesslist.AccessListMember) bool {
		// uncomment for useful test debug output:
		// fmt.Printf(">>> Checking if supplied %s/%s == expected %s/%s\n",
		// 	m.Spec.AccessList, m.Spec.Name,
		// 	acl, user)
		return m.Spec.AccessList == acl && m.Spec.Name == user
	})
}

// isRoleNamed is a testify Mock argument matcher that recognizes a Teleport
// role with a given name
func isRoleNamed(name string) any {
	return mock.MatchedBy(func(r types.Role) bool {
		return r.GetName() == name
	})
}

// resourceToAccessList generates an AccessList from a SCIM group resource
// compatible with the test framework. In production code this is the
// responsibility of the IdP compatibility shim, so it needs mocking out for
// testing the SCIM implementation
func resourceToAccessList(id string, clock clockwork.Clock) func(context.Context, *scimpb.Resource) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	return func(_ context.Context, res *scimpb.Resource) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
		group, err := decodeGroupResource(res.Attributes.AsMap())
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		acl, err := accesslist.NewAccessList(
			header.Metadata{
				Name: id,
			},
			accesslist.Spec{
				Title: group.DisplayName,
				Owners: []accesslist.Owner{
					{Name: "test-owner"},
				},
				Grants: accesslist.Grants{
					Roles: []string{"test"},
				},
			})

		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		members := make([]*accesslist.AccessListMember, len(group.Members))
		for i, srcMember := range group.Members {
			m, err := accesslist.NewAccessListMember(
				header.Metadata{Name: srcMember.Value},
				accesslist.AccessListMemberSpec{
					AccessList: acl.GetName(),
					Name:       srcMember.Value,
					Joined:     clock.Now(),
					AddedBy:    "scim-test",
				})
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			members[i] = m
		}

		return acl, members, nil
	}
}

// requireValidAccessList validates that an AccessList and to the mock
// AccessLists service meets expectations
func requireValidAccessList(t *testing.T) func(mock.Arguments) {
	return func(args mock.Arguments) {
		acl := getResultAs[*accesslist.AccessList](args, 1)
		require.NoError(t, acl.CheckAndSetDefaults())

		require.Equal(t, types.OriginConfigFile, acl.Origin())
		labels := acl.GetStaticLabels()
		require.Contains(t, labels, testUserLabel)
		require.Equal(t, testUserLabelValue, labels[testUserLabel])

		require.NotEmpty(t, acl.Spec.Title)
		require.NotNil(t, acl.Spec.Grants, "Grants must not be nil")
		require.NotNil(t, acl.Spec.Grants.Roles, "Grant roles must not be nil")
		require.NotNil(t, acl.Spec.Grants.Traits, "Grant traits must not be nil")
	}
}

// requireRoleMetadata asserts that a Role instance passed to the mock role
// service meets expectations.
func requireRoleMetadata(t *testing.T, name string) func(mock.Arguments) {
	return func(args mock.Arguments) {
		role := getResultAs[types.Role](args, 1)
		require.Equal(t, name, role.GetName())
		require.Equal(t, types.OriginConfigFile, role.Origin())

		labels := role.GetStaticLabels()
		require.Contains(t, labels, testUserLabel)
		require.Equal(t, testUserLabelValue, labels[testUserLabel])
	}
}

// requireValidAccessListWithMembers validates that the AccessList and
// AccessListMember slice given to a mock AccessLists service meets
// expectations
func requireValidAccessListWithMembers(t *testing.T, expectedMembers []*types.UserV2) func(mock.Arguments) {
	return func(args mock.Arguments) {
		requireValidAccessList(t)(args)

		members := getResultAs[[]*accesslist.AccessListMember](args, 2)
		require.Len(t, members, len(expectedMembers))

		for _, m := range members {
			require.NotEqual(t, -1,
				slices.IndexFunc(
					expectedMembers,
					func(u *types.UserV2) bool { return u.GetName() == m.Spec.Name }),
				"Member must appear in expected members list")

			labels := m.GetStaticLabels()

			require.Contains(t, labels, testUserLabel)
			require.Equal(t, testUserLabelValue, labels[testUserLabel])

			require.Equal(t, types.OriginConfigFile, m.Origin())
			require.Equal(t, testPluginID, m.Spec.AddedBy)
		}

	}
}

func setAccessListMemberMetadata(_ context.Context, m *accesslist.AccessListMember) error {
	m.Spec.AddedBy = testPluginID
	return nil
}

func requireGroupResource(t *testing.T, id, displayName string, members []*types.UserV2, resource *scimpb.Resource) {
	require.Equal(t, id, resource.Id)

	attributes := resource.Attributes.AsMap()
	require.Equal(t, displayName, attributes["displayName"])

	expectedMembers := make([]any, 0, len(members))
	for _, m := range members {
		expectedMembers = append(expectedMembers,
			map[string]any{
				"value":   m.GetName(),
				"display": m.GetName(),
			})
	}
	require.Equal(t, expectedMembers, attributes["members"])
}

// isTestPluginAccessList is a predicate that tests if an ACL is "owned" by the
// test IdP. Used to filter out AccessLists that the SCIM service shouldn't
// manipulate. In this is the responsibility of the IdP compatibility shim.
func isTestPluginAccessList(_ context.Context, acl *accesslist.AccessList) bool {
	l, _ := acl.GetLabel(testUserLabel)
	return l == testUserLabelValue
}

// mkTestAccessList creates an AccessList record
func mkTestAccessList(t *testing.T, id int, isSCIM bool) *accesslist.AccessList {
	return mkTestAccessListWithName(
		t,
		fmt.Sprintf("Access-List-%03d", id),
		fmt.Sprintf("Access List #%d", id),
		isSCIM)
}

func mkTestAccessListWithName(t *testing.T, name, title string, isSCIM bool) *accesslist.AccessList {
	labels := map[string]string{
		testUserLabel: "banana",
	}
	if isSCIM {
		labels[types.OriginLabel] = types.OriginConfigFile
		labels[testUserLabel] = testUserLabelValue
	}

	user, err := accesslist.NewAccessList(
		header.Metadata{
			Name:     name,
			Revision: "revision-" + name,
			Labels:   labels,
		},
		accesslist.Spec{
			Title: title,
			Owners: []accesslist.Owner{
				{Name: "test-owner"},
			},
			OwnerGrants: accesslist.Grants{
				Roles:  []string{"test-reviewer"},
				Traits: trait.Traits{},
			},
			Grants: accesslist.Grants{
				Roles:  []string{"test"},
				Traits: trait.Traits{},
			},
		})
	require.NoError(t, err, "making test AccessList should succeed")
	return user
}

func mkTestAccessLists(t *testing.T, n int, spacing int) []*accesslist.AccessList {
	dst := make([]*accesslist.AccessList, n)
	for i := range n {
		dst[i] = mkTestAccessList(t, i, (i%spacing == 0))
	}
	return dst
}

func mkTestMember(t *testing.T, acl string, id int, clock clockwork.Clock) *accesslist.AccessListMember {
	name := userName(id)
	aclMember, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name:     name,
			Revision: fmt.Sprintf("revision-%03d", id),
		},
		accesslist.AccessListMemberSpec{
			AccessList: acl,
			Name:       name,
			Joined:     clock.Now(),
			AddedBy:    "scim-test",
		})
	require.NoError(t, err)
	return aclMember
}

func mkTestMembers(t *testing.T, acl *accesslist.AccessList, count int, clock clockwork.Clock) []*accesslist.AccessListMember {
	result := make([]*accesslist.AccessListMember, count)
	for i := range result {
		result[i] = mkTestMember(t, acl.GetName(), i, clock)
	}
	return result
}

// passThrough is a generic function that can be used to pass inputs through to
// outputs in a mock
func passThrough[T any](_ context.Context, value T) (T, error) {
	return value, nil
}

func passThroughAccessListWithMembers(_ context.Context, acl *accesslist.AccessList, members []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	return acl, members, nil
}

func getName(u *types.UserV2) string {
	return u.GetName()
}

// member holds a SCIM group membership record as per RFC 7643 Section 4.2
type member struct {
	Value   string `mapstructure:"value"`
	Display string `mapstructure:"display"`
}

// groupResource uses holds a parsed representation of a SCIM group resource,
// as per RFC 7643 Section 4.2
type groupResource struct {
	DisplayName string   `mapstructure:"displayName"`
	Members     []member `mapstructure:"members"`
}

// decodeGroupResource parses a SCIM group resource using `mapstructure`
func decodeGroupResource(attributes map[string]any) (groupResource, error) {
	var group groupResource
	if err := mapstructure.Decode(attributes, &group); err != nil {
		return groupResource{}, trace.Wrap(err)
	}
	return group, nil
}
