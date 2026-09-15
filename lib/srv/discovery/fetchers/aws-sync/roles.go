/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/durationpb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// pollAWSRoles is a function that returns a function that fetches
// AWS roles and their inline and attached policies. The function handles
// errors by keeping the last polled result (from a.lastResult) rather
// than omitting roles or policies from the result, because omitting
// them would make it appear that the roles or policies have been deleted.
// The errors are still surfaced via the collectErr function.
//
// A role that is deleted from AWS while polling is not a failure. It is a
// change in state, and is reflected by dropping the role from the result.
func (a *Fetcher) pollAWSRoles(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		existing := a.lastResult
		result.Roles, err = a.fetchRoles(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch roles"))
			result.Roles = existing.Roles
			result.RoleAttachedPolicies = existing.RoleAttachedPolicies
			result.RoleInlinePolicies = existing.RoleInlinePolicies
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		// Limit the number of concurrent goroutines to avoid overwhelming the AWS API.
		// These goroutines are fetching inline and attached policies for each group.
		// We also have other goroutines fetching inline and attached policies for users
		// and roles.
		eG.SetLimit(5)
		roleMu := sync.Mutex{}
		for i, role := range result.Roles {
			eG.Go(func() error {
				roleInlinePolicies, err := a.fetchRoleInlinePolicies(ctx, role)
				if err != nil {
					var noSuchEntityErr *iamtypes.NoSuchEntityException
					if errors.As(err, &noSuchEntityErr) {
						// The role was removed while we are polling. Remove this role
						// from the list of discovered roles.
						result.Roles[i] = nil
						return nil
					}
					// On other errors retrieving the inline policies, use the policies from the last poll
					// (existing) so it does not appear that the inline policies have been deleted. Extract
					// just the inline policies for this role from the last poll. roleInlinePolicies is a
					// slice of all the inline policies for the role, so extract all of them from the last poll.
					roleInlinePolicies = sliceFilter(existing.RoleInlinePolicies, func(inline *accessgraphv1alpha.AWSRoleInlinePolicyV1) bool {
						return inline.GetAwsRole().GetName() == role.GetName() && inline.GetAwsRole().GetAccountId() == role.GetAccountId()
					})
					collectErr(trace.Wrap(err, "failed to fetch role %q inline policies", role.GetName()))
				}

				roleAttachedPolicies, err := a.fetchRoleAttachedPolicies(ctx, role)
				if err != nil {
					var noSuchEntityErr *iamtypes.NoSuchEntityException
					if errors.As(err, &noSuchEntityErr) {
						// The role was removed while we are polling. Remove this role
						// from the list of discovered roles.
						result.Roles[i] = nil
						return nil
					}
					// On other errors retrieving the attached policies, use the policies from the last poll
					// (existing) so it does not appear that the attached policies have been deleted. Extract
					// just the attached policies for this role from the last poll. roleAttachedPolicies is a
					// singular value that contains multiple attached policies, so extract just the first from
					// the last poll - there should be only one for each role.
					roleAttachedPolicies = sliceFilterPickFirst(existing.RoleAttachedPolicies, func(attached *accessgraphv1alpha.AWSRoleAttachedPolicies) bool {
						return attached.GetAwsRole().GetName() == role.GetName() && attached.GetAwsRole().GetAccountId() == role.GetAccountId()
					})
					collectErr(trace.Wrap(err, "failed to fetch role %q attached policies", role.GetName()))
				}

				roleMu.Lock()
				result.RoleInlinePolicies = append(result.RoleInlinePolicies, roleInlinePolicies...)
				if roleAttachedPolicies != nil {
					result.RoleAttachedPolicies = append(result.RoleAttachedPolicies, roleAttachedPolicies)
				}
				roleMu.Unlock()
				return nil
			})
		}
		// always discard the error
		_ = eG.Wait()
		// Remove any nils from roles being deleted in AWS while iterating
		result.Roles = slices.DeleteFunc(result.Roles, isNilPtr)
		return nil
	}
}

// fetchRoles fetches AWS roles and returns them as a slice of accessgraphv1alpha.AWSRoleV1.
func (a *Fetcher) fetchRoles(ctx context.Context) ([]*accessgraphv1alpha.AWSRoleV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because roles are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListRolesPaginator(
		iamClient,
		&iam.ListRolesInput{
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListRolesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var roles []*accessgraphv1alpha.AWSRoleV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return roles, trace.Wrap(err)
		}
		for _, role := range page.Roles {
			roles = append(roles, awsRoleToProtoRole(role, a.AccountID))
		}
	}
	return roles, trace.Wrap(err)
}

// fetchRoleInlinePolicies fetches inline policies for an AWS role and returns
// them as a slice of accessgraphv1alpha.AWSRoleInlinePolicyV1.
// It uses iam.ListRolePoliciesPagesWithContext to iterate over all inline policies
// and iam.GetRolePolicyWithContext to fetch policy documents.
func (a *Fetcher) fetchRoleInlinePolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) ([]*accessgraphv1alpha.AWSRoleInlinePolicyV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListRolePoliciesPaginator(
		iamClient,
		&iam.ListRolePoliciesInput{
			RoleName: aws.String(role.GetName()),
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListRolePoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var policies []*accessgraphv1alpha.AWSRoleInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			// This may return a NoSuchEntityException error, which can refer only to
			// the role, not any inline policies. That is handled by the caller by
			// removing the role from the polling results.
			return policies, trace.NewAggregate(append(errs, err)...)
		}
		for _, policyName := range page.PolicyNames {
			policy, err := iamClient.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
				RoleName:   aws.String(role.GetName()),
				PolicyName: aws.String(policyName),
			})
			// A NoSuchEntityException error means either the role or the policy no longer
			// exists. In both cases, just continue. If it was the policy that was deleted
			// concurrently, continuing is correct. If it was the role that was deleted
			// concurrently, the caller will find out when they fetch attached policies
			// next. The alternative is a fragile error string comparison, which if
			// changed could cause the role to flap.
			var noSuchEntityErr *iamtypes.NoSuchEntityException
			if errors.As(err, &noSuchEntityErr) {
				continue
			}
			if err != nil {
				errCollect(trace.Wrap(err, "failed to fetch user %q inline policy %q", role.GetName(), policyName))
				continue
			}

			policies = append(policies, awsRolePolicyToProtoUserPolicy(policy, role, a.AccountID))
		}
	}

	return policies, trace.NewAggregate(append(errs, err)...)
}

// fetchRoleAttachedPolicies fetches attached policies for an AWS role.
func (a *Fetcher) fetchRoleAttachedPolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) (*accessgraphv1alpha.AWSRoleAttachedPolicies, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListAttachedRolePoliciesPaginator(
		iamClient,
		&iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(role.GetName()),
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListAttachedRolePoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	rsp := accessgraphv1alpha.AWSRoleAttachedPolicies_builder{
		AwsRole:   role,
		AccountId: a.AccountID,
	}.Build()
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			// This may return a NoSuchEntityException error, which can refer only to
			// the role, not any attached policies. That is handled by the caller by
			// removing the role from the polling results.
			return rsp, trace.Wrap(err)
		}
		for _, policy := range page.AttachedPolicies {
			rsp.SetPolicies(append(
				rsp.GetPolicies(),
				accessgraphv1alpha.AttachedPolicyV1_builder{
					Arn:        aws.ToString(policy.PolicyArn),
					PolicyName: aws.ToString(policy.PolicyName),
				}.Build(),
			))
		}
	}
	return rsp, trace.Wrap(err)
}

// awsRoleToProtoRole converts an AWS IAM Role to a proto Role.
func awsRoleToProtoRole(role iamtypes.Role, accountID string) *accessgraphv1alpha.AWSRoleV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(role.Tags))
	for _, tag := range role.Tags {
		tags = append(tags, accessgraphv1alpha.AWSTag_builder{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		}.Build())
	}

	var permissionsBoundary *accessgraphv1alpha.RolePermissionsBoundaryV1

	if role.PermissionsBoundary != nil {
		permissionsBoundary = accessgraphv1alpha.RolePermissionsBoundaryV1_builder{
			PermissionsBoundaryArn:  aws.ToString(role.PermissionsBoundary.PermissionsBoundaryArn),
			PermissionsBoundaryType: accessgraphv1alpha.RolePermissionsBoundaryType_ROLE_PERMISSIONS_BOUNDARY_TYPE_PERMISSIONS_BOUNDARY_POLICY,
		}.Build()
	}

	var lastTimeUsed *accessgraphv1alpha.RoleLastUsedV1
	if role.RoleLastUsed != nil {
		lastTimeUsed = accessgraphv1alpha.RoleLastUsedV1_builder{
			LastUsedDate: awsTimeToProtoTime(role.RoleLastUsed.LastUsedDate),
			Region:       aws.ToString(role.RoleLastUsed.Region),
		}.Build()
	}

	return accessgraphv1alpha.AWSRoleV1_builder{
		Name:                     aws.ToString(role.RoleName),
		Arn:                      aws.ToString(role.Arn),
		AssumeRolePolicyDocument: strPtrToByteSlice(role.AssumeRolePolicyDocument),
		Path:                     aws.ToString(role.Path),
		Description:              aws.ToString(role.Description),
		MaxSessionDuration:       durationpb.New(time.Duration(aws.ToInt32(role.MaxSessionDuration)) * time.Second),
		RoleId:                   aws.ToString(role.RoleId),
		CreatedAt:                awsTimeToProtoTime(role.CreateDate),
		AccountId:                accountID,
		RoleLastUsed:             lastTimeUsed,
		Tags:                     tags,
		PermissionsBoundary:      permissionsBoundary,
	}.Build()
}

func awsRolePolicyToProtoUserPolicy(policy *iam.GetRolePolicyOutput, role *accessgraphv1alpha.AWSRoleV1, accountID string) *accessgraphv1alpha.AWSRoleInlinePolicyV1 {
	return accessgraphv1alpha.AWSRoleInlinePolicyV1_builder{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		AwsRole:        role,
		AccountId:      accountID,
	}.Build()
}
