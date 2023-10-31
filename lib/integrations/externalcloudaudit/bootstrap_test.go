/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package externalcloudaudit_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"

	ecatypes "github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/lib/integrations/externalcloudaudit"
)

func TestBootstrapInfra(t *testing.T) {
	tt := []struct {
		desc        string
		region      string
		eca         *ecatypes.ExternalCloudAuditSpec
		errExpected bool
	}{
		{
			desc:        "nil input",
			errExpected: true,
		},
		{
			desc:        "standard input",
			errExpected: false,
			region:      "us-west-2",
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket/session",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket/events",
				AthenaResultsURI:       "s3://transient-storage-bucket/query_results",
				AthenaWorkgroup:        "teleport-workgroup",
				GlueDatabase:           "teleport-database",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:        "invalid input transient and long-term share same bucket name",
			errExpected: true,
			region:      "us-west-2",
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket/session",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket/events",
				AthenaResultsURI:       "s3://long-term-storage-bucket/query_results",
				AthenaWorkgroup:        "teleport-workgroup",
				GlueDatabase:           "teleport-database",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:        "invalid input audit events and session recordings have different URIs",
			errExpected: true,
			region:      "us-west-2",
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket-sessions/session",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket-events/events",
				AthenaResultsURI:       "s3://transient-storage-bucket/query_results",
				AthenaWorkgroup:        "teleport-workgroup",
				GlueDatabase:           "teleport-database",
				GlueTable:              "audit-events",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			testCtx := context.Background()
			s3Clt := &mockBootstrapS3Client{buckets: map[string]struct{}{}}
			athenaClt := &mockBootstrapAthenaClient{}
			glueClt := &mockBootstrapGlueClient{}
			err := externalcloudaudit.BootstrapInfra(testCtx, externalcloudaudit.BootstrapInfraParams{
				Athena: athenaClt,
				Glue:   glueClt,
				S3:     s3Clt,
				Spec:   tc.eca,
				Region: tc.region,
			})
			if tc.errExpected {
				require.Error(t, err, "an error was expected in BootstrapInfra but was not present")
				return
			} else {
				require.NoError(t, err, "an unexpected error occurred in BootstrapInfra")
			}

			ltsBucket, err := url.Parse(tc.eca.AuditEventsLongTermURI)
			require.NoError(t, err)

			transientBucket, err := url.Parse(tc.eca.AthenaResultsURI)
			require.NoError(t, err)

			if _, ok := s3Clt.buckets[ltsBucket.Host]; !ok {
				t.Fatalf("Long-term bucket: %s not created by bootstrap infra", ltsBucket.Host)
			}

			if _, ok := s3Clt.buckets[transientBucket.Host]; !ok {
				t.Fatalf("Transient bucket: %s not created by bootstrap infra", transientBucket.Host)
			}

			require.Equal(t, glueClt.database, tc.eca.GlueDatabase)
			require.Equal(t, glueClt.table, tc.eca.GlueTable)
			require.Equal(t, athenaClt.workgroup, tc.eca.AthenaWorkgroup)

			// Re-run bootstrap
			require.NoError(t, externalcloudaudit.BootstrapInfra(testCtx, externalcloudaudit.BootstrapInfraParams{
				Athena: athenaClt,
				Glue:   glueClt,
				S3:     s3Clt,
				Spec:   tc.eca,
				Region: tc.region,
			}))
		})
	}
}

type mockBootstrapS3Client struct {
	buckets map[string]struct{}
}

type mockBootstrapAthenaClient struct {
	workgroup string
}

type mockBootstrapGlueClient struct {
	table    string
	database string
}

func (c *mockBootstrapS3Client) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; ok {
		// bucket already exists
		return nil, &s3types.BucketAlreadyExists{Message: aws.String("The bucket already exists")}
	}

	c.buckets[*params.Bucket] = struct{}{}

	return &s3.CreateBucketOutput{}, nil
}

func (c *mockBootstrapS3Client) PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't exist")}
	}

	return &s3.PutObjectLockConfigurationOutput{}, nil
}
func (c *mockBootstrapS3Client) PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't exist")}
	}

	return &s3.PutBucketVersioningOutput{}, nil
}

func (c *mockBootstrapS3Client) PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't exist")}
	}

	return &s3.PutBucketLifecycleConfigurationOutput{}, nil
}

func (c *mockBootstrapAthenaClient) CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error) {
	if c.workgroup != "" {
		return nil, &athenatypes.InvalidRequestException{Message: aws.String("workgroup is already created")}
	}

	c.workgroup = *params.Name

	return &athena.CreateWorkGroupOutput{}, nil
}

func (c *mockBootstrapGlueClient) UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error) {
	if c.table == "" {
		return nil, &gluetypes.InvalidInputException{Message: aws.String("the table does not exist")}
	}

	return &glue.UpdateTableOutput{}, nil
}

func (c *mockBootstrapGlueClient) CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error) {
	if c.table != "" {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("table already exists")}
	}

	c.table = *params.TableInput.Name

	return &glue.CreateTableOutput{}, nil
}

// Creates a new database in a Data Catalog.
func (c *mockBootstrapGlueClient) CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error) {
	if c.database != "" {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("database already exists")}
	}

	c.database = *params.DatabaseInput.Name

	return &glue.CreateDatabaseOutput{}, nil
}
