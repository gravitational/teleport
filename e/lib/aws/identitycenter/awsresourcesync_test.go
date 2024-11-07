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
	awsData := externalData{
		icInstance: &icsdk.InstanceInfo{
			Name:            "Mock Identity Center Instance",
			OwnerAccountID:  "2222222222",
			IdentityStoreID: "store1",
			Status:          ssoadmintypes.InstanceStatusActive,
		},
		permissionSets: awsPermissionSets,
		accounts: accountResourceMap{
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
		},
	}

	processedData, err := icSvc.preProcessExternalData(ctx, &awsData)
	require.NoError(t, err)
	require.NotNil(t, processedData)

	t.Run("InstanceInfo", func(t *testing.T) {
		require.Equal(t, awsData.icInstance, processedData.icInstance,
			"Instance info should be passed through unchanged")
	})

	t.Run("Accounts", func(t *testing.T) {
		// Accounts should have been passed through unchanged, except that
		// Account 2 should have been identified as the organization owner
		require.Len(t, processedData.accounts, len(awsData.accounts))
		require.Equal(t, awsData.accounts[acctOneID], processedData.accounts[acctOneID],
			"Account 1 should be passed through unchanged")

		expectedAcct2 := test.Account{
			ID:             acctTwoID,
			Name:           "Account2",
			ARN:            "arn:aws:iam::2222222222:account/Account2",
			IsOwner:        true,
			PermissionSets: maps.Values(awsPermissionSets),
		}.Build()
		require.Equal(t, expectedAcct2, processedData.accounts[acctTwoID],
			"Account 2 should be identified as the organization owner")
	})

	t.Run("PermissionSets", func(t *testing.T) {
		require.Equal(t, awsData.permissionSets, processedData.permissionSets,
			"PermissionSets should pass through unchanged")
	})

	t.Run("AccountAssignmentRoles", func(t *testing.T) {
		expectedRoles := accountAssignmentRolesMap{
			mkRoleKey(acctOneID, psAdminARN): test.AccountAssignmentRole{
				Name:             "admin-on-account1",
				AccountID:        acctOneID,
				PermissionSetARN: psAdminARN,
			}.Build(t),

			mkRoleKey(acctOneID, psReadOnlyARN): test.AccountAssignmentRole{
				Name:             "readonly-on-account1",
				AccountID:        acctOneID,
				PermissionSetARN: psReadOnlyARN,
			}.Build(t),

			mkRoleKey(acctTwoID, psAdminARN): test.AccountAssignmentRole{
				Name:             "admin-on-account2",
				AccountID:        acctTwoID,
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
			}.Build(t),

			mkRoleKey(acctTwoID, psReadOnlyARN): test.AccountAssignmentRole{
				Name:             "readonly-on-account2",
				AccountID:        acctTwoID,
				PermissionSetARN: psReadOnlyARN,
			}.Build(t),
		}
		require.Equal(t, expectedRoles, processedData.accountAssignmentRoles)
	})

	t.Run("AccountAssignments", func(t *testing.T) {
		expectedAccountAssignments := accountAssignmentMap{
			"account1--admin": test.AccountAssignment{
				ID:                "account1--admin",
				DisplayName:       "Admin on Account1",
				AccountName:       "Account1",
				AccountID:         acctOneID,
				PermissionSetName: "Admin",
				PermissionSetARN:  psAdminARN,
			}.Build(),

			"account1--readonly": test.AccountAssignment{
				ID:                "account1--readonly",
				DisplayName:       "ReadOnly on Account1",
				AccountName:       "Account1",
				AccountID:         acctOneID,
				PermissionSetName: "ReadOnly",
				PermissionSetARN:  psReadOnlyARN,
			}.Build(),

			"account2--admin": test.AccountAssignment{
				ID:                "account2--admin",
				DisplayName:       "Admin on Account2",
				AccountName:       "Account2",
				AccountID:         acctTwoID,
				PermissionSetName: "Admin",
				PermissionSetARN:  psAdminARN,
			}.Build(),

			"account2--readonly": test.AccountAssignment{
				ID:                "account2--readonly",
				DisplayName:       "ReadOnly on Account2",
				AccountName:       "Account2",
				AccountID:         acctTwoID,
				PermissionSetName: "ReadOnly",
				PermissionSetARN:  psReadOnlyARN,
			}.Build(),
		}
		require.Equal(t, expectedAccountAssignments, processedData.accountAssignments)
	})
}
