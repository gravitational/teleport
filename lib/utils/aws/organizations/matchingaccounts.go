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
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationstypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
)

const (
	allOrganizationalUnits = "*"
)

// RequiredAPIs lists the AWS Organizations APIs required by MatchingAccounts.
func RequiredAPIs() []string {
	return []string{
		"organizations:ListChildren",
		"organizations:ListRoots",
		"organizations:ListAccounts",
		"organizations:ListAccountsForParent",
	}
}

// OrganizationsClient is a minimal subset of AWS Organizations API used by MatchingAccounts.
type OrganizationsClient interface {
	organizations.ListChildrenAPIClient
	organizations.ListRootsAPIClient
	organizations.ListAccountsAPIClient
	organizations.ListAccountsForParentAPIClient
}

type awsOrgItem struct {
	id                  string
	organizationalUnits []*awsOrgItem
	accounts            []string
}

// MatchingAccountsFilter defines the filter to apply when retrieving matching accounts from an AWS Organization.
type MatchingAccountsFilter struct {
	// OrganizationID is the ID of the AWS Organization to query.
	// Required.
	OrganizationID string

	// IncludeOUs is the list of Organizational Unit IDs to include.
	// If empty or contains "*", all OUs are included (except those in ExcludeOUs).
	// If contains specific OU IDs, only those OUs and their children OUs are included.
	// Optional.
	IncludeOUs []string

	// ExcludeOUs is the list of Organizational Unit IDs to exclude.
	// If contains "*", all OUs are excluded.
	// Optional.
	ExcludeOUs []string
}

func (m *MatchingAccountsFilter) checkAndSetDefaults() error {
	if m.OrganizationID == "" {
		return trace.BadParameter("OrganizationID is required")
	}

	if len(m.IncludeOUs) == 0 {
		return trace.BadParameter("at least one Organizational Unit must be included")
	}

	if len(m.IncludeOUs) > 1 && slices.Contains(m.IncludeOUs, allOrganizationalUnits) {
		return trace.BadParameter("IncludeOUs cannot contain '*' along with other OU IDs")
	}

	if len(m.ExcludeOUs) > 1 && slices.Contains(m.ExcludeOUs, allOrganizationalUnits) {
		return trace.BadParameter("ExcludeOUs cannot contain '*' along with other OU IDs")
	}

	return nil
}

func (m *MatchingAccountsFilter) isIncludeNothing() bool {
	return len(m.ExcludeOUs) == 1 && m.ExcludeOUs[0] == allOrganizationalUnits
}

func (m *MatchingAccountsFilter) isIncludeAll() bool {
	return len(m.ExcludeOUs) == 0 && (len(m.IncludeOUs) == 0 || (len(m.IncludeOUs) == 1 && m.IncludeOUs[0] == allOrganizationalUnits))
}

// MatchingAccounts returns the list of account IDs that are part of the organization and match the filter.
// Every OU in ExcludeOUs is excluded from the results, including its children OUs.
func MatchingAccounts(ctx context.Context, orgsClient OrganizationsClient, filter MatchingAccountsFilter) ([]string, error) {
	if err := filter.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if filter.isIncludeNothing() {
		return []string{}, nil
	}

	if filter.isIncludeAll() {
		accountIDs, err := allAccounts(ctx, orgsClient, filter.OrganizationID)
		return accountIDs, trace.Wrap(err)
	}

	// General case: build the organization tree, this tree already removes all the excluded OUs and their children OUs.
	orgTree, err := buildOrgTree(ctx, orgsClient, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Then: iterate over the tree and collect the included accounts.
	includeEverything := len(filter.IncludeOUs) == 0 || filter.IncludeOUs[0] == allOrganizationalUnits
	return collectIncludedAccounts(orgTree, filter.IncludeOUs, includeEverything), nil
}

func collectIncludedAccounts(orgItem *awsOrgItem, included []string, isParentIncluded bool) []string {
	includeCurrentOU := isParentIncluded || slices.Contains(included, orgItem.id)

	var accountIDs []string
	if includeCurrentOU {
		accountIDs = append(accountIDs, orgItem.accounts...)
	}

	for _, orgUnit := range orgItem.organizationalUnits {
		childAccountIDs := collectIncludedAccounts(orgUnit, included, includeCurrentOU)
		accountIDs = append(accountIDs, childAccountIDs...)
	}

	return accountIDs
}

func allAccounts(ctx context.Context, orgsClient OrganizationsClient, organizationID string) ([]string, error) {
	var accountIDs []string
	organizationIDValidated := false

	paginator := organizations.NewListAccountsPaginator(orgsClient, &organizations.ListAccountsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, account := range page.Accounts {

			// This check ensures the assumed role belongs to the expected Organization.
			// Only need to validate once because all accounts will belong to the same Organization.
			if !organizationIDValidated {
				accountsOrganization, err := organizationIDFromAccountARN(aws.ToString(account.Arn))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				if accountsOrganization != organizationID {
					return nil, trace.BadParameter("the AWS Organizations client is not part of the expected Organization %s", organizationID)
				}
				organizationIDValidated = true
			}

			if account.State == organizationstypes.AccountStateActive {
				accountIDs = append(accountIDs, aws.ToString(account.Id))
			}
		}
	}

	return accountIDs, nil
}

// Limits of AWS Organizations:
// https://docs.aws.amazon.com/organizations/latest/userguide/orgs_reference_limits.html
// Most relevant limits:
// Max OU depth is 5 levels.
// At most there will be 2000 OUs in an AWS Organization.
// At most there will be 10 accounts, but that's configurable.
func buildOrgTree(ctx context.Context, orgsClient OrganizationsClient, filter MatchingAccountsFilter) (*awsOrgItem, error) {
	paginator := organizations.NewListRootsPaginator(orgsClient, &organizations.ListRootsInput{})
	var roots []organizationstypes.Root
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roots = append(roots, page.Roots...)
	}

	// AWS Docs state that:
	// > You can have only one root. AWS Organizations automatically creates the root for you when you create an organization.
	if len(roots) != 1 {
		return nil, trace.BadParameter("expected exactly one root organizational unit, got %d", len(roots))
	}
	root := roots[0]

	rootsOrganization, err := organizationIDFromRootOUARN(aws.ToString(root.Arn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rootsOrganization != filter.OrganizationID {
		return nil, trace.BadParameter("the AWS Organizations client is not part of the expected Organization %s", filter.OrganizationID)
	}

	rootOU, err := organizationalUnitDetails(ctx, orgsClient, aws.ToString(root.Id), filter.ExcludeOUs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rootOU, nil
}

func organizationalUnitDetails(ctx context.Context, orgsClient OrganizationsClient, ouID string, excludedOUs []string) (*awsOrgItem, error) {
	ret := &awsOrgItem{
		id: ouID,
	}

	// Everything under an excluded OU is not considered.
	if slices.Contains(excludedOUs, ouID) {
		return ret, nil
	}

	accountIDs, err := accountsInOrganizationalUnit(ctx, orgsClient, ouID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ret.accounts = accountIDs

	childrenOUIDs, err := childrenOUs(ctx, orgsClient, ouID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, childOUID := range childrenOUIDs {
		childOU, err := organizationalUnitDetails(ctx, orgsClient, childOUID, excludedOUs)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.organizationalUnits = append(ret.organizationalUnits, childOU)
	}

	return ret, nil
}

func childrenOUs(ctx context.Context, orgChildrenLister organizations.ListChildrenAPIClient, ouID string) ([]string, error) {
	var childOUs []string

	paginator := organizations.NewListChildrenPaginator(orgChildrenLister, &organizations.ListChildrenInput{
		ParentId:  aws.String(ouID),
		ChildType: organizationstypes.ChildTypeOrganizationalUnit,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, child := range page.Children {
			childOUs = append(childOUs, aws.ToString(child.Id))
		}
	}

	return childOUs, nil
}

func accountsInOrganizationalUnit(ctx context.Context, orgChildrenLister organizations.ListAccountsForParentAPIClient, ouID string) ([]string, error) {
	var accountIDs []string

	paginator := organizations.NewListAccountsForParentPaginator(orgChildrenLister, &organizations.ListAccountsForParentInput{
		ParentId: aws.String(ouID),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, account := range page.Accounts {
			if account.State == organizationstypes.AccountStateActive {
				accountIDs = append(accountIDs, aws.ToString(account.Id))
			}
		}
	}

	return accountIDs, nil
}
