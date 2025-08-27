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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	awsregion "github.com/gravitational/teleport/lib/utils/aws/region"
)

// s3Client defines a subset of the AWS S3 client API.
type s3Client interface {
	s3.ListBucketsAPIClient

	GetBucketAcl(context.Context, *s3.GetBucketAclInput, ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	GetBucketLocation(context.Context, *s3.GetBucketLocationInput, ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	GetBucketPolicy(context.Context, *s3.GetBucketPolicyInput, ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error)
	GetBucketPolicyStatus(context.Context, *s3.GetBucketPolicyStatusInput, ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
	GetBucketTagging(context.Context, *s3.GetBucketTaggingInput, ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error)
}

// pollAWSS3Buckets is a function that returns a function that fetches
// AWS s3 buckets and their inline and attached policies.
func (a *Fetcher) pollAWSS3Buckets(ctx context.Context, result *Resources, collectErr func(error)) func() error {
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
func (a *Fetcher) fetchS3Buckets(ctx context.Context) ([]*accessgraphv1alpha.AWSS3BucketV1, error) {
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

	buckets, getBucketRegion, err := a.listS3Buckets(ctx)
	if err != nil {
		return existing.S3Buckets, trace.Wrap(err)
	}

	// Iterate over the buckets and fetch their inline and attached policies.
	for _, bucket := range buckets {
		eG.Go(func() error {
			var failedReqs failedRequests
			var errs []error
			existingBucket := sliceFilterPickFirst(existing.S3Buckets, func(b *accessgraphv1alpha.AWSS3BucketV1) bool {
				return b.Name == aws.ToString(bucket.Name) && b.AccountId == a.AccountID
			},
			)
			bucketRegion, err := getBucketRegion(bucket.Name)
			if err != nil {
				errs = append(errs,
					trace.Wrap(err),
				)
				failedReqs.policyFailed = true
				failedReqs.failedPolicyStatus = true
				failedReqs.failedAcls = true
				failedReqs.failedTags = true
				newBucket := awsS3Bucket(aws.ToString(bucket.Name), nil, nil, nil, nil, a.AccountID)
				collect(mergeS3Protos(existingBucket, newBucket, failedReqs), trace.NewAggregate(errs...))
				return nil
			}

			details, failedReqs, errsL := a.getS3BucketDetails(ctx, bucket, bucketRegion)

			newBucket := awsS3Bucket(aws.ToString(bucket.Name), details.policy, details.policyStatus, details.acls, details.tags, a.AccountID)
			collect(mergeS3Protos(existingBucket, newBucket, failedReqs), trace.NewAggregate(append(errs, errsL...)...))
			return nil
		})
	}
	// always discard the error
	_ = eG.Wait()

	return s3s, trace.NewAggregate(errs...)
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

func awsACLsToProtoACLs(grants []s3types.Grant) []*accessgraphv1alpha.AWSS3BucketACL {
	var acls []*accessgraphv1alpha.AWSS3BucketACL
	for _, grant := range grants {
		acls = append(acls, &accessgraphv1alpha.AWSS3BucketACL{
			Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
				Id:           aws.ToString(grant.Grantee.ID),
				DisplayName:  aws.ToString(grant.Grantee.DisplayName),
				Type:         string(grant.Grantee.Type),
				Uri:          aws.ToString(grant.Grantee.URI),
				EmailAddress: aws.ToString(grant.Grantee.EmailAddress),
			},
			Permission: string(grant.Permission),
		})
	}
	return acls
}

type failedRequests struct {
	policyFailed       bool
	failedPolicyStatus bool
	failedAcls         bool
	failedTags         bool
	headFailed         bool
}

func mergeS3Protos(existing, new *accessgraphv1alpha.AWSS3BucketV1, failedReqs failedRequests) *accessgraphv1alpha.AWSS3BucketV1 {
	if existing == nil {
		return new
	}
	if new == nil {
		return existing
	}
	clone := proto.Clone(new).(*accessgraphv1alpha.AWSS3BucketV1)
	if failedReqs.policyFailed {
		clone.PolicyDocument = existing.PolicyDocument
	}
	if failedReqs.failedPolicyStatus {
		clone.IsPublic = existing.IsPublic
	}
	if failedReqs.failedAcls {
		clone.Acls = existing.Acls
	}
	if failedReqs.failedTags {
		clone.Tags = existing.Tags
	}

	return clone
}

type s3Details struct {
	policy       *s3.GetBucketPolicyOutput
	policyStatus *s3.GetBucketPolicyStatusOutput
	acls         *s3.GetBucketAclOutput
	tags         *s3.GetBucketTaggingOutput
}

func (a *Fetcher) getS3BucketDetails(ctx context.Context, bucket s3types.Bucket, bucketRegion string) (s3Details, failedRequests, []error) {
	var failedReqs failedRequests
	var errs []error
	var details s3Details

	awsCfg, err := a.AWSConfigProvider.GetConfig(ctx, bucketRegion, a.getAWSOptions()...)
	if err != nil {
		errs = append(errs,
			trace.Wrap(err, "failed to create s3 client for bucket %q", aws.ToString(bucket.Name)),
		)
		return s3Details{},
			failedRequests{
				headFailed:         true,
				policyFailed:       true,
				failedPolicyStatus: true,
				failedAcls:         true,
				failedTags:         true,
			}, errs
	}
	s3Client := a.awsClients.getS3Client(awsCfg)

	details.policy, err = s3Client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: bucket.Name,
	})
	if err != nil && !isS3BucketPolicyNotFound(err) {
		errs = append(errs,
			trace.Wrap(err, "failed to fetch bucket %q inline policy", aws.ToString(bucket.Name)),
		)
		failedReqs.policyFailed = true
	}

	details.policyStatus, err = s3Client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{
		Bucket: bucket.Name,
	})
	if err != nil && !isS3BucketPolicyNotFound(err) {
		errs = append(errs,
			trace.Wrap(err, "failed to fetch bucket %q policy status", aws.ToString(bucket.Name)),
		)
		failedReqs.failedPolicyStatus = true
	}

	details.acls, err = s3Client.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: bucket.Name,
	})
	if err != nil {
		errs = append(errs,
			trace.Wrap(err, "failed to fetch bucket %q acls policies", aws.ToString(bucket.Name)),
		)
		failedReqs.failedAcls = true
	}

	details.tags, err = s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: bucket.Name,
	})
	if err != nil && !isS3BucketNoTagSet(err) {
		errs = append(errs,
			trace.Wrap(err, "failed to fetch bucket %q tags", aws.ToString(bucket.Name)),
		)
		failedReqs.failedTags = true
	}

	return details, failedReqs, errs
}

func isAPIErrorCode(err error, code string) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode() == code
	}
	return false
}

func isS3BucketPolicyNotFound(err error) bool {
	return isAPIErrorCode(err, "NoSuchBucketPolicy")
}

func isS3BucketNoTagSet(err error) bool {
	return isAPIErrorCode(err, "NoSuchTagSet")
}

func (a *Fetcher) listS3Buckets(ctx context.Context) ([]s3types.Bucket, func(*string) (string, error), error) {
	region := awsregion.GetKnownRegions()[0]
	if len(a.Regions) > 0 {
		region = a.Regions[0]
	}

	// use any region to list buckets
	awsCfg, err := a.AWSConfigProvider.GetConfig(ctx, region, a.getAWSOptions()...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s3Client := a.awsClients.getS3Client(awsCfg)
	rsp, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return rsp.Buckets,
		func(bucket *string) (string, error) {
			rsp, err := s3Client.GetBucketLocation(
				ctx,
				&s3.GetBucketLocationInput{
					Bucket: bucket,
				},
			)
			if err != nil {
				return "", trace.Wrap(err, "failed to fetch bucket %q region", aws.ToString(bucket))
			}
			if rsp.LocationConstraint == "" {
				return "us-east-1", nil
			}
			return string(rsp.LocationConstraint), nil
		}, nil
}
