package identitycenter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestAccessListPredicate(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})

	fixture := ictest.NewFixture(t)
	ctx := fixture.Ctx

	allowRole, err := fixture.Auth.CreateRole(ctx,
		ictest.AccountAssignmentRole{
			Name:             "with-allow-account-assignment",
			AccountID:        "some-account",
			PermissionSetARN: "some-ps-arn",
		}.Build(t))
	require.NoError(t, err)

	denyRole, err := fixture.Auth.CreateRole(ctx,
		ictest.AccountAssignmentRole{
			Name:             "with-deny-account-assignment",
			AccountID:        "some-account",
			PermissionSetARN: "some-ps-arn",
		}.Build(t))
	require.NoError(t, err)

	irrelevantRole, err := types.NewRole("without-account-assignment",
		types.RoleSpecV6{})
	require.NoError(t, err)
	irrelevantRole, err = fixture.Auth.CreateRole(ctx, irrelevantRole)
	require.NoError(t, err)

	adminUser, err := types.NewUser("admin")
	require.NoError(t, err)
	adminUser, err = fixture.Auth.CreateUser(fixture.Ctx, adminUser)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		ownerGrants  []types.Role
		memberGrants []types.Role
		expectation  require.BoolAssertionFunc
		labels       map[string]string
	}{

		{
			name:        "no assignments",
			expectation: require.False,
		},
		{
			name:         "single member assignment grant",
			memberGrants: []types.Role{allowRole},
			expectation:  require.True,
		},
		{
			name:         "multiple member assignments",
			memberGrants: []types.Role{irrelevantRole, allowRole},
			expectation:  require.True,
		},
		{
			name:         "irrelevant role assignments",
			memberGrants: []types.Role{irrelevantRole},
			expectation:  require.False,
		},
		{
			name:         "single member deny assignment",
			memberGrants: []types.Role{denyRole},
			expectation:  require.True,
		},
		{
			name:        "owner assignment is ignored",
			ownerGrants: []types.Role{allowRole, denyRole},
			expectation: require.False,
		},
		{
			name:        "acl originated from AWS Identity Center ",
			ownerGrants: []types.Role{},
			labels: map[string]string{
				common.OriginLabel: common.OriginAWSIdentityCenter,
			},
			expectation: require.True,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			acl, err := fixture.Auth.UpsertAccessList(ctx, ictest.AccessList{
				Name:          normalizeResourceName(t.Name()),
				Title:         t.Name(),
				Owners:        []types.User{adminUser},
				GrantsMembers: test.memberGrants,
				GrantsOwners:  test.ownerGrants,
				Labels:        test.labels,
			}.Build(t))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, fixture.Auth.DeleteAccessList(ctx, acl.GetName()))
			})

			predicate := makeAccessListAssignmentPredicate(fixture.Auth)

			include, err := predicate(ctx, acl)
			require.NoError(t, err)
			test.expectation(t, include)
		})
	}
}
