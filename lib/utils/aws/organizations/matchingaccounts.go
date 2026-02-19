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
	"log/slog"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationstypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
)

const (
	allOrganizationalUnits = "*"
)

// RequiredAPIs lists the AWS Organizations APIs required by MatchingAccounts.
// Must match the permissions required in the OrganizationsClient.
func RequiredAPIs() []string {
	return []string{
		"organizations:ListChildren",
		"organizations:ListRoots",
		"organizations:ListAccountsForParent",
	}
}

// OrganizationsClient is a minimal subset of AWS Organizations API used by MatchingAccounts.
type OrganizationsClient interface {
	organizations.ListChildrenAPIClient
	organizations.ListRootsAPIClient
	organizations.ListAccountsForParentAPIClient
}

type awsOrgItem struct {
	id                  string
	organizationalUnits []*awsOrgItem
	accounts            []string
	notActiveAccounts   []string
}

// MatchingAccountsFilter defines the filter to apply when retrieving matching accounts from an AWS Organization.
type MatchingAccountsFilter struct {
	// OrganizationID is the ID of the AWS Organization to query.
	// Required.
	OrganizationID string

	// Include is a list of AWS Organizational Unit IDs and children OUs to include.
	// Accounts that belong to these OUs, and their children, will be included.
	// Only exact matches or wildcard (*) are supported.
	// Required.
	IncludeOUs []string

	// Exclude is a list of AWS Organizational Unit IDs and children OUs to exclude.
	// Accounts that belong to these OUs, and their children, will be excluded, even if they were included.
	// Only exact matches are supported.
	// Optional. If empty, no OUs are excluded.
	ExcludeOUs []string
}

func (m *MatchingAccountsFilter) checkAndSetDefaults() error {
	if m.OrganizationID == "" {
		return trace.BadParameter("OrganizationID is required")
	}

	if len(m.IncludeOUs) == 0 {
		return trace.BadParameter("at least one Organizational Unit must be included ('*' can be used to include everything)")
	}

	if slices.Contains(m.ExcludeOUs, allOrganizationalUnits) {
		return trace.BadParameter("excluding all OUs is not supported")
	}

	return nil
}

// MatchingAccounts returns the list of account IDs that are part of the organization and match the filter.
// Every OU in ExcludeOUs is excluded from the results, including its children OUs.
func MatchingAccounts(ctx context.Context, log *slog.Logger, orgsClient OrganizationsClient, filter MatchingAccountsFilter) ([]string, error) {
	if err := filter.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	orgTree, err := buildOrgTree(ctx, orgsClient, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	includeRootAndChildrenAccounts := filter.IncludeOUs[0] == allOrganizationalUnits

	includedActiveAccounts, includedNotActiveAccounts := collectIncludedAccounts(orgTree, filter.IncludeOUs, includeRootAndChildrenAccounts)
	logNotActiveAccountsAreIgnored(ctx, log, filter.OrganizationID, includedNotActiveAccounts)

	return includedActiveAccounts, nil
}

func collectIncludedAccounts(orgItem *awsOrgItem, included []string, isParentIncluded bool) (activeAccounts []string, notActiveAccounts []string) {
	includeCurrentOU := isParentIncluded || slices.Contains(included, orgItem.id)

	if includeCurrentOU {
		activeAccounts = append(activeAccounts, orgItem.accounts...)
		notActiveAccounts = append(notActiveAccounts, orgItem.notActiveAccounts...)
	}

	for _, orgUnit := range orgItem.organizationalUnits {
		childAccountIDs, childnotActiveAccounts := collectIncludedAccounts(orgUnit, included, includeCurrentOU)

		activeAccounts = append(activeAccounts, childAccountIDs...)
		notActiveAccounts = append(notActiveAccounts, childnotActiveAccounts...)
	}

	return activeAccounts, notActiveAccounts
}

func logNotActiveAccountsAreIgnored(ctx context.Context, log *slog.Logger, organizationID string, notActiveAccountIDs []string) {
	if len(notActiveAccountIDs) == 0 {
		return
	}

	// Log only the first 10 non-active accounts to avoid log flooding.
	if len(notActiveAccountIDs) > 10 {
		notActiveAccountIDs = notActiveAccountIDs[:10]
	}
	notActiveAccounts := strings.Join(notActiveAccountIDs, ", ")

	log.DebugContext(ctx, "non-active accounts under organization were ignored",
		"organization_id", organizationID,
		"total_not_active", len(notActiveAccountIDs),
		"not_active_accounts", notActiveAccounts,
	)
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

	activeAccountIDs, notActiveAccountIDs, err := accountsInOrganizationalUnit(ctx, orgsClient, ouID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ret.accounts = activeAccountIDs
	ret.notActiveAccounts = notActiveAccountIDs

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

func accountsInOrganizationalUnit(ctx context.Context, orgChildrenLister organizations.ListAccountsForParentAPIClient, ouID string) (activeAccountIDs []string, notActiveAccountIDs []string, err error) {
	paginator := organizations.NewListAccountsForParentPaginator(orgChildrenLister, &organizations.ListAccountsForParentInput{
		ParentId: aws.String(ouID),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, account := range page.Accounts {
			if account.State == organizationstypes.AccountStateActive {
				activeAccountIDs = append(activeAccountIDs, aws.ToString(account.Id))
			} else {
				notActiveAccountIDs = append(notActiveAccountIDs, aws.ToString(account.Id))
			}
		}
	}

	return activeAccountIDs, notActiveAccountIDs, nil
}
