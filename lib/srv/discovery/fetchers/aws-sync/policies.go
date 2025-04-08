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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// pollAWSPolicies is a function that returns a function that fetches
// AWS policies and returns an error if any.
func (a *awsFetcher) pollAWSPolicies(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.Policies, err = a.fetchPolicies(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch policies"))
		}
		return nil
	}
}

// fetchPolicies fetches AWS policies and returns them as a slice of
// accessgraphv1alpha.AWSPolicyV1.
// It uses iam.ListPoliciesPagesWithContext to iterate over all policies
// and iam.GetPolicyVersionWithContext to fetch policy documents.
func (a *awsFetcher) fetchPolicies(ctx context.Context) ([]*accessgraphv1alpha.AWSPolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSPolicyV1
	var errs []error
	var mu sync.Mutex
	var existing = a.lastResult
	collect := func(policy *accessgraphv1alpha.AWSPolicyV1, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if policy != nil {
			policies = append(policies, policy)
		}
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	eGroup, ctx := errgroup.WithContext(ctx)
	eGroup.SetLimit(5)
	pageSize := int64(20)
	err = iamClient.ListPoliciesPagesWithContext(
		ctx,
		&iam.ListPoliciesInput{
			MaxItems: aws.Int64(pageSize),
		},
		func(page *iam.ListPoliciesOutput, lastPage bool) bool {
			pp := page.Policies
			eGroup.Go(func() error {
				for _, policy := range pp {
					oldPolicy := sliceFilterPickFirst(existing.Policies, func(p *accessgraphv1alpha.AWSPolicyV1) bool {
						return p.Arn == aws.ToString(policy.Arn) && p.AccountId == a.AccountID
					})
					out, err := iamClient.GetPolicyVersionWithContext(ctx, &iam.GetPolicyVersionInput{
						PolicyArn: policy.Arn,
						VersionId: policy.DefaultVersionId,
					})
					if err != nil {
						collect(oldPolicy, trace.Wrap(err, "failed to fetch policy %q", *policy.Arn))
						continue
					}
					collect(
						awsPolicyToProtoPolicy(
							policy,
							[]byte(aws.ToString(out.PolicyVersion.Document)),
							a.AccountID,
						),
						nil,
					)
				}
				return nil
			})
			return !lastPage
		},
	)
	_ = eGroup.Wait()
	return policies, trace.NewAggregate(append(errs, err)...)
}

func awsPolicyToProtoPolicy(policy *iam.Policy, policyDoc []byte, accountID string) *accessgraphv1alpha.AWSPolicyV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(policy.Tags))
	for _, tag := range policy.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}
	return &accessgraphv1alpha.AWSPolicyV1{
		PolicyName:       aws.ToString(policy.PolicyName),
		Arn:              aws.ToString(policy.Arn),
		CreatedAt:        awsTimeToProtoTime(policy.CreateDate),
		DefaultVersionId: aws.ToString(policy.DefaultVersionId),
		Description:      aws.ToString(policy.Description),
		IsAttachable:     aws.ToBool(policy.IsAttachable),
		Path:             aws.ToString(policy.Path),
		UpdatedAt:        awsTimeToProtoTime(policy.UpdateDate),
		PolicyId:         aws.ToString(policy.PolicyId),
		Tags:             tags,
		PolicyDocument:   policyDoc,
		AccountId:        accountID,
		LastSyncTime:     timestamppb.Now(),
	}
}
