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
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestPollAWSS3(t *testing.T) {
	sortSlice := func(buckets []*accessgraphv1alpha.AWSS3BucketV1) {
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Name < buckets[j].Name
		})
	}

	const (
		accountID       = "12345678"
		bucketName      = "bucket1"
		otherBucketName = "bucket2"
	)
	var (
		regions       = []string{"eu-west-1"}
		mockedClients = &cloud.TestCloudClients{
			S3: &mocks.S3Mock{
				Buckets: s3Buckets(bucketName, otherBucketName),
				BucketPolicy: map[string]string{
					bucketName:      "policy",
					otherBucketName: "otherPolicy",
				},
				BucketPolicyStatus: map[string]*s3.PolicyStatus{
					bucketName: {
						IsPublic: aws.Bool(true),
					},
					otherBucketName: {
						IsPublic: aws.Bool(false),
					},
				},
				BucketACL: map[string][]*s3.Grant{
					bucketName: {
						{
							Grantee: &s3.Grantee{
								ID: aws.String("id"),
							},
							Permission: aws.String("READ"),
						},
					},
					otherBucketName: {
						{
							Grantee: &s3.Grantee{
								ID: aws.String("id"),
							},
							Permission: aws.String("READ"),
						},
					},
				},
				BucketTags: map[string][]*s3.Tag{
					bucketName: {
						{
							Key:   aws.String("tag"),
							Value: aws.String("val"),
						},
					},
				},
			},
		}
	)

	tests := []struct {
		name string
		want *Resources
	}{
		{
			name: "poll s3",
			want: &Resources{
				S3Buckets: []*accessgraphv1alpha.AWSS3BucketV1{
					{
						Name:           bucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("policy"),
						IsPublic:       true,
						Acls: []*accessgraphv1alpha.AWSS3BucketACL{
							{
								Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
									Id: "id",
								},
								Permission: "READ",
							},
						},
						Tags: []*accessgraphv1alpha.AWSTag{
							{
								Key:   "tag",
								Value: strPtrToWrapper(aws.String("val")),
							},
						},
					},
					{
						Name:           otherBucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("otherPolicy"),
						IsPublic:       false,
						Acls: []*accessgraphv1alpha.AWSS3BucketACL{
							{
								Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
									Id: "id",
								},
								Permission: "READ",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var (
				errs []error
				mu   sync.Mutex
			)

			collectErr := func(err error) {
				mu.Lock()
				defer mu.Unlock()
				errs = append(errs, err)
			}
			a := &awsFetcher{
				Config: Config{
					AccountID:    accountID,
					CloudClients: mockedClients,
					Regions:      regions,
					Integration:  accountID,
				},
				lastResult: &Resources{},
			}
			result := &Resources{}
			execFunc := a.pollAWSS3Buckets(context.Background(), result, collectErr)
			require.NoError(t, execFunc())
			require.NoError(t, trace.NewAggregate(errs...))

			sortSlice(tt.want.S3Buckets)
			sortSlice(result.S3Buckets)
			require.Empty(t, cmp.Diff(
				tt.want,
				result,
				protocmp.Transform(),
				// tags originate from a map so we must sort them before comparing.
				protocmp.SortRepeated(
					func(a, b *accessgraphv1alpha.AWSTag) bool {
						return a.Key < b.Key
					},
				),
				protocmp.IgnoreFields(&accessgraphv1alpha.AWSS3BucketV1{}, "last_sync_time"),
			),
			)

		})
	}
}

func s3Buckets(bucketNames ...string) []*s3.Bucket {
	var output []*s3.Bucket
	for _, name := range bucketNames {
		output = append(output, &s3.Bucket{
			Name:         aws.String(name),
			CreationDate: aws.Time(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		})

	}
	return output
}

// Helper function to create AWSS3BucketV1 for testing
func createAWSS3Bucket(name, accountID string, policyDocument []byte, isPublic bool, lastSync time.Time) *accessgraphv1alpha.AWSS3BucketV1 {
	return &accessgraphv1alpha.AWSS3BucketV1{
		Name:           name,
		AccountId:      accountID,
		PolicyDocument: policyDocument,
		IsPublic:       isPublic,
		LastSyncTime:   timestamppb.New(lastSync),
	}
}

func TestMergeS3Protos(t *testing.T) {
	// Define a common time for the test
	lastSync := time.Now()

	// Define test cases in a table-driven format
	tests := []struct {
		name       string
		existing   *accessgraphv1alpha.AWSS3BucketV1
		new        *accessgraphv1alpha.AWSS3BucketV1
		failedReqs failedRequests
		expected   *accessgraphv1alpha.AWSS3BucketV1
	}{
		{
			name:     "Both existing and new are nil",
			existing: nil,
			new:      nil,
			failedReqs: failedRequests{
				policyFailed:       false,
				failedPolicyStatus: false,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: nil,
		},
		{
			name:     "Existing is nil, new is non-nil",
			existing: nil,
			new:      createAWSS3Bucket("new-bucket", "account-1", []byte("policy"), true, lastSync),
			failedReqs: failedRequests{
				policyFailed:       false,
				failedPolicyStatus: false,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: createAWSS3Bucket("new-bucket", "account-1", []byte("policy"), true, lastSync),
		},
		{
			name:     "New is nil, existing is non-nil",
			existing: createAWSS3Bucket("existing-bucket", "account-1", []byte("existing-policy"), false, lastSync),
			new:      nil,
			failedReqs: failedRequests{
				policyFailed:       false,
				failedPolicyStatus: false,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: createAWSS3Bucket("existing-bucket", "account-1", []byte("existing-policy"), false, lastSync),
		},
		{
			name:     "New and existing both non-nil, no failures",
			existing: createAWSS3Bucket("existing-bucket", "account-1", []byte("existing-policy"), false, lastSync),
			new:      createAWSS3Bucket("new-bucket", "account-2", []byte("new-policy"), true, lastSync),
			failedReqs: failedRequests{
				policyFailed:       false,
				failedPolicyStatus: false,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: createAWSS3Bucket("new-bucket", "account-2", []byte("new-policy"), true, lastSync),
		},
		{
			name:     "Policy merge failed",
			existing: createAWSS3Bucket("existing-bucket", "account-1", []byte("existing-policy"), false, lastSync),
			new:      createAWSS3Bucket("new-bucket", "account-2", []byte("new-policy"), true, lastSync),
			failedReqs: failedRequests{
				policyFailed:       true,
				failedPolicyStatus: false,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: createAWSS3Bucket("new-bucket", "account-2", []byte("existing-policy"), true, lastSync),
		},
		{
			name:     "Policy status merge failed",
			existing: createAWSS3Bucket("existing-bucket", "account-1", []byte("existing-policy"), false, lastSync),
			new:      createAWSS3Bucket("new-bucket", "account-2", []byte("new-policy"), true, lastSync),
			failedReqs: failedRequests{
				policyFailed:       false,
				failedPolicyStatus: true,
				failedAcls:         false,
				failedTags:         false,
			},
			expected: createAWSS3Bucket("new-bucket", "account-2", []byte("new-policy"), false, lastSync),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeS3Protos(tt.existing, tt.new, tt.failedReqs)

			require.Empty(t, cmp.Diff(tt.expected, result, protocmp.Transform()))
		})
	}
}
