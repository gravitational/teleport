package identitycenter

import (
	"context"
	"iter"
	"maps"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestAccountAssignmentRoleReconciliation(t *testing.T) {
	ctx := t.Context()

	fixture := ictest.NewFixture(t)
	icSvc := newTestService(t, fixture)

	testCases := []struct {
		name           string
		oldRoles       []*types.RoleV6
		newRoles       []*types.RoleV6
		expectErr      require.ErrorAssertionFunc
		expectResult   func(*testing.T, accountAssignmentRolesMap)
		expectRolesSvc func(*testing.T, RolesService)
	}{
		{
			name: "empty",
			// GIVEN empty rolesets
			// WHEN I reconcile the roles, EXPECT that the operation succeeds
			expectErr: require.NoError,
			// EXPECT that the in-memory result is empty
			expectResult: func(t *testing.T, roles accountAssignmentRolesMap) {
				require.Empty(t, roles)
			},
			// EXPECT that the roles service is still empty
			expectRolesSvc: func(t *testing.T, roles RolesService) {
				n := countRoles(t, iciter.AllAccountAssignmentRoles(ctx, roles))
				require.Equal(t, 0, n)
			},
		},
		{
			name: "created new",

			// GIVEN an (implicitly) empty "current" roleset, and a single new role
			newRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "new-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-arn",
				}.Build(t),
			},
			// WHEN I reconcile the roles, EXPECT that the operation succeeds
			expectErr: require.NoError,
			// EXPECT that the in-memory result map contains the new role,
			// indexed by the AWS Account and Permission Set that the role
			// grants
			expectResult: func(t *testing.T, roles accountAssignmentRolesMap) {
				require.Len(t, roles, 1)
				require.Contains(t, roles, mkRoleKey("1234567890", "some-ps-arn", "1234567890"))
			},
			// EXPECT that the new role has been saved to the cluster Role database
			expectRolesSvc: func(t *testing.T, roles RolesService) {
				_, err := roles.GetRole(ctx, "new-01")
				require.NoError(t, err)
			},
		},
		{
			name: "deprecated obsolete",

			// GIVEN a collection of existing roles
			oldRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "current-01",
					AccountID:        "1234567890",
					PermissionSetARN: "current-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "obsolete-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-arn",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "current-02",
					AccountID:        "1234567890",
					PermissionSetARN: "current-ps-02",
				}.Build(t),
			},
			// GIVEN a collection of "new" roles that is identical to the
			// existing role set, except that one "obsolete" role has been
			// removed.
			newRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "current-01",
					AccountID:        "1234567890",
					PermissionSetARN: "current-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "current-02",
					AccountID:        "1234567890",
					PermissionSetARN: "current-ps-02",
				}.Build(t),
			},
			// WHEN I reconcile the roles, EXPECT that the operation succeeds
			expectErr: require.NoError,
			// EXPECT that the in-memory result does not contain the obsolete
			// role. ALSO EXPECT that it still contains the "current" roles
			// in the "newRoles" list
			expectResult: func(t *testing.T, roles accountAssignmentRolesMap) {
				require.ElementsMatch(t,
					collectRoleNames(roles),
					[]string{"current-01", "current-02"})
			},
			// EXPECT that the cluster role service does not contain the obsolete
			// role. ALSO EXPECT that it still contains the "current" roles
			// in the "newRoles" list
			expectRolesSvc: func(t *testing.T, roles RolesService) {
				r, err := roles.GetRole(ctx, "obsolete-01")
				require.NoError(t, err, "Obsolete role must be deprecated not deleted")
				require.Empty(t, r.GetSubKind())

				// The obsolete role must not appear in the list of Account Assignment
				// roles when queried.
				accountAssignmentRoles, err := icSvc.loadAccountAssignmentRoles(ctx)
				require.NoError(t, err)
				require.ElementsMatch(t,
					collectRoleNames(accountAssignmentRoles),
					[]string{"current-01", "current-02"})
			},
		},
		{
			name: "updated role",
			// GIVEN a collection of existing roles
			oldRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "role-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "role-02",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-02",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "role-03",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-03",
				}.Build(t),
			},
			// GIVEN a collection of new roles that is the same as the old role
			// collection, apart from a modification to "role-02" labels
			newRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "role-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "role-02",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-02",
				}.Customize(
					func(r *types.RoleV6) {
						r.Metadata.Labels["updated"] = "true"
					}).Build(t),
				ictest.AccountAssignmentRole{
					Name:             "role-03",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-03",
				}.Build(t),
			},
			// WHEN I reconcile the roles, EXPECT that the operation succeeds
			expectErr: require.NoError,
			// EXPECT that the in-memory result reflects the updated role labels
			expectResult: func(t *testing.T, roles accountAssignmentRolesMap) {
				require.Len(t, roles, 3)
				r := roles[mkRoleKey("1234567890", "some-ps-02", "1234567890")]
				require.Equal(t, "true", r.Metadata.Labels["updated"])
			},
			// EXPECT that the cluster role service reflects the updated labels
			// for the target role
			expectRolesSvc: func(t *testing.T, roles RolesService) {
				r, err := roles.GetRole(ctx, "role-02")
				require.NoError(t, err)
				require.IsType(t, (*types.RoleV6)(nil), r)
				require.Equal(t, "true", r.(*types.RoleV6).Metadata.Labels["updated"])
			},
		},
		{
			name: "renamed role",

			// GIVEN an existing role collections
			oldRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "old-name-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "old-name-02",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-02",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "old-name-03",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-03",
				}.Build(t),
			},
			// GIVEN a collection of "new" roles, where one of the roles
			// references the same AWS Account and Permission Set as one of the
			// existing roles, but has a different name. (This can happen if an
			// AWS Admin renames an AWS account or PermissionSet)
			newRoles: []*types.RoleV6{
				ictest.AccountAssignmentRole{
					Name:             "old-name-01",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-01",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "new-name-02",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-02",
				}.Build(t),
				ictest.AccountAssignmentRole{
					Name:             "old-name-03",
					AccountID:        "1234567890",
					PermissionSetARN: "some-ps-03",
				}.Build(t),
			},
			// EXPECT that the operation succeeds
			expectErr: require.NoError,
			// EXPECT that the in-memory result contains the new role
			expectResult: func(t *testing.T, roles accountAssignmentRolesMap) {
				require.Len(t, roles, 3)

				r, ok := roles[mkRoleKey("1234567890", "some-ps-02", "1234567890")]
				require.True(t, ok, "Expected modified role to be in result set")
				require.Equal(t, "new-name-02", r.GetName())

				require.ElementsMatch(t,
					collectRoleNames(roles),
					[]string{"old-name-01", "new-name-02", "old-name-03"})
			},
			// EXPECT that old role has been deprecated in the cluster role
			// database and does not appear in the list of Account Assignment Roles
			// when loaded.
			expectRolesSvc: func(t *testing.T, roles RolesService) {
				// Old role must be deprecated
				oldRole, err := roles.GetRole(ctx, "old-name-02")
				require.NoError(t, err, "Obsolete role must be deprecated, not deleted")
				require.Empty(t, oldRole.GetSubKind())
				replacement, ok := oldRole.GetLabel("teleport.internal/replaced_with")
				require.True(t, ok, "Replacement label must be set")
				require.Equal(t, "new-name-02", replacement)

				// New role must exist
				_, err = roles.GetRole(ctx, "new-name-02")
				require.NoError(t, err, "New role expected to exist")

				// Old role must not appear in the list of Account Assignment
				// roles when queried.
				accountAssignmentRoles, err := icSvc.loadAccountAssignmentRoles(ctx)
				require.NoError(t, err)
				require.ElementsMatch(t,
					collectRoleNames(accountAssignmentRoles),
					[]string{"old-name-01", "new-name-02", "old-name-03"})
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(deleteAllRoles(t, fixture.Auth))

			// populate the test cluster roles service, and build up map that the
			// reconciler expects
			oldRoles := make(accountAssignmentRolesMap, len(test.oldRoles))
			for _, r := range test.oldRoles {
				r, err := fixture.Auth.CreateRole(ctx, r)
				require.NoError(t, err)

				require.IsType(t, (*types.RoleV6)(nil), r)
				rV6 := r.(*types.RoleV6)
				oldRoles[mustMakeRoleKey(t, rV6)] = rV6
			}

			// repackage the new roles into the map tha the reconciler expects
			newRoles := make(accountAssignmentRolesMap, len(test.newRoles))
			for _, r := range test.newRoles {
				newRoles[mustMakeRoleKey(t, r)] = r
			}

			// actually run the test
			updatedRoles, err := icSvc.reconcileAccountAssignmentRoles(ctx, oldRoles, newRoles)
			test.expectErr(t, err)
			test.expectResult(t, updatedRoles)
			test.expectRolesSvc(t, fixture.Auth.Services)
		})
	}
}

func countRoles(t *testing.T, s iter.Seq2[*types.RoleV6, error]) int {
	t.Helper()
	result := 0
	for _, err := range s {
		require.NoError(t, err)
		result++
	}
	return result
}

func deleteAllRoles(t *testing.T, rolesSvc RolesService) func() {
	getPage := func(ctx context.Context, pageSize int, pageToken string) ([]*types.RoleV6, string, error) {
		response, err := rolesSvc.ListRoles(ctx, &proto.ListRolesRequest{
			StartKey: pageToken,
			Limit:    int32(pageSize),
			Filter:   &types.RoleFilter{SkipSystemRoles: true},
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return response.Roles, response.NextKey, nil
	}

	return func() {
		ctx := t.Context()
		for r, err := range clientutils.Resources(ctx, getPage) {
			require.NoError(t, err)
			require.NoError(t, rolesSvc.DeleteRole(ctx, r.GetName()))
		}
	}
}

func mustMakeRoleKey(t *testing.T, role *types.RoleV6) accountAssignmentRoleKey {
	key, err := mkRoleKeyForRole(role)
	require.NoError(t, err)
	return key
}

func collectRoleNames(roles accountAssignmentRolesMap) []string {
	return sliceutils.Map(
		slices.Collect(maps.Values(roles)),
		types.GetName)
}
