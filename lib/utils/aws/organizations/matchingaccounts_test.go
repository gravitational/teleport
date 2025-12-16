/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package organizations

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationstypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type mockOrganizationsClient struct {
	organizationID string
	rootOUID       string
	ouItems        map[string]ouItem
}

type ouItem struct {
	innerOUs               []string
	innerAccounts          []string
	innerNotActiveAccounts []string
}

func (m *mockOrganizationsClient) ListChildren(ctx context.Context, input *organizations.ListChildrenInput, opts ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error) {
	if input.ChildType != organizationstypes.ChildTypeOrganizationalUnit {
		return nil, trace.NotImplemented("unexpected call to organizations.ListChildren, with ChildType != OU")
	}

	ouItem, ok := m.ouItems[*input.ParentId]
	if !ok {
		return nil, trace.NotFound("OU %s does not exist", *input.ParentId)
	}

	var children []organizationstypes.Child
	for _, ouID := range ouItem.innerOUs {
		children = append(children, organizationstypes.Child{
			Id:   aws.String(ouID),
			Type: organizationstypes.ChildTypeOrganizationalUnit,
		})
	}
	return &organizations.ListChildrenOutput{
		Children: children,
	}, nil
}

func (m *mockOrganizationsClient) ListRoots(ctx context.Context, input *organizations.ListRootsInput, opts ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
	rootARN := fmt.Sprintf("arn:aws:organizations::0000000000:root/%s/%s", m.organizationID, m.rootOUID)
	return &organizations.ListRootsOutput{
		Roots: []organizationstypes.Root{
			{
				Id:  aws.String(m.rootOUID),
				Arn: aws.String(rootARN),
			},
		},
	}, nil
}

func (m *mockOrganizationsClient) ListAccountsForParent(ctx context.Context, input *organizations.ListAccountsForParentInput, opts ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
	ouItem, ok := m.ouItems[*input.ParentId]
	if !ok {
		return nil, trace.NotFound("OU %s does not exist", *input.ParentId)
	}

	var accounts []organizationstypes.Account
	for _, accountID := range ouItem.innerAccounts {
		accountARN := fmt.Sprintf("arn:aws:organizations::0000000000:account/%s/%s", m.organizationID, accountID)
		accounts = append(accounts, organizationstypes.Account{
			Id:    aws.String(accountID),
			State: organizationstypes.AccountStateActive,
			Arn:   aws.String(accountARN),
		})
	}
	for _, accountID := range ouItem.innerNotActiveAccounts {
		accountARN := fmt.Sprintf("arn:aws:organizations::0000000000:account/%s/%s", m.organizationID, accountID)
		accounts = append(accounts, organizationstypes.Account{
			Id:    aws.String(accountID),
			State: organizationstypes.AccountStateSuspended,
			Arn:   aws.String(accountARN),
		})
	}
	return &organizations.ListAccountsForParentOutput{
		Accounts: accounts,
	}, nil
}

func TestMatchingAccounts(t *testing.T) {
	for _, tt := range []struct {
		name             string
		filter           MatchingAccountsFilter
		orgsClient       *mockOrganizationsClient
		errCheck         require.ErrorAssertionFunc
		expectedAccounts []string
	}{
		{
			name: "only root OU, include all filter: returns all accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-r1-01", "o1-r1-02"},
		},
		{
			name: "exclude everything returns an error",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				ExcludeOUs:     []string{"*"},
				IncludeOUs:     []string{"r-1"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
					},
				},
			},
			errCheck:         require.Error,
			expectedAccounts: []string{},
		},
		{
			name: "missing organization id returns an error",
			filter: MatchingAccountsFilter{
				ExcludeOUs: []string{"*"},
				IncludeOUs: []string{"r-1"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
					},
				},
			},
			errCheck:         require.Error,
			expectedAccounts: []string{},
		},
		{
			name: "non-active accounts are discarded",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerNotActiveAccounts: []string{
							"o1-r1-01-suspended",
							"o1-r1-02-suspended",
							"o1-r1-03-suspended",
							"o1-r1-04-suspended",
							"o1-r1-05-suspended",
							"o1-r1-06-suspended",
							"o1-r1-07-suspended",
							"o1-r1-08-suspended",
							"o1-r1-09-suspended",
							"o1-r1-10-suspended",
							"o1-r1-11-suspended",
							"o1-r1-12-suspended",
						},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-r1-01", "o1-r1-02"},
		},
		{
			name: "only root OU, no filter, invalid org: returns error",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-2",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o2-r1-01", "o2-r1-02"},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "one excluded, but wrong organization id: returns error",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				ExcludeOUs:     []string{"r-1"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-2",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o2-r1-01", "o2-r1-02"},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "multiple OUs, no filter: returns all accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerAccounts: []string{"o1-ou10-01"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-r1-01", "o1-r1-02", "o1-ou10-01", "o1-ou20-01"},
		},
		{
			name: "multiple OUs with empty OUs, no filter: returns all accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-r1-01", "o1-r1-02", "o1-ou11-01", "o1-ou11-02", "o1-ou20-01"},
		},
		{
			name: "filter excludes all OUs explicitly: returns no accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
				ExcludeOUs:     []string{"r-1", "ou-11", "ou-20"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{},
		},
		{
			name: "filter excludes all OUs explicitly, except the root: returns only the root accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"*"},
				ExcludeOUs:     []string{"ou-10", "ou-20"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-r1-01", "o1-r1-02"},
		},
		{
			name: "filter only includes one OU: returns only the OU and its children accounts",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"ou-10"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
		},
		{
			name: "excludes one OU, which is under an included OU",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"ou-10"},
				ExcludeOUs:     []string{"ou-11"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10", "ou-20"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-20": {
						innerAccounts: []string{"o1-ou20-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{},
		},
		{
			name: "5 levels of OUs (max according to AWS docs) works correctly",
			filter: MatchingAccountsFilter{
				OrganizationID: "o-1",
				IncludeOUs:     []string{"ou-10"},
				ExcludeOUs:     []string{"ou-11"},
			},
			orgsClient: &mockOrganizationsClient{
				organizationID: "o-1",
				rootOUID:       "r-1",
				ouItems: map[string]ouItem{
					"r-1": {
						innerAccounts: []string{"o1-r1-01", "o1-r1-02"},
						innerOUs:      []string{"ou-10"},
					},
					"ou-10": {
						innerOUs: []string{"ou-11", "ou-12"},
					},
					"ou-11": {
						innerAccounts: []string{"o1-ou11-01", "o1-ou11-02"},
					},
					"ou-12": {
						innerOUs: []string{"ou-120"},
					},
					"ou-120": {
						innerOUs: []string{"ou-1200"},
					},
					"ou-1200": {
						innerOUs: []string{"ou-12000"},
					},
					"ou-12000": {
						innerOUs: []string{"ou-120000"},
					},
					"ou-120000": {
						innerOUs: []string{"ou-1200000"},
					},
					"ou-1200000": {
						innerAccounts: []string{"o1-ou1200000-01"},
					},
				},
			},
			errCheck:         require.NoError,
			expectedAccounts: []string{"o1-ou1200000-01"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			matchingAccounts, err := MatchingAccounts(
				t.Context(),
				logtest.NewLogger(),
				tt.orgsClient,
				tt.filter,
			)
			tt.errCheck(t, err)
			if tt.expectedAccounts != nil {
				require.ElementsMatch(t, tt.expectedAccounts, matchingAccounts)
			}
		})
	}
}
