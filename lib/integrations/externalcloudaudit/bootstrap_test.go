package externalcloudaudit

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
	ecatypes "github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			clt := &fakeBootstrapInfraClient{buckets: map[string]struct{}{}}
			testCtx := context.Background()
			err := BootstrapInfra(testCtx, clt, tc.eca, tc.region)
			if tc.errExpected {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			ltsBucket, err := url.Parse(tc.eca.AuditEventsLongTermURI)
			require.NoError(t, err)

			transientBucket, err := url.Parse(tc.eca.AthenaResultsURI)
			require.NoError(t, err)

			if _, ok := clt.buckets[ltsBucket.Host]; !ok {
				require.Fail(t, "long-term bucket: %s not created by bootstrap infra", ltsBucket.Host)
			}

			if _, ok := clt.buckets[transientBucket.Host]; !ok {
				require.Fail(t, "transient bucket: %s not created by bootstrap infra", transientBucket.Host)
			}

			require.Equal(t, clt.database, tc.eca.GlueDatabase)
			require.Equal(t, clt.table, tc.eca.GlueTable)
			require.Equal(t, clt.workgroup, tc.eca.AthenaWorkgroup)

			// Re-run bootstrap
			require.NoError(t, BootstrapInfra(testCtx, clt, tc.eca, tc.region))
		})
	}
}

type fakeBootstrapInfraClient struct {
	buckets   map[string]struct{}
	workgroup string
	table     string
	database  string
}

func (c *fakeBootstrapInfraClient) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; ok {
		// bucket already exists
		return nil, &s3types.BucketAlreadyExists{Message: aws.String("The bucket already exists")}
	}

	c.buckets[*params.Bucket] = struct{}{}

	return &s3.CreateBucketOutput{}, nil
}

func (c *fakeBootstrapInfraClient) PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't not exist")}
	}

	return &s3.PutObjectLockConfigurationOutput{}, nil
}
func (c *fakeBootstrapInfraClient) PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't not exist")}
	}

	return &s3.PutBucketVersioningOutput{}, nil
}

func (c *fakeBootstrapInfraClient) PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	if _, ok := c.buckets[*params.Bucket]; !ok {
		// bucket doesn't exist return no such bucket error
		return nil, &s3types.NoSuchBucket{Message: aws.String("The bucket doesn't not exist")}
	}

	return &s3.PutBucketLifecycleConfigurationOutput{}, nil
}

func (c *fakeBootstrapInfraClient) CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error) {
	if c.workgroup != "" {
		return nil, &athenatypes.InvalidRequestException{Message: aws.String("workgroup is already created")}
	}

	c.workgroup = *params.Name

	return &athena.CreateWorkGroupOutput{}, nil
}

func (c *fakeBootstrapInfraClient) UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error) {
	if c.table == "" {
		return nil, &gluetypes.InvalidInputException{Message: aws.String("the table does not exist")}
	}

	return &glue.UpdateTableOutput{}, nil
}

func (c *fakeBootstrapInfraClient) CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error) {
	if c.table != "" {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("table already exists")}
	}

	c.table = *params.TableInput.Name

	return &glue.CreateTableOutput{}, nil
}

// Creates a new database in a Data Catalog.
func (c *fakeBootstrapInfraClient) CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error) {
	if c.database != "" {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("database already exists")}
	}

	c.database = *params.DatabaseInput.Name

	return &glue.CreateDatabaseOutput{}, nil
}

func TestValidateAndParseS3Input(t *testing.T) {
	tt := []struct {
		desc        string
		eca         *ecatypes.ExternalCloudAuditSpec
		errExpected bool
	}{
		{
			desc:        "nil external cloud audit spec",
			errExpected: true,
		},
		{
			desc:        "unique s3 uris for all three buckets",
			errExpected: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://sessions/prefix",
				AuditEventsLongTermURI: "s3://events/prefix",
				AthenaResultsURI:       "s3://results/query_results",
			},
		},
		{
			desc:        "valid s3 configuration",
			errExpected: false,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://lts/prefix",
				AuditEventsLongTermURI: "s3://lts/prefix",
				AthenaResultsURI:       "s3://results/query_results",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			ltsBucket, transientBucket, err := validateAndParseS3Input(tc.eca)
			if tc.errExpected {
				require.Error(t, err, trace.DebugReport(err))
				return
			} else {
				require.NoError(t, err, trace.DebugReport(err))
			}

			require.Contains(t, tc.eca.AuditEventsLongTermURI, ltsBucket)
			require.Contains(t, tc.eca.AthenaResultsURI, transientBucket)
		})
	}
}
