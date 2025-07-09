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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/gravitational/trace"
)

// S3Mock mocks AWS S3 API.
type S3Mock struct {
	s3iface.S3API
	Buckets            []*s3.Bucket
	BucketPolicy       map[string]string
	BucketPolicyStatus map[string]*s3.PolicyStatus
	BucketACL          map[string][]*s3.Grant
	BucketTags         map[string][]*s3.Tag
	BucketLocations    map[string]string
}

func (m *S3Mock) ListBucketsWithContext(_ aws.Context, _ *s3.ListBucketsInput, _ ...request.Option) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{
		Buckets: m.Buckets,
	}, nil
}

func (m *S3Mock) GetBucketPolicyWithContext(_ aws.Context, input *s3.GetBucketPolicyInput, _ ...request.Option) (*s3.GetBucketPolicyOutput, error) {
	if aws.StringValue(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	policy, ok := m.BucketPolicy[aws.StringValue(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.StringValue(input.Bucket))
	}
	return &s3.GetBucketPolicyOutput{
		Policy: aws.String(policy),
	}, nil
}

func (m *S3Mock) GetBucketPolicyStatusWithContext(_ aws.Context, input *s3.GetBucketPolicyStatusInput, _ ...request.Option) (*s3.GetBucketPolicyStatusOutput, error) {
	if aws.StringValue(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	policyStatus, ok := m.BucketPolicyStatus[aws.StringValue(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.StringValue(input.Bucket))
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: policyStatus,
	}, nil
}

func (m *S3Mock) GetBucketAclWithContext(_ aws.Context, input *s3.GetBucketAclInput, _ ...request.Option) (*s3.GetBucketAclOutput, error) {
	if aws.StringValue(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	grants, ok := m.BucketACL[aws.StringValue(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.StringValue(input.Bucket))
	}
	return &s3.GetBucketAclOutput{
		Grants: grants,
	}, nil
}

func (m *S3Mock) GetBucketTaggingWithContext(_ aws.Context, input *s3.GetBucketTaggingInput, _ ...request.Option) (*s3.GetBucketTaggingOutput, error) {
	if aws.StringValue(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	tags, ok := m.BucketTags[aws.StringValue(input.Bucket)]
	if !ok {
		return nil, awserr.New("NoSuchTagSet", "The specified bucket does not have a tag set", nil)
	}
	return &s3.GetBucketTaggingOutput{
		TagSet: tags,
	}, nil
}

func (m *S3Mock) GetBucketLocationWithContext(_ aws.Context, input *s3.GetBucketLocationInput, _ ...request.Option) (*s3.GetBucketLocationOutput, error) {
	if aws.StringValue(input.Bucket) == "" {
		return nil, trace.BadParameter("incorrect bucket name")
	}
	location, ok := m.BucketLocations[aws.StringValue(input.Bucket)]
	if !ok {
		return nil, trace.NotFound("bucket %v not found", aws.StringValue(input.Bucket))
	}
	return &s3.GetBucketLocationOutput{
		LocationConstraint: aws.String(location),
	}, nil
}
