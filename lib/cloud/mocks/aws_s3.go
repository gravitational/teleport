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

package mocks

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"
)

// S3Client mocks AWS S3 API.
type S3Client struct {
	Buckets            []s3types.Bucket
	BucketPolicy       map[string]string
	BucketPolicyStatus map[string]s3types.PolicyStatus
	BucketACL          map[string][]s3types.Grant
	BucketTags         map[string][]s3types.Tag
	BucketLocations    map[string]s3types.BucketLocationConstraint
	// RequireMaxBuckets rejects unpaginated ListBuckets requests, as AWS does
	// for accounts with a bucket quota above maxListBuckets.
	RequireMaxBuckets bool
}

// maxListBuckets is the largest page size ListBuckets accepts for max-buckets.
const maxListBuckets = 10000

// ListBuckets pages through Buckets using max-buckets as the page size. The
// continuation token is the index of the first bucket of the next page.
func (m *S3Client) ListBuckets(_ context.Context, input *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	if input.MaxBuckets == nil {
		if m.RequireMaxBuckets {
			return nil, trace.BadParameter("unpaginated ListBuckets requests are rejected for accounts with a bucket quota greater than %d", maxListBuckets)
		}
		return &s3.ListBucketsOutput{
			Buckets: m.Buckets,
		}, nil
	}
	pageSize := int(aws.ToInt32(input.MaxBuckets))
	if pageSize < 1 || pageSize > maxListBuckets {
		return nil, trace.BadParameter("max-buckets must be between 1 and %d, got %d", maxListBuckets, pageSize)
	}

	start := 0
	if token := aws.ToString(input.ContinuationToken); token != "" {
		var err error
		start, err = strconv.Atoi(token)
		if err != nil {
			return nil, trace.BadParameter("invalid continuation token %q", token)
		}
	}
	end := min(start+pageSize, len(m.Buckets))
	out := &s3.ListBucketsOutput{
		Buckets: m.Buckets[start:end],
	}
	if end < len(m.Buckets) {
		out.ContinuationToken = aws.String(strconv.Itoa(end))
	}
	return out, nil
}

func (m *S3Client) GetBucketPolicy(_ context.Context, input *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	if aws.ToString(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	policy, ok := m.BucketPolicy[aws.ToString(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.ToString(input.Bucket))
	}
	return &s3.GetBucketPolicyOutput{
		Policy: aws.String(policy),
	}, nil
}

func (m *S3Client) GetBucketPolicyStatus(_ context.Context, input *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if aws.ToString(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	policyStatus, ok := m.BucketPolicyStatus[aws.ToString(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.ToString(input.Bucket))
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: &policyStatus,
	}, nil
}

func (m *S3Client) GetBucketAcl(_ context.Context, input *s3.GetBucketAclInput, _ ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	if aws.ToString(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	grants, ok := m.BucketACL[aws.ToString(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.ToString(input.Bucket))
	}
	return &s3.GetBucketAclOutput{
		Grants: grants,
	}, nil
}

func (m *S3Client) GetBucketTagging(_ context.Context, input *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	if aws.ToString(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	tags, ok := m.BucketTags[aws.ToString(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("The specified bucket does not have a tag set")
	}
	return &s3.GetBucketTaggingOutput{
		TagSet: tags,
	}, nil
}

func (m *S3Client) GetBucketLocation(_ context.Context, input *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if aws.ToString(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	location, ok := m.BucketLocations[aws.ToString(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.ToString(input.Bucket))
	}
	return &s3.GetBucketLocationOutput{
		LocationConstraint: location,
	}, nil
}
