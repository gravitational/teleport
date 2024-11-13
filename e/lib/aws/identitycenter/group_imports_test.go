package identitycenter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/entitlements"
	_ "github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestStartGroupsAndGroupMembersImport(t *testing.T) {
	ctx := context.Background()
	tEnv, err := newTEnv(t)
	require.NoError(t, err)

	createRoles(t, ctx, tEnv.service.rolesSvc)
	createUsers(t, ctx, tEnv.service.usersSvc)

	var (
		account1 = &icsdk.Account{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"}
		account2 = &icsdk.Account{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"}
		// permission set ARN for assignment1 and assignment2 matches arn value returned from sdkPermSets func.
		assignment1 = &icsdk.Assigment{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"}
		assignment2 = &icsdk.Assigment{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"}
	)

	testCases := []struct {
		name         string
		existingList []listWithMembersAndRoles
		icData       icData
		expectedList []listWithMembersAndRoles
	}{
		{
			name: "new integration with a fresh Access List from group and group member imports",
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						name:        "alist1",
						id:          "alist1",
						members:     teleportUsers,
						assignments: []*icsdk.Assigment{assignment1, assignment2},
					},
					{
						name:        "alist2",
						id:          "alist2",
						members:     []string{"user1", "user2", "user3"},
						assignments: []*icsdk.Assigment{assignment1},
					},
					{
						name:        "alist3",
						id:          "alist3",
						members:     []string{"user3"},
						assignments: []*icsdk.Assigment{assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"admin-on-account1", "readonly-on-account2"},
				},
				{
					name:    "alist2",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"admin-on-account1"},
				},
				{
					name:    "alist3",
					title:   "alist3",
					members: []string{"user3"},
					roles:   []string{"readonly-on-account2"},
				},
			},
		},
		{
			name: "matching existing access list with Identiy Center groups (tests an existing integration)",
			existingList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"roleAdmin", "roleReadOnly"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist2",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"role2"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist3",
					title:   "alist3",
					members: []string{"user3"},
					roles:   []string{"role3"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						name:        "alist1",
						id:          "alist1",
						members:     teleportUsers,
						assignments: []*icsdk.Assigment{assignment1, assignment2},
					},
					{
						name:        "alist2",
						id:          "alist2",
						members:     []string{"user1", "user2", "user3"},
						assignments: []*icsdk.Assigment{assignment1},
					},
					{
						name:        "alist3",
						id:          "alist3",
						members:     []string{"user3"},
						assignments: []*icsdk.Assigment{assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"admin-on-account1", "readonly-on-account2"},
				},
				{
					name:    "alist2",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"admin-on-account1"},
				},
				{
					name:    "alist3",
					title:   "alist3",
					members: []string{"user3"},
					roles:   []string{"readonly-on-account2"},
				},
			},
		},
		{
			name: "existing access list with different origin remains unchanged",
			existingList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: []string{"user1", "user2", "user3", "user4", "user5"},
					roles:   []string{"role1", "role2"},
					origin:  common.OriginOkta,
				},
				{
					name:    "groups",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"role2"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						name:        "alist1",
						id:          "group1",
						members:     teleportUsers,
						assignments: []*icsdk.Assigment{assignment1, assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"role1", "role2"},
					origin:  common.OriginOkta,
				},
				{
					name:    "group1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"admin-on-account1", "readonly-on-account2"},
				},
			},
		},
		{
			name: "identity center group should replace existing access list of the origin OriginAWSIdentityCenter",
			existingList: []listWithMembersAndRoles{
				{
					name:    "group1",
					title:   "alist1",
					members: []string{"user1", "user2"},
					roles:   []string{"role1", "role2"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						id:          "group1",
						name:        "alist1",
						members:     []string{"user3", "user4"},
						assignments: []*icsdk.Assigment{assignment1, assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group1",
					title:   "alist1",
					members: []string{"user3", "user4"},
					roles:   []string{"admin-on-account1", "readonly-on-account2"},
				},
			},
		},
		{
			name: "identity center group members whose user account does not exist in Teleport should still be added as Access List members",
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						id:          "group2",
						name:        "alist2",
						members:     []string{"user3", "user4", "external1", "external2"},
						assignments: []*icsdk.Assigment{assignment1, assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group2",
					title:   "alist2",
					members: []string{"user3", "user4", "external1", "external2"},
					roles:   []string{"admin-on-account1", "readonly-on-account2"},
				},
			},
		},
		{
			name: "with an assigment that does not exist (faulty account and permission set map)",
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssigment{
					{
						id:          "group2",
						name:        "alist2",
						members:     []string{"user3", "user4"},
						assignments: []*icsdk.Assigment{{AccountID: "12345", PermissionSetARN: "arn:aws:sso:::permissionSet/NetworkAdmin"}},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group2",
					title:   "alist2",
					members: []string{"user3", "user4"},
					roles:   nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockState := icsdk.NewMockedAWSState()
			mockState.Accounts = tc.icData.Accounts
			mockState.PermissionSets = sdkPermSets(t)
			mockState.Users = sdkUsers(t)
			mockState.Groups = sdkGroups(t, tc.icData.Groups)
			mockState.GroupMemberships = sdkGroupMembers(t, tc.icData.Groups)
			mockState.GroupAssignments = sdkGroupAssignments(t, tc.icData.Groups)
			tEnv.setICSDKClient(icsdk.NewClientMock(&mockState))

			createAccessLists(t, ctx, tc.existingList, tEnv.service.accessListSvc)
			createAccessListMembers(t, ctx, tc.existingList, tEnv.service.accessListSvc)

			err = tEnv.service.startGroupsAndGroupMembersImport(ctx)
			require.NoError(t, err, "reconcileAccessLists")

			compareAccessLists(t, ctx, tc.expectedList, tEnv.service.accessListSvc)
		})
	}
}

func compareAccessLists(t *testing.T, ctx context.Context, expected []listWithMembersAndRoles, service services.AccessLists) {
	t.Helper()
	for _, e := range expected {
		list, err := service.GetAccessList(ctx, e.name)
		require.NoError(t, err, "compareAccessLists: GetAccessList")
		require.Equal(t, e.name, list.GetName())
		require.Equal(t, e.title, list.Spec.Title)
		require.Equal(t, e.roles, list.Spec.Grants.Roles, fmt.Sprintf("access list %q", e.name))

		membersFromBackend := listMembers(t, ctx, e.name, service)
		require.ElementsMatch(t, e.members, membersFromBackend, "access list members")
	}
}

func listMembers(t *testing.T, ctx context.Context, accesListName string, service services.AccessLists) []string {
	t.Helper()
	var out []string

	var members []*accesslist.AccessListMember
	var pageToken string
	var err error
	for {
		members, pageToken, err = service.ListAccessListMembers(ctx, accesListName, 0 /* use the default page size */, pageToken)
		require.NoError(t, err, "listMembersForACL: ListAccessListMembers")
		if pageToken == "" {
			break
		}
	}

	for _, m := range members {
		if m.GetName() == "external1" || m.GetName() == "external2" {
			require.Equal(t, common.OriginAWSIdentityCenter, m.Origin())
			require.Equal(t, m.GetAllLabels()[provisioning.ExternalIDLabel.String()], m.GetName())
		}

		out = append(out, m.GetName())
	}

	return out
}

func createAccessLists(t *testing.T, ctx context.Context, in []listWithMembersAndRoles, service services.AccessLists) {
	t.Helper()
	for _, l := range in {
		acl, err := accesslist.NewAccessList(
			header.Metadata{
				Name: l.name,
			},
			accesslist.Spec{
				Title:  l.title,
				Owners: toAclOwner(accessListDefaultOwners),
				Grants: accesslist.Grants{
					Roles: l.roles,
				},
			},
		)
		require.NoError(t, err)
		acl.SetOrigin(l.origin)
		_, err = service.UpsertAccessList(ctx, acl)
		require.NoError(t, err)
	}
}

func createAccessListMembers(t *testing.T, ctx context.Context, in []listWithMembersAndRoles, service services.AccessLists) {
	t.Helper()
	for _, l := range in {
		for _, m := range l.members {
			alm, err := accesslist.NewAccessListMember(
				header.Metadata{
					Name: m,
				},
				accesslist.AccessListMemberSpec{
					AccessList: l.name,
					Name:       m,
					Joined:     time.Now().UTC(),
					AddedBy:    teleport.UserSystem,
				},
			)
			require.NoError(t, err)

			_, err = service.UpsertAccessListMember(ctx, alm)
			require.NoError(t, err)
		}
	}
}

var (
	teleportUsers = []string{"user1", "user2", "user3", "user4", "user5"}
	icUsers       = []string{"user1", "user2", "user3", "user4", "user5", "external1", "external2"}
)

func sdkUsers(t *testing.T) []*icsdk.User {
	t.Helper()
	out := make([]*icsdk.User, 0, len(icUsers))
	for _, u := range icUsers {
		out = append(out, &icsdk.User{
			ID:       u,
			UserName: u,
		})
	}

	return out
}

func sdkGroups(t *testing.T, in []groupWithMemberAndAssigment) []*icsdk.Group {
	t.Helper()
	out := make([]*icsdk.Group, 0, len(in))
	for _, g := range in {
		out = append(out, &icsdk.Group{
			ID:          g.id,
			DisplayName: g.name,
		})
	}

	return out
}

func sdkGroupMembers(t *testing.T, in []groupWithMemberAndAssigment) map[string][]*icsdk.GroupMember {
	t.Helper()
	out := make(map[string][]*icsdk.GroupMember)
	for _, g := range in {
		members := make([]*icsdk.GroupMember, 0, len(g.members))
		for _, m := range g.members {
			members = append(members, &icsdk.GroupMember{
				MemberID: m,
			})
		}
		out[g.id] = members
	}

	return out
}

func sdkPermSets(t *testing.T) []*icsdk.PermissionSet {
	t.Helper()
	return []*icsdk.PermissionSet{
		{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
		{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
	}
}

func sdkGroupAssignments(t *testing.T, in []groupWithMemberAndAssigment) map[string][]*icsdk.Assigment {
	t.Helper()
	out := make(map[string][]*icsdk.Assigment)
	for _, g := range in {
		out[g.id] = g.assignments
	}

	return out
}

type icData struct {
	Accounts []*icsdk.Account
	Groups   []groupWithMemberAndAssigment
}

type groupWithMemberAndAssigment struct {
	name        string
	id          string
	members     []string
	assignments []*icsdk.Assigment
}

type listWithMembersAndRoles struct {
	name    string
	title   string
	members []string
	roles   []string
	origin  string
}

// tEnv is a test service for Identity Center service
type tEnv struct {
	service *Service
}

func newTEnv(t *testing.T) (*tEnv, error) {
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	newAccessListService, err := local.NewAccessListService(backend, clock)
	require.NoError(t, err)

	usersService, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)

	roleService := local.NewAccessService(backend)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
			Cloud: true,
		},
	})

	return &tEnv{
		service: &Service{
			accessListSvc: newAccessListService,
			usersSvc:      usersService,
			rolesSvc:      roleService,
			log:           slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
			importConfig: ImportConfig{
				AccessListDefaultOwners: accessListDefaultOwners,
			},
		},
	}, nil
}

func (e tEnv) setICSDKClient(client icsdk.Client) {
	e.service.icClient = client
}

var accessListDefaultOwners = []string{"user1", "user2"}

var roles = []string{"role1", "role2", "roleAdmin", "roleReadOnly"}

func createRoles(t *testing.T, ctx context.Context, service RolesService) {
	for _, r := range roles {
		role, err := types.NewRole(r, types.RoleSpecV6{})
		require.NoError(t, err)
		_, err = service.CreateRole(ctx, role)
		require.NoError(t, err)
	}
}

func createUsers(t *testing.T, ctx context.Context, userService UsersService) {
	t.Helper()
	for i := 1; i <= 5; i++ {
		u, err := types.NewUser("user" + strconv.Itoa(i))
		require.NoError(t, err)
		_, err = userService.CreateUser(ctx, u)
		require.NoError(t, err)
	}
}
