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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestPollAWSS3(t *testing.T) {
	sortSlice := func(buckets []*accessgraphv1alpha.AWSS3BucketV1) {
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Name < buckets[j].Name
		})
	}

	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "integration-test"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	require.NoError(t, err)

	const (
		accountID        = "12345678"
		bucketName       = "bucket1"
		otherBucketName  = "bucket2"
		missingBucket    = "missing_perm_bucket"
		regionlessBucket = "regionless_bucket"
	)
	var (
		regions      = []string{"eu-west-1"}
		fakedClients = fakeAWSClients{
			// The absent entries in the various fields of mocks.S3Client mean
			// the mocked calls to retrieve that data for the absent buckets will
			// fail, triggering the fallback cases in the unit under test.
			s3Client: &mocks.S3Client{
				// regionlessBucket reports no region and has no
				// GetBucketLocation entry, so its region cannot be resolved and
				// none of its details can be fetched. The rest report a region,
				// so GetBucketLocation is never consulted for them.
				Buckets: append(
					s3Buckets("eu-west-1", bucketName, otherBucketName, missingBucket),
					s3Buckets("", regionlessBucket)...,
				),
				// No BucketPolicy for missingBucket or regionlessBucket
				BucketPolicy: map[string]string{
					bucketName:      "policy",
					otherBucketName: "otherPolicy",
				},
				// No BucketPolicyStatus for missingBucket or regionlessBucket
				BucketPolicyStatus: map[string]s3types.PolicyStatus{
					bucketName: {
						IsPublic: aws.Bool(true),
					},
					otherBucketName: {
						IsPublic: aws.Bool(false),
					},
				},
				// No BucketACL for missingBucket or regionlessBucket
				BucketACL: map[string][]s3types.Grant{
					bucketName: {
						{
							Grantee: &s3types.Grantee{
								ID: aws.String("id"),
							},
							Permission: s3types.PermissionRead,
						},
					},
					otherBucketName: {
						{
							Grantee: &s3types.Grantee{
								ID: aws.String("id"),
							},
							Permission: s3types.PermissionRead,
						},
					},
				},
				// No BucketTags for otherBucketName, missingBucket or regionlessBucket
				BucketTags: map[string][]s3types.Tag{
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

	// fetchedACLs and fetchedTags are what the mocked client returns for the
	// buckets it has entries for.
	fetchedACLs := func() []*accessgraphv1alpha.AWSS3BucketACL {
		return []*accessgraphv1alpha.AWSS3BucketACL{
			{
				Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
					Id: "id",
				},
				Permission: "READ",
			},
		}
	}
	fetchedTags := func() []*accessgraphv1alpha.AWSTag {
		return []*accessgraphv1alpha.AWSTag{
			{
				Key:   "tag",
				Value: strPtrToWrapper(aws.String("val")),
			},
		}
	}

	// existing is what a previous sync recorded for a bucket. Every field a
	// request fails to fetch must be carried over from here.
	existing := func(name string) *accessgraphv1alpha.AWSS3BucketV1 {
		return &accessgraphv1alpha.AWSS3BucketV1{
			Name:           name,
			AccountId:      accountID,
			PolicyDocument: []byte("existingPolicy"),
			IsPublic:       true,
			Acls: []*accessgraphv1alpha.AWSS3BucketACL{
				{
					Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
						Id: "existingID",
					},
					Permission: "WRITE",
				},
			},
			Tags: []*accessgraphv1alpha.AWSTag{
				{
					Key:   "existingTag",
					Value: strPtrToWrapper(aws.String("existingVal")),
				},
			},
		}
	}

	tests := []struct {
		name string
		// lastResult is what the previous sync recorded.
		lastResult *Resources
		want       *Resources
	}{
		{
			name:       "poll s3",
			lastResult: &Resources{},
			want: &Resources{
				S3Buckets: []*accessgraphv1alpha.AWSS3BucketV1{
					{
						Name:           bucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("policy"),
						IsPublic:       true,
						Acls:           fetchedACLs(),
						Tags:           fetchedTags(),
					},
					{
						Name:           otherBucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("otherPolicy"),
						IsPublic:       false,
						Acls:           fetchedACLs(),
					},
					{
						Name:      missingBucket,
						AccountId: accountID,
					},
					{
						Name:      regionlessBucket,
						AccountId: accountID,
					},
				},
			},
		},
		{
			// A failed request must not look like the attribute was removed, so
			// each one falls back to what the last sync recorded.
			name: "reuse existing buckets on failure",
			lastResult: &Resources{
				S3Buckets: []*accessgraphv1alpha.AWSS3BucketV1{
					existing(bucketName),
					existing(otherBucketName),
					existing(missingBucket),
					existing(regionlessBucket),
				},
			},
			want: &Resources{
				S3Buckets: []*accessgraphv1alpha.AWSS3BucketV1{
					// Every request succeeded, so nothing is carried over.
					{
						Name:           bucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("policy"),
						IsPublic:       true,
						Acls:           fetchedACLs(),
						Tags:           fetchedTags(),
					},
					// Only the tag request failed.
					{
						Name:           otherBucketName,
						AccountId:      accountID,
						PolicyDocument: []byte("otherPolicy"),
						IsPublic:       false,
						Acls:           fetchedACLs(),
						Tags:           existing(otherBucketName).GetTags(),
					},
					// Every request failed, so every field is carried over.
					existing(missingBucket),
					// The region was never resolved, so no request was made.
					existing(regionlessBucket),
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
			a := &Fetcher{
				Config: Config{
					AWSConfigProvider: &mocks.AWSConfigProvider{
						OIDCIntegrationClient: &mocks.FakeOIDCIntegrationClient{
							Integration: awsOIDCIntegration,
							Token:       "fake-oidc-token",
						},
					},
					AccountID:   accountID,
					Regions:     regions,
					Integration: awsOIDCIntegration.GetName(),
					awsClients:  fakedClients,
				},
				lastResult: tt.lastResult,
			}
			result := &Resources{}
			execFunc := a.pollAWSS3Buckets(t.Context(), result, collectErr)
			require.NoError(t, execFunc())
			require.Error(t, trace.NewAggregate(errs...))

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
			),
			)

		})
	}
}

// failingListBucketsS3Client wraps an s3Client and returns err on the
// failOnPage'th ListBuckets call
type failingListBucketsS3Client struct {
	s3Client
	failOnPage int
	err        error
	page       int
}

func (c *failingListBucketsS3Client) ListBuckets(ctx context.Context, input *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	c.page++
	if c.page == c.failOnPage {
		return nil, c.err
	}
	return c.s3Client.ListBuckets(ctx, input, optFns...)
}

// The mock pages by the max-buckets the fetcher sends, so page boundaries are
// expressed relative to pageSize rather than chosen by each test case.
func TestListS3Buckets(t *testing.T) {
	errListBuckets := trace.AccessDenied("access denied listing buckets")

	tests := []struct {
		name              string
		totalBuckets      int
		failOnPage        int
		rejectUnpaginated bool
		wantErr           error
	}{
		{
			name:         "no buckets",
			totalBuckets: 0,
		},
		{
			name:         "partial single page",
			totalBuckets: int(pageSize) - 1,
		},
		{
			name:         "exactly one full page",
			totalBuckets: int(pageSize),
		},
		{
			name:         "multi-page with a partial final page",
			totalBuckets: int(pageSize)*4 + 1,
		},
		{
			name:         "multi-page dividing evenly",
			totalBuckets: int(pageSize) * 5,
		},
		{
			name:         "error on the first page",
			totalBuckets: int(pageSize) * 5,
			failOnPage:   1,
			wantErr:      errListBuckets,
		},
		{
			name:         "error on a later page",
			totalBuckets: int(pageSize) * 5,
			failOnPage:   3,
			wantErr:      errListBuckets,
		},
		{
			name:              "reject unpaginated requests",
			totalBuckets:      int(pageSize) * 5,
			rejectUnpaginated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantNames := make([]string, 0, tt.totalBuckets)
			for i := range tt.totalBuckets {
				wantNames = append(wantNames, "bucket-"+strconv.Itoa(i))
			}

			var client s3Client = &mocks.S3Client{
				Buckets:           s3Buckets("eu-west-1", wantNames...),
				RequireMaxBuckets: tt.rejectUnpaginated,
			}
			if tt.failOnPage > 0 {
				client = &failingListBucketsS3Client{
					s3Client:   client,
					failOnPage: tt.failOnPage,
					err:        tt.wantErr,
				}
			}

			buckets, err := listS3Buckets(t.Context(), client)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				// A failed page discards the whole listing, so a partial list
				// is never mistaken for the account's full set of buckets.
				require.Nil(t, buckets)
				return
			}

			require.NoError(t, err)
			gotNames := make([]string, 0, len(buckets))
			for _, b := range buckets {
				gotNames = append(gotNames, aws.ToString(b.Name))
			}
			require.Equal(t, wantNames, gotNames)
		})
	}
}

func TestBucketRegion(t *testing.T) {
	const bucketName = "bucket"

	tests := []struct {
		name string
		// listedRegion is the BucketRegion reported by ListBuckets. Empty means
		// the response omitted it.
		listedRegion string
		// locations are the GetBucketLocation responses, keyed by bucket name.
		// A bucket absent from the map makes the lookup fail.
		locations  map[string]s3types.BucketLocationConstraint
		wantRegion string
		wantErr    string
	}{
		{
			name:         "BucketRegion is used when reported",
			listedRegion: "ap-south-1",
			// locations is empty, so any GetBucketLocation call fails the test.
			wantRegion: "ap-south-1",
		},
		{
			name:       "absent BucketRegion falls back to GetBucketLocation",
			locations:  map[string]s3types.BucketLocationConstraint{bucketName: "eu-west-1"},
			wantRegion: "eu-west-1",
		},
		{
			name:       "an empty location constraint means us-east-1",
			locations:  map[string]s3types.BucketLocationConstraint{bucketName: ""},
			wantRegion: "us-east-1",
		},
		{
			name:    "a failed lookup is an error",
			wantErr: `failed to fetch bucket "bucket" region`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clt := &mocks.S3Client{BucketLocations: tt.locations}
			bucket := s3Buckets(tt.listedRegion, bucketName)[0]

			region, err := getBucketRegion(t.Context(), clt, bucket)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantRegion, region)
		})
	}
}

// s3Buckets builds ListBuckets results. An empty region models a response that
// omitted BucketRegion, which is what AWS returns for unpaginated requests.
func s3Buckets(region string, bucketNames ...string) []s3types.Bucket {
	var output []s3types.Bucket
	for _, name := range bucketNames {
		bucket := s3types.Bucket{
			Name:         aws.String(name),
			CreationDate: aws.Time(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		}
		if region != "" {
			bucket.BucketRegion = aws.String(region)
		}
		output = append(output, bucket)
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
