package externalcloudaudit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
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
		desc            string
		region          string
		eca             *ecatypes.ExternalCloudAuditSpec
		bucketExists    bool
		tableExists     bool
		databaseExists  bool
		workgroupExists bool
		errExpected     bool
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
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:         "standard input bucket exists",
			errExpected:  false,
			region:       "us-west-2",
			bucketExists: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:        "standard input table exists",
			errExpected: false,
			region:      "us-west-2",
			tableExists: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:         "standard input database exists",
			errExpected:  false,
			region:       "us-west-2",
			bucketExists: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:            "standard input workgroup exists",
			errExpected:     false,
			region:          "us-west-2",
			workgroupExists: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
		{
			desc:            "standard input all exist",
			errExpected:     false,
			region:          "us-west-2",
			bucketExists:    true,
			databaseExists:  true,
			tableExists:     true,
			workgroupExists: true,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://long-term-storage-bucket",
				AuditEventsLongTermURI: "s3://long-term-storage-bucket",
				AthenaResultsURI:       "s3://transient-storage-bucket",
				AthenaWorkgroup:        "teleport",
				GlueDatabase:           "teleport",
				GlueTable:              "audit-events",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			clt := &fakeBootstrapInfraClient{
				bucketExists:    tc.bucketExists,
				tableExists:     tc.tableExists,
				databaseExists:  tc.databaseExists,
				workgroupExists: tc.workgroupExists,
			}
			err := BootstrapInfra(context.Background(), clt, tc.eca, tc.region)
			if tc.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type fakeBootstrapInfraClient struct {
	bucketExists    bool
	workgroupExists bool
	databaseExists  bool
	tableExists     bool
}

func (c *fakeBootstrapInfraClient) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if c.bucketExists {
		return nil, &s3types.BucketAlreadyExists{Message: aws.String("The bucket already exists")}
	}

	return &s3.CreateBucketOutput{}, nil
}

func (c *fakeBootstrapInfraClient) PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error) {
	return &s3.PutObjectLockConfigurationOutput{}, nil
}
func (c *fakeBootstrapInfraClient) PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	return &s3.PutBucketVersioningOutput{}, nil
}

func (c *fakeBootstrapInfraClient) PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	return &s3.PutBucketLifecycleConfigurationOutput{}, nil
}

func (c *fakeBootstrapInfraClient) CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error) {
	return &athena.CreateWorkGroupOutput{}, nil
}

func (c *fakeBootstrapInfraClient) UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error) {
	return &glue.UpdateTableOutput{}, nil
}

func (c *fakeBootstrapInfraClient) CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error) {
	if c.tableExists {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("table already exists")}
	}
	return &glue.CreateTableOutput{}, nil
}

// Creates a new database in a Data Catalog.
func (c *fakeBootstrapInfraClient) CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error) {
	if c.databaseExists {
		return nil, &gluetypes.AlreadyExistsException{Message: aws.String("database already exists")}
	}
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
				AthenaResultsURI:       "s3://results/",
			},
		},
		{
			desc:        "valid s3 configuration",
			errExpected: false,
			eca: &ecatypes.ExternalCloudAuditSpec{
				SessionsRecordingsURI:  "s3://lts/prefix",
				AuditEventsLongTermURI: "s3://lts/prefix",
				AthenaResultsURI:       "s3://results/",
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
