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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/durationpb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// pollAWSRoles is a function that returns a function that fetches
// AWS roles and their inline and attached policies.
func (a *awsFetcher) pollAWSRoles(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error

		result.Roles, err = a.fetchRoles(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch roles"))
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		// Limit the number of concurrent goroutines to avoid overwhelming the AWS API.
		// These goroutines are fetching inline and attached policies for each group.
		// We also have other goroutines fetching inline and attached policies for users
		// and roles.
		eG.SetLimit(5)
		roleMu := sync.Mutex{}
		for _, role := range result.Roles {
			role := role
			eG.Go(func() error {
				roleInlinePolicies, err := a.fetchRoleInlinePolicies(ctx, role)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch role %q inline policies", role.Name))
				}

				roleAttachedPolicies, err := a.fetchRoleAttachedPolicies(ctx, role)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch role %q attached policies", role.Name))
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
		return nil
	}
}

// fetchRoles fetches AWS roles and returns them as a slice of accessgraphv1alpha.AWSRoleV1.
func (a *awsFetcher) fetchRoles(ctx context.Context) ([]*accessgraphv1alpha.AWSRoleV1, error) {
	var roles []*accessgraphv1alpha.AWSRoleV1

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because roles are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListRolesPagesWithContext(ctx, &iam.ListRolesInput{
		MaxItems: aws.Int64(pageSize),
	},
		func(page *iam.ListRolesOutput, lastPage bool) bool {
			for _, role := range page.Roles {
				roles = append(roles, awsRoleToProtoRole(role, a.AccountID))
			}
			return !lastPage
		},
	)

	return roles, trace.Wrap(err)
}

// fetchRoleInlinePolicies fetches inline policies for an AWS role and returns
// them as a slice of accessgraphv1alpha.AWSRoleInlinePolicyV1.
// It uses iam.ListRolePoliciesPagesWithContext to iterate over all inline policies
// and iam.GetRolePolicyWithContext to fetch policy documents.
func (a *awsFetcher) fetchRoleInlinePolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) ([]*accessgraphv1alpha.AWSRoleInlinePolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSRoleInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = iamClient.ListRolePoliciesPagesWithContext(
		ctx,
		&iam.ListRolePoliciesInput{
			RoleName: aws.String(role.Name),
			MaxItems: aws.Int64(pageSize),
		},
		func(page *iam.ListRolePoliciesOutput, lastPage bool) bool {
			for _, policyName := range page.PolicyNames {
				policy, err := iamClient.GetRolePolicyWithContext(ctx, &iam.GetRolePolicyInput{
					RoleName:   aws.String(role.Name),
					PolicyName: policyName,
				})
				if err != nil {
					errCollect(trace.Wrap(err, "failed to fetch user %q inline policy %q", role.Name, *policyName))
					continue
				}

				policies = append(policies, awsRolePolicyToProtoUserPolicy(policy, role, a.AccountID))
			}
			return !lastPage
		})

	return policies, trace.NewAggregate(append(errs, err)...)
}

// fetchRoleAttachedPolicies fetches attached policies for an AWS role.
func (a *awsFetcher) fetchRoleAttachedPolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) (*accessgraphv1alpha.AWSRoleAttachedPolicies, error) {
	rsp := &accessgraphv1alpha.AWSRoleAttachedPolicies{
		AwsRole:   role,
		AccountId: a.AccountID,
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListAttachedRolePoliciesPagesWithContext(
		ctx,
		&iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(role.Name),
			MaxItems: aws.Int64(pageSize),
		},
		func(page *iam.ListAttachedRolePoliciesOutput, lastPage bool) bool {
			for _, policy := range page.AttachedPolicies {
				rsp.Policies = append(
					rsp.Policies,
					&accessgraphv1alpha.AttachedPolicyV1{
						Arn:        aws.ToString(policy.PolicyArn),
						PolicyName: aws.ToString(policy.PolicyName),
					},
				)
			}
			return !lastPage
		},
	)

	return rsp, trace.Wrap(err)
}

// awsRoleToProtoRole converts an AWS IAM Role to a proto Role.
func awsRoleToProtoRole(role *iam.Role, accountID string) *accessgraphv1alpha.AWSRoleV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(role.Tags))
	for _, tag := range role.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	var permissionsBoundary *accessgraphv1alpha.RolePermissionsBoundaryV1

	if role.PermissionsBoundary != nil {
		permissionsBoundary = &accessgraphv1alpha.RolePermissionsBoundaryV1{
			PermissionsBoundaryArn:  aws.ToString(role.PermissionsBoundary.PermissionsBoundaryArn),
			PermissionsBoundaryType: accessgraphv1alpha.RolePermissionsBoundaryType_ROLE_PERMISSIONS_BOUNDARY_TYPE_PERMISSIONS_BOUNDARY_POLICY,
		}
	}

	var lastTimeUsed *accessgraphv1alpha.RoleLastUsedV1
	if role.RoleLastUsed != nil {
		lastTimeUsed = &accessgraphv1alpha.RoleLastUsedV1{
			LastUsedDate: awsTimeToProtoTime(role.RoleLastUsed.LastUsedDate),
			Region:       aws.ToString(role.RoleLastUsed.Region),
		}
	}

	return &accessgraphv1alpha.AWSRoleV1{
		Name:                     aws.ToString(role.RoleName),
		Arn:                      aws.ToString(role.Arn),
		AssumeRolePolicyDocument: strPtrToByteSlice(role.AssumeRolePolicyDocument),
		Path:                     aws.ToString(role.Path),
		Description:              aws.ToString(role.Description),
		MaxSessionDuration:       durationpb.New(time.Duration(aws.ToInt64(role.MaxSessionDuration)) * time.Second),
		RoleId:                   aws.ToString(role.RoleId),
		CreatedAt:                awsTimeToProtoTime(role.CreateDate),
		AccountId:                accountID,
		RoleLastUsed:             lastTimeUsed,
		Tags:                     tags,
		PermissionsBoundary:      permissionsBoundary,
	}
}

func awsRolePolicyToProtoUserPolicy(policy *iam.GetRolePolicyOutput, role *accessgraphv1alpha.AWSRoleV1, accountID string) *accessgraphv1alpha.AWSRoleInlinePolicyV1 {
	return &accessgraphv1alpha.AWSRoleInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		AwsRole:        role,
		AccountId:      accountID,
	}
}
