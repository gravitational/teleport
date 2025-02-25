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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// pollAWSUsers is a function that returns a function that fetches
// AWS users and their inline and attached policies, and groups.
func (a *Fetcher) pollAWSUsers(ctx context.Context, result, existing *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error

		result.Users, err = a.fetchUsers(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch users"))
			result.Users = existing.Users
		}

		eG, ctx := errgroup.WithContext(ctx)
		// Limit the number of concurrent goroutines to avoid overwhelming the AWS API.
		// These goroutines are fetching inline and attached policies for each group.
		// We also have other goroutines fetching inline and attached policies for users
		// and roles.
		eG.SetLimit(5)
		usersMu := sync.Mutex{}
		// fetch user inline policies, attached policies, and groups in parallel
		// and collect the results.
		for _, user := range result.Users {
			user := user
			eG.Go(func() error {
				userInlinePolicies, err := a.fetchUserInlinePolicies(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q inline policies", user.UserName))
					userInlinePolicies = sliceFilter(existing.UserInlinePolicies, func(inline *accessgraphv1alpha.AWSUserInlinePolicyV1) bool {
						return inline.User.UserName == user.UserName && inline.AccountId == a.AccountID
					})
				}

				userAttachedPolicies, err := a.fetchUserAttachedPolicies(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q attached policies", user.UserName))
					userAttachedPolicies = sliceFilterPickFirst(
						existing.UserAttachedPolicies,
						func(attached *accessgraphv1alpha.AWSUserAttachedPolicies) bool {
							return attached.User.UserName == user.UserName && attached.AccountId == a.AccountID
						},
					)

				}

				userGroups, err := a.fetchGroupsForUser(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q groups", user.UserName))
					userGroups = sliceFilterPickFirst(existing.UserGroups, func(groups *accessgraphv1alpha.AWSUserGroupsV1) bool {
						return groups.User.UserName == user.UserName && groups.User.AccountId == a.AccountID
					})
				}

				usersMu.Lock()
				result.UserInlinePolicies = append(result.UserInlinePolicies, userInlinePolicies...)
				if userAttachedPolicies != nil {
					result.UserAttachedPolicies = append(result.UserAttachedPolicies, userAttachedPolicies)
				}
				if userGroups != nil {
					result.UserGroups = append(result.UserGroups, userGroups)
				}
				usersMu.Unlock()
				return nil
			})
		}
		// always discard the error
		_ = eG.Wait()
		return nil
	}
}

// fetchUsers fetches AWS users and returns them as a slice of accessgraphv1alpha.AWSUserV1.
// It uses iam.ListUsersPagesWithContext to iterate over all users.
func (a *Fetcher) fetchUsers(ctx context.Context) ([]*accessgraphv1alpha.AWSUserV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListUsersPaginator(
		iamClient,
		&iam.ListUsersInput{
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListUsersPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var users []*accessgraphv1alpha.AWSUserV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return users, trace.Wrap(err)
		}
		for _, user := range page.Users {
			users = append(users, awsUserToProtoUser(user, a.AccountID))
		}
	}
	return users, trace.Wrap(err)
}

// awsUserToProtoUser converts an AWS IAM user to a proto user.
func awsUserToProtoUser(user iamtypes.User, accountID string) *accessgraphv1alpha.AWSUserV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(user.Tags))
	for _, tag := range user.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	var permissionsBoundary *accessgraphv1alpha.UsersPermissionsBoundaryV1

	if user.PermissionsBoundary != nil {
		permissionsBoundary = &accessgraphv1alpha.UsersPermissionsBoundaryV1{
			PermissionsBoundaryArn:  aws.ToString(user.PermissionsBoundary.PermissionsBoundaryArn),
			PermissionsBoundaryType: accessgraphv1alpha.UsersPermissionsBoundaryType_USERS_PERMISSIONS_BOUNDARY_TYPE_PERMISSIONS_BOUNDARY_POLICY,
		}
	}

	return &accessgraphv1alpha.AWSUserV1{
		UserName:            aws.ToString(user.UserName),
		Arn:                 aws.ToString(user.Arn),
		Path:                aws.ToString(user.Path),
		UserId:              aws.ToString(user.UserId),
		CreatedAt:           awsTimeToProtoTime(user.CreateDate),
		AccountId:           accountID,
		PasswordLastUsed:    awsTimeToProtoTime(user.PasswordLastUsed),
		Tags:                tags,
		PermissionsBoundary: permissionsBoundary,
		LastSyncTime:        timestamppb.Now(),
	}
}

func (a *Fetcher) fetchUserInlinePolicies(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) ([]*accessgraphv1alpha.AWSUserInlinePolicyV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListUserPoliciesPaginator(
		iamClient,
		&iam.ListUserPoliciesInput{
			UserName: aws.String(user.UserName),
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListUserPoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var policies []*accessgraphv1alpha.AWSUserInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return policies, trace.NewAggregate(append(errs, err)...)
		}
		for _, policyName := range page.PolicyNames {
			policy, err := iamClient.GetUserPolicy(ctx, &iam.GetUserPolicyInput{
				UserName:   aws.String(user.UserName),
				PolicyName: aws.String(policyName),
			})
			if err != nil {
				errCollect(trace.Wrap(err, "failed to fetch user %q inline policy %q", user.UserName, policyName))
				continue
			}

			policies = append(policies, awsUserPolicyToProtoUserPolicy(policy, user, a.AccountID))
		}
	}
	return policies, trace.NewAggregate(append(errs, err)...)
}

func awsUserPolicyToProtoUserPolicy(policy *iam.GetUserPolicyOutput, user *accessgraphv1alpha.AWSUserV1, accountID string) *accessgraphv1alpha.AWSUserInlinePolicyV1 {
	return &accessgraphv1alpha.AWSUserInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		User:           user,
		AccountId:      accountID,
		LastSyncTime:   timestamppb.Now(),
	}
}

func (a *Fetcher) fetchUserAttachedPolicies(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) (*accessgraphv1alpha.AWSUserAttachedPolicies, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListAttachedUserPoliciesPaginator(
		iamClient,
		&iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(user.UserName),
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListAttachedUserPoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	rsp := &accessgraphv1alpha.AWSUserAttachedPolicies{
		User:         user,
		AccountId:    a.AccountID,
		LastSyncTime: timestamppb.Now(),
	}
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return rsp, trace.Wrap(err)
		}
		for _, policy := range page.AttachedPolicies {
			rsp.Policies = append(
				rsp.Policies,
				&accessgraphv1alpha.AttachedPolicyV1{
					Arn:        aws.ToString(policy.PolicyArn),
					PolicyName: aws.ToString(policy.PolicyName),
				},
			)
		}
	}
	return rsp, trace.Wrap(err)
}

func (a *Fetcher) fetchGroupsForUser(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) (*accessgraphv1alpha.AWSUserGroupsV1, error) {
	userGroups := &accessgraphv1alpha.AWSUserGroupsV1{
		User:         user,
		LastSyncTime: timestamppb.Now(),
	}

	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)

	pager := iam.NewListGroupsForUserPaginator(
		iamClient,
		&iam.ListGroupsForUserInput{
			UserName: aws.String(user.UserName),
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListGroupsForUserPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, group := range page.Groups {
			userGroups.Groups = append(userGroups.Groups, awsGroupToProtoGroup(group, a.AccountID))
		}
	}
	return userGroups, nil
}
