package accessrequest

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
)

func TestScoreRelevance(t *testing.T) {
	t.Parallel()

	makeAccessList := func(roles ...string) *accesslist.AccessList {
		return &accesslist.AccessList{
			Spec: accesslist.Spec{
				Grants: accesslist.Grants{Roles: roles},
			},
		}
	}

	type args struct {
		request types.AccessRequest
		lists   []*accesslist.AccessList
	}
	tests := []struct {
		name string
		args args
		want []*accesslist.AccessList
	}{
		{
			name: "no lists",
			args: args{
				request: &types.AccessRequestV3{},
				lists:   []*accesslist.AccessList{},
			},
			want: []*accesslist.AccessList{},
		},
		{
			name: "one role",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
			},
		},
		{
			name: "irrelevant roles are penalized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1", "role2"),
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
				makeAccessList("role1", "role2"),
			},
		},
		{
			name: "relevant roles are prioritized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1", "role3"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role100", "role101"),
					makeAccessList("role1", "role3"),
					makeAccessList("role1", "role2"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1", "role3"),
				makeAccessList("role1", "role2"),
				makeAccessList("role100", "role101"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreRelevance(tt.args.request, tt.args.lists)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateAccessRequestPromotions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		currentResources   []testNodeDesc
		currentRoles       []testRoleDesc
		currentUsers       []testUserDesc
		currentAccessLists []testAccessListDesc
		accessRequest      testAccessRequestDesc
		expectedPromotions []string
	}{
		{
			name: "Access List is promoted when it allows requested resource",
			currentResources: []testNodeDesc{
				{name: "server1", labels: map[string]string{"server": "1"}},
				{name: "server3000", labels: map[string]string{"server": "3000"}},
			},
			currentRoles: []testRoleDesc{
				{name: "server1-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"1"}}}},
				{name: "server3000-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"3000"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "access-list1",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server1-access"},
				},
				{
					name:         "access-list3000",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server3000-access"},
				},
			},
			accessRequest: testAccessRequestDesc{
				name:             "test-access-request",
				user:             "user1",
				requestedServers: []string{"server1"},
			},
			expectedPromotions: []string{
				"access-list1",
			},
		},
		{
			name: "Access List is not promoted when it does not allow requested resource",
			currentResources: []testNodeDesc{
				{name: "server1", labels: map[string]string{"server": "1"}},
				{name: "server3000", labels: map[string]string{"server": "3000"}},
			},
			currentRoles: []testRoleDesc{
				{name: "server1-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"1"}}}},
				{name: "server3000-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"3000"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "access-list3000",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server3000-access"},
				},
				{
					name:         "access-list3000-again",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server3000-access"},
				},
			},
			accessRequest: testAccessRequestDesc{
				name:             "test-access-request",
				user:             "user1",
				requestedServers: []string{"server1"},
			},
			expectedPromotions: []string{
				// no promotions
			},
		},
		{
			name: "Access List is not promoted the user is a member of it",
			currentResources: []testNodeDesc{
				{name: "server1", labels: map[string]string{"server": "1"}},
				{name: "server3000", labels: map[string]string{"server": "3000"}},
			},
			currentRoles: []testRoleDesc{
				{name: "server1-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"1"}}}},
				{name: "server3000-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"3000"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "access-list1",
					owners:       []string{"test-owner"},
					members:      []string{"user1"},
					grantedRoles: []string{"server1-access"},
				},
				{
					name:         "access-list3000-again",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server3000-access"},
				},
			},
			accessRequest: testAccessRequestDesc{
				name:             "test-access-request",
				user:             "user1",
				requestedServers: []string{"server1"},
			},
			expectedPromotions: []string{
				// no promotions
			},
		},
		{
			name: "Access List is promoted when user is already able to access the resource",
			currentResources: []testNodeDesc{
				{name: "server1", labels: map[string]string{"server": "1"}},
				{name: "server3000", labels: map[string]string{"server": "3000"}},
			},
			currentRoles: []testRoleDesc{
				{name: "server1-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"1"}}}},
				{name: "server3000-access", allow: types.RoleConditions{NodeLabels: types.Labels{"server": []string{"3000"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{"server1-access"}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "access-list1",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server1-access"},
				},
				{
					name:         "access-list3000",
					owners:       []string{"test-owner"},
					members:      []string{},
					grantedRoles: []string{"server3000-access"},
				},
			},
			accessRequest: testAccessRequestDesc{
				name:             "test-access-request",
				user:             "user1",
				requestedServers: []string{"server1"},
			},
			expectedPromotions: []string{
				"access-list1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			g := &mockAccessResourcesGetter{}
			testSetupNodes(t, g, tc.currentResources)
			testSetupRoles(t, g, tc.currentRoles)
			testSetupUsers(t, g, tc.currentUsers)
			testSetupAccessListsWithMembers(t, g, tc.currentAccessLists)

			ar := testNewAccessRequest(t, tc.accessRequest)
			promotions, err := GenerateAccessRequestPromotions(ctx, g, ar)
			require.NoError(t, err)

			var actualPromotions []string
			for _, p := range promotions.Promotions {
				actualPromotions = append(actualPromotions, p.AccessListName)
			}
			require.ElementsMatch(t, tc.expectedPromotions, actualPromotions)
		})
	}

}

const testClusterName = "test-cluster"

func TestGenerateLongTermResourceGrouping(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name               string
		currentResources   []testNodeDesc
		currentRoles       []testRoleDesc
		currentUsers       []testUserDesc
		currentAccessLists []testAccessListDesc
		requestedIDs       []types.ResourceID
		expectedCanProceed bool
		expectedMessage    string
	}{
		{
			name: "only one resource is covered by a valid access list",
			currentResources: []testNodeDesc{
				{name: "dev-node", labels: map[string]string{"env": "dev"}},
				{name: "prod-node", labels: map[string]string{"env": "prod"}},
			},
			currentRoles: []testRoleDesc{
				{name: "dev-access", allow: types.RoleConditions{NodeLabels: types.Labels{"env": []string{"dev"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "dev-list",
					owners:       []string{"admin"},
					grantedRoles: []string{"dev-access"},
				},
			},
			requestedIDs: []types.ResourceID{
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "dev-node"},
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "prod-node"},
			},
			expectedCanProceed: false,
			expectedMessage:    "Long-term access is not available for some selected resources",
		},
		{
			name: "all resources covered by same access list",
			currentResources: []testNodeDesc{
				{name: "dev-node", labels: map[string]string{"env": "dev"}},
				{name: "dev-node-2", labels: map[string]string{"env": "dev"}},
			},
			currentRoles: []testRoleDesc{
				{name: "dev-access", allow: types.RoleConditions{NodeLabels: types.Labels{"env": []string{"dev"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "dev-list",
					owners:       []string{"admin"},
					grantedRoles: []string{"dev-access"},
				},
			},
			requestedIDs: []types.ResourceID{
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "dev-node"},
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "dev-node-2"},
			},
			expectedCanProceed: true,
			expectedMessage:    "",
		},
		{
			name: "resources split across multiple access lists",
			currentResources: []testNodeDesc{
				{name: "dev-node", labels: map[string]string{"env": "dev"}},
				{name: "qa-node", labels: map[string]string{"env": "qa"}},
			},
			currentRoles: []testRoleDesc{
				{name: "dev-access", allow: types.RoleConditions{NodeLabels: types.Labels{"env": []string{"dev"}}}},
				{name: "qa-access", allow: types.RoleConditions{NodeLabels: types.Labels{"env": []string{"qa"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "dev-list",
					owners:       []string{"admin"},
					grantedRoles: []string{"dev-access"},
				},
				{
					name:         "qa-list",
					owners:       []string{"admin"},
					grantedRoles: []string{"qa-access"},
				},
			},
			requestedIDs: []types.ResourceID{
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "dev-node"},
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "qa-node"},
			},
			expectedCanProceed: false,
			expectedMessage:    "Selected resources cannot be grouped for long-term access",
		},
		{
			name: "resources split across multiple clusters",
			currentResources: []testNodeDesc{
				{name: "prod-node", labels: map[string]string{"env": "prod"}},
			},
			currentRoles: []testRoleDesc{
				{name: "prod-access", allow: types.RoleConditions{NodeLabels: types.Labels{"env": []string{"prod"}}}},
			},
			currentUsers: []testUserDesc{
				{name: "user1", roles: []string{}},
			},
			currentAccessLists: []testAccessListDesc{
				{
					name:         "prod-access",
					owners:       []string{"admin"},
					grantedRoles: []string{"prod-access"},
				},
			},
			requestedIDs: []types.ResourceID{
				// This node won't exist, but we shouldn't ever get far enough for that to matter.
				{ClusterName: "dev-cluster", Kind: types.KindNode, Name: "dev-node"},
				{ClusterName: testClusterName, Kind: types.KindNode, Name: "prod-node"},
			},
			expectedCanProceed: false,
			expectedMessage:    "Long-term access is not available for resources in different clusters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &mockAccessResourcesGetter{}
			testSetupNodes(t, g, tc.currentResources)
			testSetupRoles(t, g, tc.currentRoles)
			testSetupUsers(t, g, tc.currentUsers)
			testSetupAccessListsWithMembers(t, g, tc.currentAccessLists)

			// All we need is a user and requested IDs
			stubRequest := &types.AccessRequestV3{
				Spec: types.AccessRequestSpecV3{
					User:                 "user1",
					RequestedResourceIDs: tc.requestedIDs,
				},
			}

			suggestion, err := GenerateLongTermResourceGrouping(ctx, g, stubRequest)
			require.NoError(t, err)
			require.Equal(t, tc.expectedCanProceed, suggestion.CanProceed)
			if !tc.expectedCanProceed {
				require.Equal(t, tc.expectedMessage, suggestion.ValidationMessage)
			}
		})
	}
}

type testAccessRequestDesc struct {
	name             string
	user             string
	requestedServers []string
}

func testNewAccessRequest(t *testing.T, desc testAccessRequestDesc) types.AccessRequest {
	t.Helper()

	var resourcesIDs []types.ResourceID
	for _, n := range desc.requestedServers {
		id := types.ResourceID{
			ClusterName: "test-cluster",
			Kind:        types.KindNode,
			Name:        n,
		}
		resourcesIDs = append(resourcesIDs, id)
	}
	ar, err := types.NewAccessRequestWithResources(desc.name, desc.user, []string{}, resourcesIDs)
	require.NoError(t, err, "types.NewAccessRequest")

	return ar
}

type testUserDesc struct {
	name  string
	roles []string
}

func testSetupUser(t *testing.T, g *mockAccessResourcesGetter, desc testUserDesc) {
	t.Helper()

	if g.users == nil {
		g.users = make(map[string]types.User)
	}

	u, err := types.NewUser(desc.name)
	require.NoError(t, err, "types.NewUser")

	u.SetRoles(desc.roles)

	g.users[u.GetName()] = u
}

func testSetupUsers(t *testing.T, g *mockAccessResourcesGetter, descs []testUserDesc) {
	t.Helper()
	for _, d := range descs {
		testSetupUser(t, g, d)
	}
}

type testNodeDesc struct {
	name   string
	labels map[string]string
}

func testSetupNode(t *testing.T, g *mockAccessResourcesGetter, desc testNodeDesc) {
	t.Helper()

	s, err := types.NewServerWithLabels(desc.name, types.KindNode, types.ServerSpecV2{}, desc.labels)
	require.NoError(t, err, "types.NewServerWithLabels")

	g.resources = append(g.resources, s)
}

func testSetupNodes(t *testing.T, g *mockAccessResourcesGetter, descs []testNodeDesc) {
	t.Helper()
	for _, d := range descs {
		testSetupNode(t, g, d)
	}
}

type testRoleDesc struct {
	name  string
	allow types.RoleConditions
}

func testSetupRole(t *testing.T, g *mockAccessResourcesGetter, desc testRoleDesc) {
	t.Helper()

	if g.roles == nil {
		g.roles = make(map[string]types.Role)
	}

	r, err := types.NewRole(
		desc.name,
		types.RoleSpecV6{
			Allow: desc.allow,
		},
	)
	require.NoError(t, err, "types.NewRole")

	g.roles[r.GetName()] = r
}

func testSetupRoles(t *testing.T, g *mockAccessResourcesGetter, descs []testRoleDesc) {
	t.Helper()
	for _, d := range descs {
		testSetupRole(t, g, d)
	}
}

type testAccessListDesc struct {
	name         string
	owners       []string
	members      []string
	grantedRoles []string
}

func testSetupAccessListWithMembers(t *testing.T, g *mockAccessResourcesGetter, desc testAccessListDesc) {
	t.Helper()

	if g.accessLists == nil {
		g.accessLists = make(map[string]*accesslist.AccessList)
	}
	if g.accessListMembers == nil {
		g.accessListMembers = make(map[string]map[string]*accesslist.AccessListMember)
	}

	var owners []accesslist.Owner
	for _, o := range desc.owners {
		owners = append(owners, accesslist.Owner{Name: o})
	}
	al, err := accesslist.NewAccessList(
		header.Metadata{
			Name: desc.name,
		},
		accesslist.Spec{
			Title:  desc.name,
			Owners: owners,
			Grants: accesslist.Grants{Roles: desc.grantedRoles},
		},
	)
	require.NoError(t, err, "accesslist.NewAccessList")
	g.accessLists[al.GetName()] = al

	if g.accessListMembers[al.GetName()] == nil {
		g.accessListMembers[al.GetName()] = make(map[string]*accesslist.AccessListMember)
	}

	for _, m := range desc.members {
		member, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: m,
			},
			accesslist.AccessListMemberSpec{
				AccessList: desc.name,
				Name:       m,
				AddedBy:    "does-not-matter-added-by",
				Joined:     time.Now().Add(-1 * time.Hour),
			},
		)
		require.NoError(t, err, "accesslist.NewAccessListMember")
		g.accessListMembers[al.GetName()][member.GetName()] = member
	}
}

func testSetupAccessListsWithMembers(t *testing.T, g *mockAccessResourcesGetter, descs []testAccessListDesc) {
	t.Helper()
	for _, d := range descs {
		testSetupAccessListWithMembers(t, g, d)
	}
}

type mockAccessResourcesGetter struct {
	resources         []types.ResourceWithLabels
	users             map[string]types.User
	roles             map[string]types.Role
	accessLists       map[string]*accesslist.AccessList
	accessListMembers map[string]map[string]*accesslist.AccessListMember // map[accessListName]map[memberName]
}

func (g *mockAccessResourcesGetter) ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error) {
	return slices.Collect(maps.Values(g.accessLists)), "", nil
}

func (g *mockAccessResourcesGetter) ListResources(_ context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	var resources []types.ResourceWithLabels
	if req.PredicateExpression == "" {
		resources = g.resources
	} else {
		filter, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			panic(err)
		}
		for _, r := range g.resources {
			ok, err := filter.Evaluate(r)
			if err != nil {
				panic(err)
			}
			if ok {
				resources = append(resources, r)
			}
		}
	}

	return &types.ListResourcesResponse{
		Resources:  resources,
		NextKey:    "",
		TotalCount: len(g.resources),
	}, nil
}

func (g *mockAccessResourcesGetter) GetAccessList(_ context.Context, name string) (*accesslist.AccessList, error) {
	v, err := getMockValue(name, g.accessLists)
	return v, trace.Wrap(err)
}

func (g *mockAccessResourcesGetter) GetAccessLists(_ context.Context) ([]*accesslist.AccessList, error) {
	return slices.Collect(maps.Values(g.accessLists)), nil
}

func (g *mockAccessResourcesGetter) ListAccessListMembers(_ context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	v, err := getMockValue(accessList, g.accessListMembers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return slices.Collect(maps.Values(v)), "", nil
}

func (g *mockAccessResourcesGetter) GetAccessListMember(_ context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	v1, err := getMockValue(accessList, g.accessListMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v2, err := getMockValue(memberName, v1)
	return v2, trace.Wrap(err)
}

func (g *mockAccessResourcesGetter) GetUser(_ context.Context, userName string, withSecrets bool) (types.User, error) {
	if withSecrets {
		return nil, trace.NotImplemented("mockAccessResourcesGetter.GetUser( withSecrets = true )")
	}

	v, err := getMockValue(userName, g.users)
	return v, trace.Wrap(err)
}

func (g *mockAccessResourcesGetter) GetRole(_ context.Context, name string) (types.Role, error) {
	v, err := getMockValue(name, g.roles)
	return v, trace.Wrap(err)
}

func (g *mockAccessResourcesGetter) GetLock(_ context.Context, name string) (types.Lock, error) {
	return nil, trace.NotImplemented("mockAccessResourcesGetter.GetLock")
}

func (g *mockAccessResourcesGetter) GetLocks(_ context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	return nil, trace.NotImplemented("mockAccessResourcesGetter.GetLocks")
}

func getMockValue[T any](k string, m map[string]T) (v T, err error) {
	if m == nil {
		return v, trace.NotImplemented("map of type %T not set in the mock", m)
	}
	if v, ok := m[k]; ok {
		return v, nil
	}
	return v, trace.NotFound("value.(%T) for key=%q not found", v, k)
}
