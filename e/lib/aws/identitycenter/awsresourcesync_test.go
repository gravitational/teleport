package identitycenter

import (
	"context"
	"maps"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/require"

	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/lib/services"
)

// TestPreprocessing asserts that data fetched from AWS is pre-processed into
// a Teleport-ready form as expected
func TestPreprocessing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fixture := ictest.NewFixture(t)
	icSvc := newTestService(t, fixture)

	const (
		acctOneID     = services.IdentityCenterAccountID("1111111111")
		acctTwoID     = services.IdentityCenterAccountID("2222222222")
		psReadOnlyARN = "arn:aws:sso:::permissionSet/ReadOnly"
		psAdminARN    = "arn:aws:sso:::permissionSet/Admin"
	)

	awsPermissionSets := psResourceMap{
		"permissionset_admin": test.PermissionSet{
			ID:          "permissionset_admin",
			Name:        "Admin",
			Description: "Admin permissions",
			ARN:         psAdminARN,
		}.Build(),

		"permissionset_readonly": test.PermissionSet{
			ID:          "permissionset_readonly",
			Name:        "ReadOnly",
			Description: "Read-only permissions",
			ARN:         psReadOnlyARN,
		}.Build(),
	}

	awsAccounts := accountResourceMap{
		acctOneID: test.Account{
			ID:             acctOneID,
			Name:           "Account1",
			ARN:            "arn:aws:iam::1111111111:account/Account1",
			IsOwner:        false,
			PermissionSets: maps.Values(awsPermissionSets),
		}.Build(),

		acctTwoID: test.Account{
			ID:             acctTwoID,
			Name:           "Account2",
			ARN:            "arn:aws:iam::2222222222:account/Account2",
			IsOwner:        false,
			PermissionSets: maps.Values(awsPermissionSets),
		}.Build(),
	}

	awsData := externalData{
		icInstance: &icsdk.InstanceInfo{
			Name:            "Mock Identity Center Instance",
			OwnerAccountID:  "2222222222",
			IdentityStoreID: "store1",
			Status:          ssoadmintypes.InstanceStatusActive,
		},
		permissionSets: awsPermissionSets,
		// the preprocessor will modify resources in-place, so we need to take a
		// copy or we will just end up comparing the modified resources against
		// themselves.
		accounts: awsAccounts.deepCopy(),
	}

	processedData, err := icSvc.preProcessExternalData(ctx, &awsData)
	require.NoError(t, err)
	require.NotNil(t, processedData)

	t.Run("InstanceInfo", func(t *testing.T) {
		require.Equal(t, awsData.icInstance, processedData.icInstance,
			"Instance info should be passed through unchanged")
	})

	t.Run("Accounts", func(t *testing.T) {
		require.Len(t, processedData.accounts, len(awsAccounts))

		// The only change to Account 1 should be the addition of assignment
		// names to the resources
		expectedAcct1 := awsAccounts[acctOneID].CloneResource().(services.IdentityCenterAccount)
		expectedAcct1.Spec.PermissionSetInfo[0].AssignmentId = "1111111111--admin"
		expectedAcct1.Spec.PermissionSetInfo[1].AssignmentId = "1111111111--readonly"
		require.Equal(t, expectedAcct1, processedData.accounts[acctOneID],
			"Account 1 should be passed through with only the assignment names added")

		// Account 2 should be identified as the IC instance owner, as well as
		// having the assignment names added to the permission set info
		expectedAcct2 := awsAccounts[acctTwoID].CloneResource().(services.IdentityCenterAccount)
		expectedAcct2.Spec.IsOrganizationOwner = true
		expectedAcct2.Spec.PermissionSetInfo[0].AssignmentId = "2222222222--admin"
		expectedAcct2.Spec.PermissionSetInfo[1].AssignmentId = "2222222222--readonly"
		require.Equal(t, expectedAcct2, processedData.accounts[acctTwoID],
			"Account 2 should be identified as the organization owner")
	})

	t.Run("PermissionSets", func(t *testing.T) {
		require.Equal(t, awsData.permissionSets, processedData.permissionSets,
			"PermissionSets should pass through unchanged")
	})

	t.Run("AccountAssignmentRoles", func(t *testing.T) {
		expectedRoles := accountAssignmentRolesMap{
			mkRoleKey(acctOneID, psAdminARN, string(acctOneID)): test.AccountAssignmentRole{
				Name:             "admin-on-account1-1111111111",
				AccountID:        acctOneID,
				PermissionSetARN: psAdminARN,
			}.Build(t),

			mkRoleKey(acctOneID, psReadOnlyARN, string(acctOneID)): test.AccountAssignmentRole{
				Name:             "readonly-on-account1-1111111111",
				AccountID:        acctOneID,
				PermissionSetARN: psReadOnlyARN,
			}.Build(t),

			mkRoleKey(acctTwoID, psAdminARN, string(acctTwoID)): test.AccountAssignmentRole{
				Name:             "admin-on-account2-2222222222",
				AccountID:        acctTwoID,
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
			}.Build(t),

			mkRoleKey(acctTwoID, psReadOnlyARN, string(acctTwoID)): test.AccountAssignmentRole{
				Name:             "readonly-on-account2-2222222222",
				AccountID:        acctTwoID,
				PermissionSetARN: psReadOnlyARN,
			}.Build(t),
		}
		require.Equal(t, expectedRoles, processedData.accountAssignmentRoles)
	})

	t.Run("AccountAssignments", func(t *testing.T) {
		expectedAccountAssignments := accountAssignmentMap{
			"1111111111--admin": test.AccountAssignment{
				ID:                "1111111111--admin",
				DisplayName:       `"Admin" on "Account1"`,
				AccountName:       "Account1",
				AccountID:         acctOneID,
				PermissionSetName: "Admin",
				PermissionSetARN:  psAdminARN,
			}.Build(),

			"1111111111--readonly": test.AccountAssignment{
				ID:                "1111111111--readonly",
				DisplayName:       `"ReadOnly" on "Account1"`,
				AccountName:       "Account1",
				AccountID:         acctOneID,
				PermissionSetName: "ReadOnly",
				PermissionSetARN:  psReadOnlyARN,
			}.Build(),

			"2222222222--admin": test.AccountAssignment{
				ID:                "2222222222--admin",
				DisplayName:       `"Admin" on "Account2"`,
				AccountName:       "Account2",
				AccountID:         acctTwoID,
				PermissionSetName: "Admin",
				PermissionSetARN:  psAdminARN,
			}.Build(),

			"2222222222--readonly": test.AccountAssignment{
				ID:                "2222222222--readonly",
				DisplayName:       `"ReadOnly" on "Account2"`,
				AccountName:       "Account2",
				AccountID:         acctTwoID,
				PermissionSetName: "ReadOnly",
				PermissionSetARN:  psReadOnlyARN,
			}.Build(),
		}
		require.Equal(t, expectedAccountAssignments, processedData.accountAssignments)
	})
}

func (m accountResourceMap) deepCopy() accountResourceMap {
	dst := make(accountResourceMap, len(m))
	for k, v := range m {
		dst[k] = v.CloneResource().(services.IdentityCenterAccount)
	}
	return dst
}
