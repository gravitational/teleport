package identitycenter

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	icfixture "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

func TestGroupImportAndEmitStatus(t *testing.T) {
	ctx := context.Background()

	var (
		account1 = &icsdk.Account{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"}
		account2 = &icsdk.Account{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"}
		// permission set ARN for assignment1 and assignment2 matches arn value returned from sdkPermSets func.
		assignment1 = &icsdk.Assignment{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"}
		assignment2 = &icsdk.Assignment{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"}
	)

	testCases := []struct {
		name          string
		rolesSyncMode RolesSyncMode
		existingList  []listWithMembersAndRoles
		icData        icData
		expectedList  []listWithMembersAndRoles
		groupFilters  icfilters.Filters
	}{
		{
			name:          "new integration with a fresh Access List from group and group member imports",
			rolesSyncMode: RolesSyncModeAll,
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssignment{
					{
						name:        "alist1",
						id:          "alist1",
						members:     teleportUsers,
						assignments: []*icsdk.Assignment{assignment1, assignment2},
					},
					{
						name:        "alist2",
						id:          "alist2",
						members:     []string{"user1", "user2", "user3"},
						assignments: []*icsdk.Assignment{assignment1},
					},
					{
						name:        "alist3",
						id:          "alist3",
						members:     []string{"user3"},
						assignments: []*icsdk.Assignment{assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"admin-on-account1-1111111111", "readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist2",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"admin-on-account1-1111111111"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist3",
					title:   "alist3",
					members: []string{"user3"},
					roles:   []string{"readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "matching existing access list with Identity Center groups (tests an existing integration)",
			rolesSyncMode: RolesSyncModeAll,
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
				Groups: []groupWithMemberAndAssignment{
					{
						name:        "alist1",
						id:          "alist1",
						members:     teleportUsers,
						assignments: []*icsdk.Assignment{assignment1, assignment2},
					},
					{
						name:        "alist2",
						id:          "alist2",
						members:     []string{"user1", "user2", "user3"},
						assignments: []*icsdk.Assignment{assignment1},
					},
					{
						name:        "alist3",
						id:          "alist3",
						members:     []string{"user3"},
						assignments: []*icsdk.Assignment{assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "alist1",
					title:   "alist1",
					members: teleportUsers,
					roles:   []string{"admin-on-account1-1111111111", "readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist2",
					title:   "alist2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"admin-on-account1-1111111111"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "alist3",
					title:   "alist3",
					members: []string{"user3"},
					roles:   []string{"readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "existing access list with different origin remains unchanged",
			rolesSyncMode: RolesSyncModeAll,
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
				Groups: []groupWithMemberAndAssignment{
					{
						name:        "alist1",
						id:          "group1",
						members:     teleportUsers,
						assignments: []*icsdk.Assignment{assignment1, assignment2},
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
					roles:   []string{"admin-on-account1-1111111111", "readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "identity center group should replace existing access list of the origin OriginAWSIdentityCenter",
			rolesSyncMode: RolesSyncModeAll,
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
				Groups: []groupWithMemberAndAssignment{
					{
						id:          "group1",
						name:        "alist1",
						members:     []string{"user3", "user4"},
						assignments: []*icsdk.Assignment{assignment1, assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group1",
					title:   "alist1",
					members: []string{"user3", "user4"},
					roles:   []string{"admin-on-account1-1111111111", "readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "identity center group members whose user account does not exist in Teleport should still be added as Access List members",
			rolesSyncMode: RolesSyncModeAll,
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssignment{
					{
						id:          "group2",
						name:        "alist2",
						members:     []string{"user3", "user4", "external1", "external2"},
						assignments: []*icsdk.Assignment{assignment1, assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group2",
					title:   "alist2",
					members: []string{"external1", "external2", "user3", "user4"},
					roles:   []string{"admin-on-account1-1111111111", "readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "with an assignment that does not exist (faulty account and permission set map)",
			rolesSyncMode: RolesSyncModeAll,
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssignment{
					{
						id:          "group2",
						name:        "alist2",
						members:     []string{"user3", "user4"},
						assignments: []*icsdk.Assignment{{AccountID: "12345", PermissionSetARN: "arn:aws:sso:::permissionSet/NetworkAdmin"}},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "group2",
					title:   "alist2",
					members: []string{"user3", "user4"},
					roles:   nil,
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
		{
			name:          "groups import with filters",
			rolesSyncMode: RolesSyncModeAll,
			groupFilters: icfilters.Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "id2"}},
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "acl3"}},
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "acl7"}},
			},
			icData: icData{
				Accounts: []*icsdk.Account{account1, account2},
				Groups: []groupWithMemberAndAssignment{
					{
						name:        "acl1",
						id:          "id1",
						members:     teleportUsers,
						assignments: []*icsdk.Assignment{assignment1, assignment2},
					},
					{
						name:        "acl2",
						id:          "id2",
						members:     []string{"user1", "user2", "user3"},
						assignments: []*icsdk.Assignment{assignment1},
					},
					{
						name:        "acl3",
						id:          "id3",
						members:     []string{"user3"},
						assignments: []*icsdk.Assignment{assignment2},
					},
				},
			},
			expectedList: []listWithMembersAndRoles{
				{
					name:    "id2",
					title:   "acl2",
					members: []string{"user1", "user2", "user3"},
					roles:   []string{"admin-on-account1-1111111111"},
					origin:  common.OriginAWSIdentityCenter,
				},
				{
					name:    "id3",
					title:   "acl3",
					members: []string{"user3"},
					roles:   []string{"readonly-on-account2-2222222222"},
					origin:  common.OriginAWSIdentityCenter,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockState := icsdk.NewMockedAWSState(
				icsdk.WithAccounts(tc.icData.Accounts...),
				icsdk.WithPermissionSets(sdkPermSets(t)...),
				icsdk.WithUsers(sdkUsers(t)...),
				icsdk.WithGroups(sdkGroups(t, tc.icData.Groups)...),
				icsdk.WithGroupMemberships(sdkGroupMembers(t, tc.icData.Groups)),
				icsdk.WithGroupAssignments(sdkGroupAssignments(t, tc.icData.Groups)),
			)

			statusSink := &integration.FakeStatusSink{}

			fixture := icfixture.NewFixture(t,
				icfixture.WithStatusSink(statusSink),
				icfixture.WithAWSState(&mockState))

			fixture.CreatePluginResource(t, icfixture.WithGroupFilters(tc.groupFilters))
			createRoles(t, ctx, fixture.Auth)
			createUsers(t, ctx, fixture.Auth)
			createAccessLists(t, ctx, tc.existingList, fixture.Auth)
			createAccessListMembers(t, ctx, tc.existingList, fixture.Auth)

			svc := newTestService(t, fixture,
				withRolesSyncMode(tc.rolesSyncMode),
				withLogger(slog.Default().With("test", t.Name())))

			err := svc.importAndEmitStatus(ctx)
			require.NoError(t, err)

			compareAccessLists(t, ctx, tc.expectedList, fixture.Auth.AccessLists)

			require.Eventually(t, func() bool {
				return statusSink.Get() != nil
			}, time.Second, time.Second/100)
			require.Equal(t, types.PluginStatusCode_RUNNING, statusSink.Get().GetCode())
			require.NotNil(t, statusSink.Get().GetAwsIc())
			require.Equal(t, types.AWSICGroupImportStatusCode_DONE, statusSink.Get().GetAwsIc().GroupImportStatus.StatusCode)

			expectResourceSyncEvent(t, fixture.Emitter, func(e *apievents.AWSICResourceSync) {
				require.Equal(t, events.AWSICResourceSyncSuccessCode, e.GetCode())
				require.Equal(t, events.AWSICResourceSyncSuccessEvent, e.GetType())
				require.Equal(t, countICOriginatedList(tc.expectedList), int(e.TotalUserGroups))
			})
		})
	}
}

// TestGroupImportTriggers asserts that groups are imported from AWS
// if GroupImportStatus.StatusCode is not AWSICGroupImportStatusCode_DONE.
// By design, the group import is only performed once, except when the user wants to update
// the group filter. In that case, REIMPORT_REQUESTED status is used to override group
// status code, which re-triggers group import.
func TestGroupImportTriggers(t *testing.T) {
	fixture := icfixture.NewFixture(t, icfixture.WithStartedCache)
	ctx := fixture.Ctx
	fixture.CreatePluginResource(t,
		icfixture.WithGroupFilters(icfilters.Filters{
			// import exactly one group.
			{Include: &types.AWSICResourceFilter_Id{Id: "group1"}},
		}))

	testCases := []struct {
		name            string
		updatePluginReq *types.PluginV1
		statusCode      types.AWSICGroupImportStatusCode
		expectImport    bool
	}{
		{
			name:         "done status skips import",
			statusCode:   types.AWSICGroupImportStatusCode_DONE,
			expectImport: false,
		},
		{
			name:         "default empty group import status triggers import",
			expectImport: true,
		},
		{
			name:         "error group import status triggers import",
			statusCode:   types.AWSICGroupImportStatusCode_FAILED,
			expectImport: true,
		},
		{
			name:         "reimport requested status triggers import",
			statusCode:   types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED,
			expectImport: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture.MustGetPluginResource(t)
			p.Status = types.PluginStatusV1{
				Code: types.PluginStatusCode_RUNNING,
				Details: &types.PluginStatusV1_AwsIc{
					AwsIc: &types.PluginAWSICStatusV1{
						GroupImportStatus: &types.AWSICGroupImportStatus{
							StatusCode: tc.statusCode,
						},
					},
				},
			}
			_, err := fixture.PluginService.UpdatePlugin(ctx, p)
			require.NoError(t, err)
			// runs the main sync cycle that covers group, account,
			// permission set and account assignment sync.
			_, stopService := runNewTestService(t, ctx, fixture)
			if tc.expectImport {
				expectResourceSyncEvent(t, fixture.Emitter, func(e *apievents.AWSICResourceSync) {
					require.Equal(t, int32(1), e.TotalUserGroups)
				})

			} else {
				expectResourceSyncEvent(t, fixture.Emitter, func(e *apievents.AWSICResourceSync) {
					require.Equal(t, int32(0), e.TotalUserGroups)
				})
			}
			fixture.ResetEmitter()
			stopService()
		})
	}
}

func requireEventually(t *testing.T, condition func(collect *assert.CollectT)) {
	const (
		defaultWaitFor = time.Second * 10
		defaultTick    = time.Millisecond * 50
	)
	require.EventuallyWithT(t, condition, defaultWaitFor, defaultTick)
}

func downstreamGroupsMatch(ctx context.Context, icClient icsdk.Client, expectedGroups ...string) func(*assert.CollectT) {
	return func(collect *assert.CollectT) {
		gs, err := icClient.ListGroups(ctx)
		if !assert.NoError(collect, err) {
			return
		}
		assert.ElementsMatch(collect, expectedGroups, collectGroupIDs(gs))
	}
}

func accessListsMatch(ctx context.Context, svc services.AccessListsGetter, expectedACLs ...string) func(*assert.CollectT) {
	return func(collect *assert.CollectT) {
		var actualACLs []string
		for acl, err := range iciter.AllAccessLists(ctx, svc) {
			if !assert.NoError(collect, err) {
				return
			}
			actualACLs = append(actualACLs, acl.GetName())
		}
		assert.ElementsMatch(collect, expectedACLs, actualACLs)
	}
}

func accessListsAreMarkedAsProvisioned(ctx context.Context, svc services.DownstreamProvisioningStateGetter, expectedGroups ...string) func(*assert.CollectT) {
	return func(collect *assert.CollectT) {
		for _, gid := range expectedGroups {
			pState, err := svc.GetProvisioningState(ctx, IdentityCenterDownstreamID, services.ProvisioningStateID("acl-"+gid))
			if !assert.NoError(collect, err, "No provisioning state exists for ACL %s", gid) {
				return
			}
			assert.Equal(collect,
				provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
				pState.GetStatus().GetProvisioningState(),
				"Provisioning state for ACL %q is %s", gid, pState.GetStatus().GetProvisioningState().String())
		}
	}
}

func accessListsHaveNoProvisioningState(ctx context.Context, svc services.DownstreamProvisioningStateGetter, expectedGroups ...string) func(*assert.CollectT) {
	return func(collect *assert.CollectT) {
		for _, gid := range expectedGroups {
			_, err := svc.GetProvisioningState(ctx, IdentityCenterDownstreamID, services.ProvisioningStateID("acl-"+gid))
			if !assert.True(collect, trace.IsNotFound(err), "No provisioning state must exist for ACL %s", gid) {
				return
			}
		}
	}
}

func TestGroupDeletesAreSuppressed(t *testing.T) {
	logger := slog.Default()
	ctx := t.Context()

	// GIVEN an Identity Center service managing several Access Lists that were
	// imported from the downstream Identity Center instance
	awsICState := icsdk.NewMockedAWSState(
		icsdk.WithUser("alice", "alice@example.com"),
		icsdk.WithUser("bob", "bob@example.com"),
		icsdk.WithUser("carol", "carol@example.com"),
		icsdk.WithGroup("alpha", "Alpha", "alice", "bob", "carol"),
		icsdk.WithGroup("bravo", "Bravo", "alice", "bob", "carol"),
		icsdk.WithGroup("charlie", "Charlie", "alice", "bob", "carol"),
		icsdk.WithGroup("delta", "Delta", "alice", "bob", "carol"),
	)
	fixture := icfixture.NewFixture(t,
		icfixture.WithAWSState(&awsICState),
		icfixture.WithStartedCache,
		icfixture.WithWriteThroughStatusSink)

	fixture.CreatePluginResource(t)

	_, stopService := runNewTestService(t, ctx, fixture, withLogger(logger.With("phase", "setup")))
	defer stopService()

	allGroups := []string{"alpha", "bravo", "charlie", "delta"}
	requireEventually(t, downstreamGroupsMatch(ctx, fixture.ICClient, allGroups...))

	// On the first run through, the underlying SCIM provisioner has to adopt
	// the downstream groups before the IC permissions calculator and provisioner
	// even get started on them. This can take a while in resource-constrained
	// environments (like the flaky test detector), so we need to give
	// this assertion a bit more time than the regular `requireEventually()`
	require.EventuallyWithT(t,
		accessListsAreMarkedAsProvisioned(ctx, fixture.Auth, allGroups...),
		30*time.Second,
		100*time.Millisecond)
	stopService()

	// WHEN I change the Identity Center group import filters such that only two
	// of the downstream groups are imported, and start the service (implicitly
	// triggering a new import), and re-start the service
	pr := fixture.MustGetPluginResource(t)
	pr.Spec.GetAwsIc().GroupSyncFilters = icfilters.Filters{
		{Include: &types.AWSICResourceFilter_Id{Id: "bravo"}},
		{Include: &types.AWSICResourceFilter_Id{Id: "delta"}},
	}
	pr.Status.GetAwsIc().GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED
	fixture.MustUpdatePluginResource(t, pr)

	_, stopService = runNewTestService(t, ctx, fixture, withLogger(logger.With("phase", "test")))
	defer stopService()

	// EXPECT that the now-excluded Access Lists have been deleted and their
	// Principal State records have been destroyed, but that they still exist
	// as Groups in the downstream system
	requireEventually(t, accessListsMatch(ctx, fixture.Auth, "bravo", "delta"))
	requireEventually(t, accessListsHaveNoProvisioningState(ctx, fixture.Auth, "alpha", "charlie"))
	requireEventually(t, downstreamGroupsMatch(ctx, fixture.ICClient, allGroups...))

	// WHEN I explicitly delete an IC-sourced access list
	err := fixture.Auth.AccessLists.DeleteAccessList(ctx, "delta")
	require.NoError(t, err, "test requires deletion to succeed")

	// Expect that the downstream group is deleted actually deleted
	requireEventually(t, accessListsMatch(ctx, fixture.Auth, "bravo"))
	requireEventually(t, downstreamGroupsMatch(ctx, fixture.ICClient, "alpha", "bravo", "charlie"))
}

func collectGroupIDs(groups []*icsdk.Group) []string {
	var result []string
	for _, g := range groups {
		result = append(result, g.ID)
	}
	return result
}

func countICOriginatedList(in []listWithMembersAndRoles) int {
	count := 0
	for _, l := range in {
		if l.origin == common.OriginAWSIdentityCenter {
			count++
		}
	}
	return count
}

func compareAccessLists(t *testing.T, ctx context.Context, expected []listWithMembersAndRoles, service services.AccessLists) {
	t.Helper()
	accessLists, err := service.GetAccessLists(ctx)
	require.NoError(t, err)

	var actual []listWithMembersAndRoles
	for _, e := range accessLists {
		membersFromBackend := listMembers(t, ctx, e.GetName(), service)
		actual = append(actual, listWithMembersAndRoles{
			name:    e.GetName(),
			title:   e.Spec.Title,
			roles:   e.Spec.Grants.Roles,
			members: membersFromBackend,
			origin:  e.Origin(),
		})
	}
	require.ElementsMatch(t, expected, actual)
}

func listMembers(t *testing.T, ctx context.Context, accesListName string, service services.AccessLists) []string {
	t.Helper()
	var out []string

	var members []*accesslist.AccessListMember
	var pageToken string
	for {
		var err error
		var page []*accesslist.AccessListMember

		page, pageToken, err = service.ListAccessListMembers(ctx, accesListName, 0 /* use the default page size */, pageToken)
		require.NoError(t, err, "listMembersForACL: ListAccessListMembers")
		members = append(members, page...)
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
	slices.Sort(out)
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

func sdkGroups(t *testing.T, in []groupWithMemberAndAssignment) []*icsdk.Group {
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

func sdkGroupMembers(t *testing.T, in []groupWithMemberAndAssignment) map[string][]*icsdk.GroupMember {
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

func sdkGroupAssignments(t *testing.T, in []groupWithMemberAndAssignment) map[string][]*icsdk.Assignment {
	t.Helper()
	out := make(map[string][]*icsdk.Assignment)
	for _, g := range in {
		out[g.id] = g.assignments
	}

	return out
}

type icData struct {
	Accounts []*icsdk.Account
	Groups   []groupWithMemberAndAssignment
}

type groupWithMemberAndAssignment struct {
	name        string
	id          string
	members     []string
	assignments []*icsdk.Assignment
}

type listWithMembersAndRoles struct {
	name    string
	title   string
	members []string
	roles   []string
	origin  string
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

func TestMaybeImportGroupAndGroupMembersPropagatesError(t *testing.T) {
	ctx := context.Background()
	statusSink := &integration.FakeStatusSink{}
	mockedData := icsdk.NewMockedAWSState()
	fixture := icfixture.NewFixture(t,
		icfixture.WithAWSState(&mockedData),
		icfixture.WithStatusSink(statusSink))

	// test that failed import operation error is propagated.
	const errorMsg = "invalid credential"

	// maybeImportGroupAndGroupMembers eventually calls ListPermissionSets method to fetch permission sets.
	fixture.ICClient.MonkeyPatch.ListPermissionSets = func(context.Context) ([]*icsdk.PermissionSet, error) {
		return nil, trace.AccessDenied("%s", errorMsg)
	}

	svc := newTestService(t, fixture)

	err := svc.maybeImportGroupAndGroupMembers(ctx)
	require.Error(t, err)
	require.Eventually(t, func() bool {
		return statusSink.Get() != nil
	}, time.Second, time.Second/100)
	require.Equal(t, types.PluginStatusCode_OTHER_ERROR, statusSink.Get().GetCode())
	require.NotNil(t, statusSink.Get().GetAwsIc())
	require.Equal(t, types.AWSICGroupImportStatusCode_FAILED, statusSink.Get().GetAwsIc().GroupImportStatus.StatusCode)
	require.Contains(t, statusSink.Get().GetAwsIc().GroupImportStatus.ErrorMessage, errorMsg)

	// test that successful import operation and status emission is propagated.
	fixture.ICClient.MonkeyPatch.ListPermissionSets = nil /* Resetting a monkeypatch makes the sdkClient use a working ListPermissionSets mock */

	err = svc.maybeImportGroupAndGroupMembers(ctx)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return statusSink.Get() != nil
	}, time.Second, time.Second/100)
	require.Equal(t, types.PluginStatusCode_RUNNING, statusSink.Get().GetCode())
	require.NotNil(t, statusSink.Get().GetAwsIc())
	require.Equal(t, types.AWSICGroupImportStatusCode_DONE, statusSink.Get().GetAwsIc().GroupImportStatus.StatusCode)

	// test for successful import event but failed status event emission error is propagated.
	failSink := &FailingStatusSink{}
	svc = newTestService(t, fixture, withStatusSink(failSink))
	err = svc.maybeImportGroupAndGroupMembers(ctx)
	require.Error(t, err)
}

// FailingStatusSink fails to emit status.
type FailingStatusSink struct{}

func (s *FailingStatusSink) Emit(_ context.Context, _ types.PluginStatus) error {
	return trace.AccessDenied("failed")
}
