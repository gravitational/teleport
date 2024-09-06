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
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	awsutil "github.com/gravitational/teleport/lib/utils/aws"
)

// pollAWSS3Buckets is a function that returns a function that fetches
// AWS s3 buckets and their inline and attached policies.
func (a *awsFetcher) pollAWSS3Buckets(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.S3Buckets, err = a.fetchS3Buckets(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch s3 buckets"))
		}
		return nil
	}
}

// fetchS3Buckets fetches AWS s3 buckets and returns them as a slice of
// accessgraphv1alpha.AWSS3BucketV1.
func (a *awsFetcher) fetchS3Buckets(ctx context.Context) ([]*accessgraphv1alpha.AWSS3BucketV1, error) {
	var s3s []*accessgraphv1alpha.AWSS3BucketV1
	var errs []error
	var mu sync.Mutex
	var existing = a.lastResult
	eG, ctx := errgroup.WithContext(ctx)
	// Set the limit to 5 to avoid too many concurrent requests.
	// This is a temporary solution until we have a better way to limit the
	// number of concurrent requests.
	eG.SetLimit(5)
	collect := func(s3 *accessgraphv1alpha.AWSS3BucketV1, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if s3 != nil {
			s3s = append(s3s, s3)
		}
	}

	region := awsutil.GetKnownRegions()[0]
	if len(a.Regions) > 0 {
		region = a.Regions[0]
	}

	s3Client, err := a.CloudClients.GetAWSS3Client(
		ctx,
		region,
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := s3Client.ListBucketsWithContext(
		ctx,
		&s3.ListBucketsInput{},
	)
	if err != nil {
		return existing.S3Buckets, trace.Wrap(err)
	}

	for _, bucket := range rsp.Buckets {
		bucket := bucket
		eG.Go(func() error {
			existingBucket := sliceFilterPickFirst(existing.S3Buckets, func(b *accessgraphv1alpha.AWSS3BucketV1) bool {
				return b.Name == aws.ToString(bucket.Name) && b.AccountId == a.AccountID
			},
			)
			policy, err := s3Client.GetBucketPolicyWithContext(ctx, &s3.GetBucketPolicyInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(existingBucket, trace.Wrap(err, "failed to fetch bucket %q inline policy", aws.ToString(bucket.Name)))
				return nil
			}

			policyStatus, err := s3Client.GetBucketPolicyStatusWithContext(ctx, &s3.GetBucketPolicyStatusInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(existingBucket, trace.Wrap(err, "failed to fetch bucket %q policy status", aws.ToString(bucket.Name)))
				return nil
			}

			acls, err := s3Client.GetBucketAclWithContext(ctx, &s3.GetBucketAclInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(existingBucket, trace.Wrap(err, "failed to fetch bucket %q acls policies", aws.ToString(bucket.Name)))
				return nil
			}

			tagsOutput, err := s3Client.GetBucketTaggingWithContext(ctx, &s3.GetBucketTaggingInput{
				Bucket: bucket.Name,
			})
			var awsErr awserr.Error
			const noSuchTagSet = "NoSuchTagSet" // error code when there are no tags or the bucket does not support them
			if errors.As(err, &awsErr) && awsErr.Code() == noSuchTagSet {
				// If there are no tags, set the error to nil.
				err = nil
			}
			if err != nil {
				collect(existingBucket, trace.Wrap(err, "failed to fetch bucket %q tags", aws.ToString(bucket.Name)))
				return nil
			}

			collect(
				awsS3Bucket(aws.ToString(bucket.Name), policy, policyStatus, acls, tagsOutput, a.AccountID),
				nil)
			return nil
		})
	}
	// always discard the error
	_ = eG.Wait()

	return s3s, trace.Wrap(err)
}

func awsS3Bucket(name string,
	policy *s3.GetBucketPolicyOutput,
	policyStatus *s3.GetBucketPolicyStatusOutput,
	acls *s3.GetBucketAclOutput,
	tags *s3.GetBucketTaggingOutput,
	accountID string,
) *accessgraphv1alpha.AWSS3BucketV1 {
	s3 := &accessgraphv1alpha.AWSS3BucketV1{
		Name:         name,
		AccountId:    accountID,
		LastSyncTime: timestamppb.Now(),
	}
	if policy != nil {
		s3.PolicyDocument = []byte(aws.ToString(policy.Policy))
	}
	if policyStatus != nil && policyStatus.PolicyStatus != nil {
		s3.IsPublic = aws.ToBool(policyStatus.PolicyStatus.IsPublic)
	}
	if acls != nil {
		s3.Acls = awsACLsToProtoACLs(acls.Grants)
	}
	if tags != nil {
		for _, tag := range tags.TagSet {
			s3.Tags = append(s3.Tags, &accessgraphv1alpha.AWSTag{
				Key:   aws.ToString(tag.Key),
				Value: strPtrToWrapper(tag.Value),
			})
		}
	}
	return s3
}

func awsACLsToProtoACLs(grants []*s3.Grant) []*accessgraphv1alpha.AWSS3BucketACL {
	var acls []*accessgraphv1alpha.AWSS3BucketACL
	for _, grant := range grants {
		acls = append(acls, &accessgraphv1alpha.AWSS3BucketACL{
			Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
				Id:           aws.ToString(grant.Grantee.ID),
				DisplayName:  aws.ToString(grant.Grantee.DisplayName),
				Type:         aws.ToString(grant.Grantee.Type),
				Uri:          aws.ToString(grant.Grantee.URI),
				EmailAddress: aws.ToString(grant.Grantee.EmailAddress),
			},
			Permission: aws.ToString(grant.Permission),
		})
	}
	return acls
}
