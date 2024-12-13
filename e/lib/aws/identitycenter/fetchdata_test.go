package identitycenter

import (
	"context"
	"maps"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
)

// TestAWSDataFetch asserts that the data fetched from AWS is converted into
// Teleport resources as expected
func TestAWSDataFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fixture := ictest.NewFixture(t)
	icSvc := newTestService(t, fixture)

	// Input data for this test is the default data set for the mocked Identity
	// Center client. See `sdk.NewMockedAWSState()` in `e/lib/aws/identitycenter/sdk/client_mock.go`
	data, err := icSvc.refreshExternalData(ctx)
	require.NoError(t, err)
	require.NotNil(t, data)

	// We need these PermissionSet records for multiple subtests, so define
	// them up here.
	expectedPermissionSets := psResourceMap{
		"permissionset_admin": test.PermissionSet{
			ID:          "permissionset_admin",
			Name:        "Admin",
			Description: "Admin permissions",
			ARN:         "arn:aws:sso:::permissionSet/Admin",
		}.Build(),

		"permissionset_readonly": test.PermissionSet{
			ID:          "permissionset_readonly",
			Name:        "ReadOnly",
			Description: "Read-only permissions",
			ARN:         "arn:aws:sso:::permissionSet/ReadOnly",
		}.Build(),
	}

	t.Run("InstanceInfo", func(t *testing.T) {
		expectedInstanceInfo := icsdk.InstanceInfo{
			Name:            "Mock Identity Center Instance",
			OwnerAccountID:  "2222222222",
			IdentityStoreID: "store1",
			Status:          ssoadmintypes.InstanceStatusActive,
		}
		require.Equal(t, &expectedInstanceInfo, data.icInstance)
	})

	t.Run("PermissionSets", func(t *testing.T) {
		require.Equal(t, expectedPermissionSets, data.permissionSets)
	})

	t.Run("Accounts", func(t *testing.T) {
		expectedAccounts := accountResourceMap{
			"1111111111": test.Account{
				ID:             "1111111111",
				Name:           "Account1",
				ARN:            "arn:aws:iam::1111111111:account/Account1",
				IsOwner:        false,
				PermissionSets: maps.Values(expectedPermissionSets),
				StartURL:       "https://store1.awsapps.com/start/#/console?account_id=1111111111",
			}.Build(),

			"2222222222": test.Account{
				ID:             "2222222222",
				Name:           "Account2",
				ARN:            "arn:aws:iam::2222222222:account/Account2",
				IsOwner:        false,
				PermissionSets: maps.Values(expectedPermissionSets),
				StartURL:       "https://store1.awsapps.com/start/#/console?account_id=2222222222",
			}.Build(),
		}
		require.Equal(t, expectedAccounts, data.accounts)
	})
}

func TestAWSDataFetchPropagatesClientFailure(t *testing.T) {
	fixture := ictest.NewFixture(t)
	icSvc := newTestService(t, fixture)

	// TODO: fill out other client operations
	testCases := []struct {
		name        string
		patchClient func(*testing.T, *icsdk.ClientMock)
	}{
		{
			name: "DescribeInstance",
			patchClient: func(subtestT *testing.T, client *icsdk.ClientMock) {
				client.MonkeyPatch.DescribeInstance =
					func(context.Context) (*icsdk.InstanceInfo, error) {
						return nil, trace.NotFound("no such endpoint")
					}
				subtestT.Cleanup(func() {
					client.MonkeyPatch.DescribeInstance = nil
				})
			},
		},
		{
			name: "ListPermissionSets",
			patchClient: func(subtestT *testing.T, client *icsdk.ClientMock) {
				client.MonkeyPatch.ListPermissionSets =
					func(context.Context) ([]*icsdk.PermissionSet, error) {
						return nil, trace.BadParameter("oops")
					}
				subtestT.Cleanup(func() {
					client.MonkeyPatch.ListPermissionSets = nil
				})
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.patchClient(t, fixture.ICClient)
			_, err := icSvc.refreshExternalData(ctx)
			require.Error(t, err)
		})
	}
}
