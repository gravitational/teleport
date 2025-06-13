package identitycenter

import (
	"context"
	"maps"
	"slices"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
)

// TestAWSDataFetch asserts that the data fetched from AWS is converted into
// Teleport resources as expected
func TestAWSDataFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fixture := test.NewFixture(t)
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

func TestFetchAccountFilters(t *testing.T) {
	testCases := []struct {
		name             string
		awsAccounts      []*icsdk.Account
		filters          icfilters.Filters
		expectedAccounts []*identitycenterv1.Account
	}{
		{
			name: "unfiltered",
			awsAccounts: []*icsdk.Account{
				{Name: "alpha", ID: "1234567890", ARN: "arn:aws:iam:::account/alpha"},
				{Name: "bravo", ID: "0987654321", ARN: "arn:aws:iam:::account/bravo"},
			},
			expectedAccounts: []*identitycenterv1.Account{
				test.Account{Name: "alpha", ID: "1234567890", ARN: "arn:aws:iam:::account/alpha"}.Build(),
				test.Account{Name: "bravo", ID: "0987654321", ARN: "arn:aws:iam:::account/bravo"}.Build(),
			},
		},
		{
			name: "filtered by ID",
			awsAccounts: []*icsdk.Account{
				{Name: "alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"},
				{Name: "bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"},
				{Name: "charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"},
			},
			filters: icfilters.Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "2222222222"}},
			},
			expectedAccounts: []*identitycenterv1.Account{
				test.Account{Name: "bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"}.Build(),
			},
		},
		{
			name: "filtered by regex",
			awsAccounts: []*icsdk.Account{
				{Name: "include-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"},
				{Name: "exclude-bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"},
				{Name: "include-charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"},
			},
			filters: icfilters.Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "^include-.*$"}},
			},
			expectedAccounts: []*identitycenterv1.Account{
				test.Account{Name: "include-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"}.Build(),
				test.Account{Name: "include-charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"}.Build(),
			},
		},
		{
			name: "filtered by glob",
			awsAccounts: []*icsdk.Account{
				{Name: "include-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"},
				{Name: "exclude-bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"},
				{Name: "include-charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"},
			},
			filters: icfilters.Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "include-*"}},
			},
			expectedAccounts: []*identitycenterv1.Account{
				test.Account{Name: "include-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"}.Build(),
				test.Account{Name: "include-charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"}.Build(),
			},
		},
		{
			name: "multiple filters",
			awsAccounts: []*icsdk.Account{
				{Name: "name-match-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"},
				{Name: "id-match-bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"},
				{Name: "exclude-charlie", ID: "3333333333", ARN: "arn:aws:iam:::account/charlie"},
				{Name: "name-match-delta", ID: "4444444444", ARN: "arn:aws:iam:::account/delta"},
			},
			filters: icfilters.Filters{
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_Id{Id: "2222222222"}},
				&types.AWSICResourceFilter{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "^name-match-.*$"}},
			},
			expectedAccounts: []*identitycenterv1.Account{
				test.Account{Name: "name-match-alpha", ID: "1111111111", ARN: "arn:aws:iam:::account/alpha"}.Build(),
				test.Account{Name: "id-match-bravo", ID: "2222222222", ARN: "arn:aws:iam:::account/bravo"}.Build(),
				test.Account{Name: "name-match-delta", ID: "4444444444", ARN: "arn:aws:iam:::account/delta"}.Build(),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fixture := test.NewFixture(t)
	icSvc := newTestService(t, fixture)
	mockIC := fixture.ICClient

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// GIVEN a mock AWS client configured with a known collection of AWS
			// accounts
			mockIC.Accounts = test.awsAccounts

			// GIVEN an Identity Center service configured with a set of account
			// filters
			icSvc.importConfig.AccountFilters = test.filters

			// WHEN I fetch the accounts from AWS
			awsdata, err := icSvc.fetchAccounts(ctx, mockIC.Info.IdentityStoreID)
			require.NoError(t, err)

			// EXPECT that only accounts matching the supplied filter set are returned.
			actualAccounts := slices.Collect(maps.Values(awsdata))
			require.ElementsMatch(t, test.expectedAccounts, actualAccounts)
		})
	}
}

func TestAWSDataFetchPropagatesClientFailure(t *testing.T) {
	fixture := test.NewFixture(t)
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
