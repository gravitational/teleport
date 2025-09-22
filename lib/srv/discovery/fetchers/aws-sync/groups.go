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

// pollAWSGroups is a function that returns a function that fetches
// AWS groups and their inline and attached policies.
func (a *Fetcher) pollAWSGroups(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error

		result.Groups, err = a.fetchGroups(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch groups"))
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		// Limit the number of concurrent goroutines to avoid overwhelming the AWS API.
		// These goroutines are fetching inline and attached policies for each group.
		// We also have other goroutines fetching inline and attached policies for users
		// and roles.
		eG.SetLimit(5)
		groupsMu := sync.Mutex{}
		var existing = a.lastResult
		for _, group := range result.Groups {
			eG.Go(func() error {
				groupInlinePolicies, err := a.fetchGroupInlinePolicies(ctx, group)
				if err != nil {
					groupInlinePolicies = sliceFilter(existing.GroupInlinePolicies, func(inline *accessgraphv1alpha.AWSGroupInlinePolicyV1) bool {
						return inline.Group.Name == group.Name && inline.Group.AccountId == group.AccountId
					})
					collectErr(trace.Wrap(err, "failed to fetch group %q inline policies", group.Name))
				}

				groupAttachedPolicies, err := a.fetchGroupAttachedPolicies(ctx, group)
				if err != nil {
					groupAttachedPolicies = sliceFilterPickFirst(existing.GroupAttachedPolicies, func(inline *accessgraphv1alpha.AWSGroupAttachedPolicies) bool {
						return inline.Group.Name == group.Name && inline.Group.AccountId == group.AccountId
					})
					collectErr(trace.Wrap(err, "failed to fetch group %q attached policies", group.Name))
				}

				groupsMu.Lock()
				if groupAttachedPolicies != nil {
					result.GroupAttachedPolicies = append(result.GroupAttachedPolicies, groupAttachedPolicies)
				}
				result.GroupInlinePolicies = append(result.GroupInlinePolicies, groupInlinePolicies...)
				groupsMu.Unlock()
				return nil
			})
		}

		// always discard the error
		_ = eG.Wait()

		return nil
	}
}

// fetchGroups fetches AWS groups and returns them as a slice of accessgraphv1alpha.AWSGroupV1.
// It uses ListGroupsPagesWithContext to iterate over all groups.
func (a *Fetcher) fetchGroups(ctx context.Context) ([]*accessgraphv1alpha.AWSGroupV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return a.lastResult.Groups, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListGroupsPaginator(
		iamClient,
		&iam.ListGroupsInput{
			MaxItems: aws.Int32(pageSize),
		},
		func(opts *iam.ListGroupsPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var groups []*accessgraphv1alpha.AWSGroupV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return groups, trace.Wrap(err)
		}
		for _, group := range page.Groups {
			groups = append(groups, awsGroupToProtoGroup(group, a.AccountID))
		}
	}
	return groups, trace.Wrap(err)
}

// awsGroupToProtoGroup converts an AWS IAM group to a proto group.
func awsGroupToProtoGroup(group iamtypes.Group, accountID string) *accessgraphv1alpha.AWSGroupV1 {
	return &accessgraphv1alpha.AWSGroupV1{
		Name:         aws.ToString(group.GroupName),
		Arn:          aws.ToString(group.Arn),
		Path:         aws.ToString(group.Path),
		GroupId:      aws.ToString(group.GroupId),
		CreatedAt:    awsTimeToProtoTime(group.CreateDate),
		AccountId:    accountID,
		LastSyncTime: timestamppb.Now(),
	}
}

// fetchGroupInlinePolicies fetches inline policies for a group and returns them
// as a slice of accessgraphv1alpha.AWSGroupInlinePolicyV1.
// It uses ListGroupPoliciesPagesWithContext to iterate over all inline policies
// associated with the group.
func (a *Fetcher) fetchGroupInlinePolicies(ctx context.Context, group *accessgraphv1alpha.AWSGroupV1) ([]*accessgraphv1alpha.AWSGroupInlinePolicyV1, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListGroupPoliciesPaginator(
		iamClient,
		&iam.ListGroupPoliciesInput{
			GroupName: aws.String(group.Name),
			MaxItems:  aws.Int32(pageSize),
		},
		func(opts *iam.ListGroupPoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	var policies []*accessgraphv1alpha.AWSGroupInlinePolicyV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return policies, trace.Wrap(err)
		}
		for _, policyName := range page.PolicyNames {
			policy, err := iamClient.GetGroupPolicy(ctx, &iam.GetGroupPolicyInput{
				GroupName:  aws.String(group.Name),
				PolicyName: aws.String(policyName),
			})
			if err != nil {
				errCollect(trace.Wrap(err, "failed to fetch group %q inline policy %q", group.Name, policyName))
				continue
			}
			policies = append(policies, awsGroupPolicyToProtoGroupPolicy(policy, a.AccountID, group))
		}
	}
	return policies, trace.Wrap(err)
}

func awsGroupPolicyToProtoGroupPolicy(policy *iam.GetGroupPolicyOutput, accountID string, group *accessgraphv1alpha.AWSGroupV1) *accessgraphv1alpha.AWSGroupInlinePolicyV1 {
	return &accessgraphv1alpha.AWSGroupInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		Group:          group,
		AccountId:      accountID,
		LastSyncTime:   timestamppb.Now(),
	}
}

// fetchGroupAttachedPolicies fetches attached policies for a group.
func (a *Fetcher) fetchGroupAttachedPolicies(ctx context.Context, group *accessgraphv1alpha.AWSGroupV1) (*accessgraphv1alpha.AWSGroupAttachedPolicies, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClient := a.awsClients.getIAMClient(awsCfg)
	pager := iam.NewListAttachedGroupPoliciesPaginator(
		iamClient,
		&iam.ListAttachedGroupPoliciesInput{
			GroupName: aws.String(group.Name),
			MaxItems:  aws.Int32(pageSize),
		},
		func(opts *iam.ListAttachedGroupPoliciesPaginatorOptions) {
			opts.StopOnDuplicateToken = true
		},
	)

	rsp := &accessgraphv1alpha.AWSGroupAttachedPolicies{
		Group:        group,
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
