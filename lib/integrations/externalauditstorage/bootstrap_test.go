/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package externalauditstorage_test

import (
	"context"
	"maps"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eastypes "github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage"
)

func TestBootstrapInfra(t *testing.T) {
	t.Parallel()
	goodSpec := &eastypes.ExternalAuditStorageSpec{
		SessionRecordingsURI:   "s3://long-term-storage-bucket/session",
		AuditEventsLongTermURI: "s3://long-term-storage-bucket/events",
		AthenaResultsURI:       "s3://transient-storage-bucket/query_results",
		AthenaWorkgroup:        "teleport-workgroup",
		GlueDatabase:           "teleport-database",
		GlueTable:              "audit-events",
	}
	tt := []struct {
		desc                     string
		region                   string
		spec                     *eastypes.ExternalAuditStorageSpec
		errWanted                string
		locationConstraintWanted s3types.BucketLocationConstraint
	}{
		{
			desc:      "nil input",
			region:    "us-west-2",
			errWanted: "param Spec required",
		},
		{
			desc:      "empty region input",
			errWanted: "param Region required",
			spec:      goodSpec,
		},
		{
			desc:                     "us-west-2",
			region:                   "us-west-2",
			spec:                     goodSpec,
			locationConstraintWanted: s3types.BucketLocationConstraintUsWest2,
		},
		{
			desc:   "us-east-1",
			region: "us-east-1",
			spec:   goodSpec,
			// No location constraint wanted for us-east-1 because it is the
			// default and AWS has decided, in all their infinite wisdom, that
			// the CreateBucket API should fail if you explicitly pass the
			// default location constraint.
		},
		{
			desc:                     "eu-central-1",
			region:                   "eu-central-1",
			spec:                     goodSpec,
			locationConstraintWanted: s3types.BucketLocationConstraintEuCentral1,
		},
		{
			desc:                     "ap-south-1",
			region:                   "ap-south-1",
			spec:                     goodSpec,
			locationConstraintWanted: s3types.BucketLocationConstraintApSouth1,
		},
		{
			desc:                     "ap-southeast-1",
			region:                   "ap-southeast-1",
			spec:                     goodSpec,
			locationConstraintWanted: s3types.BucketLocationConstraintApSoutheast1,
		},
		{
			desc:                     "sa-east-1",
			region:                   "sa-east-1",
			spec:                     goodSpec,
			locationConstraintWanted: s3types.BucketLocationConstraintSaEast1,
		},
		{
			desc:      "invalid input transient and long-term share same bucket name",
			errWanted: "athena results bucket URI must not match audit events or session bucket URI",
			region:    "us-west-2",
			spec: &eastypes.ExternalAuditStorageSpec{
				SessionRecordingsURI:   "s3://long-term-storage-bucket/session",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket/events",
				AthenaResultsURI:       "s3://long-term-storage-bucket/query_results",
				AthenaWorkgroup:        "teleport-workgroup",
				GlueDatabase:           "teleport-database",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:      "invalid input audit events and session recordings have different URIs",
			errWanted: "audit events bucket URI must match session bucket URI",
			region:    "us-west-2",
			spec: &eastypes.ExternalAuditStorageSpec{
				SessionRecordingsURI:   "s3://long-term-storage-bucket-sessions/session",
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
			s3Clt := &mockBootstrapS3Client{buckets: make(map[string]bucket)}
			athenaClt := &mockBootstrapAthenaClient{}
			glueClt := &mockBootstrapGlueClient{}
			err := externalauditstorage.BootstrapInfra(testCtx, externalauditstorage.BootstrapInfraParams{
				Athena:          athenaClt,
				Glue:            glueClt,
				S3:              s3Clt,
				Spec:            tc.spec,
				Region:          tc.region,
				ClusterName:     "my-cluster",
				IntegrationName: "my-integration",
			})
			if tc.errWanted != "" {
				require.ErrorContainsf(t, err, tc.errWanted, "the error returned did not contain: %s", tc.errWanted)
				return
			} else {
				require.NoError(t, err, "an unexpected error occurred in BootstrapInfra")
			}

			ltsBucket, err := url.Parse(tc.spec.AuditEventsLongTermURI)
			require.NoError(t, err)

			transientBucket, err := url.Parse(tc.spec.AthenaResultsURI)
			require.NoError(t, err)

			if b, ok := s3Clt.buckets[ltsBucket.Host]; ok {
				assert.Equal(t, tc.locationConstraintWanted, b.locationConstraint)
			} else {
				t.Fatalf("Long-term bucket: %s not created by bootstrap infra", ltsBucket.Host)
			}

			if b, ok := s3Clt.buckets[transientBucket.Host]; ok {
				assert.Equal(t, tc.locationConstraintWanted, b.locationConstraint)
			} else {
				t.Fatalf("Transient bucket: %s not created by bootstrap infra", transientBucket.Host)
			}

			assert.Equal(t, tc.spec.GlueDatabase, glueClt.database)
			assert.Equal(t, tc.spec.GlueTable, glueClt.table)
			assert.Equal(t, tc.spec.AthenaWorkgroup, athenaClt.workgroup)

			// Re-run bootstrap
			assert.NoError(t, externalauditstorage.BootstrapInfra(testCtx, externalauditstorage.BootstrapInfraParams{
				Athena:          athenaClt,
				Glue:            glueClt,
				S3:              s3Clt,
				Spec:            tc.spec,
				Region:          tc.region,
				ClusterName:     "my-cluster",
				IntegrationName: "my-integration",
			}))

			// Enrure ownership tags were set on all resources.
			// S3 Buckets
			for bucketName, bucket := range s3Clt.buckets {
				require.ElementsMatch(t,
					[]s3types.Tag{
						{Key: aws.String("teleport.dev/cluster"), Value: aws.String("my-cluster")},
						{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
						{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
					},
					bucket.tags,
					"tags in bucket=%s do not match the ownership tags", bucketName)
			}
			// Athena Workgroup
			require.ElementsMatch(t,
				[]athenatypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("my-cluster")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
				},
				athenaClt.tags,
			)
			// Glue Database
			require.Equal(t,
				map[string]string{
					"teleport.dev/cluster":     "my-cluster",
					"teleport.dev/origin":      "integration_awsoidc",
					"teleport.dev/integration": "my-integration",
				},
				glueClt.databaseTags,
			)
		})
	}
}

type mockBootstrapS3Client struct {
	buckets map[string]bucket
}

type bucket struct {
	locationConstraint s3types.BucketLocationConstraint
	tags               []s3types.Tag
}

type mockBootstrapAthenaClient struct {
	workgroup string
	tags      []athenatypes.Tag
}

type mockBootstrapGlueClient struct {
	table        string
	database     string
	databaseTags map[string]string
}

func (c *mockBootstrapS3Client) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; ok {
		// bucket already exists
		return nil, &s3types.BucketAlreadyExists{Message: aws.String("The bucket already exists")}
	}

	var locationConstraint s3types.BucketLocationConstraint
	if params.CreateBucketConfiguration != nil {
		locationConstraint = params.CreateBucketConfiguration.LocationConstraint
	}
	c.buckets[*params.Bucket] = bucket{
		locationConstraint: locationConstraint,
	}

	return &s3.CreateBucketOutput{}, nil
}

func (c *mockBootstrapS3Client) PutBucketTagging(ctx context.Context, params *s3.PutBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.PutBucketTaggingOutput, error) {
	bucket, ok := c.buckets[*params.Bucket]
	if !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't exist")}
	}

	bucket.tags = params.Tagging.TagSet
	c.buckets[*params.Bucket] = bucket

	return &s3.PutBucketTaggingOutput{}, nil
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
	c.tags = params.Tags

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
	c.databaseTags = maps.Clone(params.Tags)

	return &glue.CreateDatabaseOutput{}, nil
}
