package externalcloudaudit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"

	ecatypes "github.com/gravitational/teleport/api/types/externalcloudaudit"
	awsutil "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// defaultRetentionYears defines default object lock retention period
	defaultRetentionYears = 4
	// databaseDescription is the description of the glue database
	databaseDescription = "Teleport external cloud audit events database for Athena"
)

// BootstrapInfraClient describes the methods required to create bootstrap the
// external cloud audit infrastructure
type BootstrapInfraClient interface {
	createBucketClient
	createAthenaWorkgroupClient
	createGlueInfraClient
}

// DefaultDefaultBootstrapInfraClient wraps an glue, s3 and athena client to create a BootstrapInfraClient
type DefaultBootstrapInfraClient struct {
	Glue   *glue.Client
	S3     *s3.Client
	Athena *athena.Client
}

func (d *DefaultBootstrapInfraClient) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	return d.S3.CreateBucket(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error) {
	return d.S3.PutObjectLockConfiguration(ctx, params, optFns...)
}
func (d *DefaultBootstrapInfraClient) PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	return d.S3.PutBucketVersioning(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	return d.S3.PutBucketLifecycleConfiguration(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error) {
	return d.Athena.CreateWorkGroup(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error) {
	return d.Glue.CreateDatabase(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error) {
	return d.Glue.CreateTable(ctx, params, optFns...)
}

func (d *DefaultBootstrapInfraClient) UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error) {
	return d.Glue.UpdateTable(ctx, params, optFns...)
}

// BootstrapInfra bootstraps external cloud audit infrastructure
// We are currently very opinionated about inputs and have additional checks
func BootstrapInfra(ctx context.Context, clt BootstrapInfraClient, eca *ecatypes.ExternalCloudAuditSpec, region string) error {
	ltsBucket, transientBucket, err := validateAndParseS3Input(eca)
	if err != nil {
		return trace.Wrap(err)
	}

	err = createLTSBucket(ctx, clt, ltsBucket, region)
	if err != nil {
		return trace.Wrap(err)
	}

	err = createTransientBucket(ctx, clt, transientBucket, region)
	if err != nil {
		return trace.Wrap(err)
	}

	err = createAthenaWorkgroup(ctx, clt, eca.AthenaWorkgroup)
	if err != nil {
		return trace.Wrap(err)
	}

	err = createGlueInfra(ctx, clt, eca.GlueTable, eca.GlueDatabase, ltsBucket)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// createBucketClient contains methods needed to create a new  bucket
type createBucketClient interface {
	// Creates a new S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	// Places an Object Lock configuration on the specified bucket.
	PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error)
	// Sets the versioning state of an existing bucket.
	PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error)
	// Creates a new lifecycle configuration for the bucket or replaces an existing lifecycle configuration.
	PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error)
}

// Creates a bucket with the given name in the given region.
// Long Term Storage buckets have the following properties:
// * Bucket versioning enabled
// * Object locking enabled with Governance mode and default retention of 4 years
// * Object ownership set to BucketOwnerEnforced
// * Default SSE-S3 encryption
func createLTSBucket(ctx context.Context, clt createBucketClient, bucketName string, region string) error {
	err := createBucket(ctx, clt, bucketName, region, true)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "creating long-term S3 bucket")
		}
	}

	_, err = clt.PutObjectLockConfiguration(ctx, &s3.PutObjectLockConfigurationInput{
		Bucket: &bucketName,
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
			ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
			Rule: &s3types.ObjectLockRule{
				DefaultRetention: &s3types.DefaultRetention{
					Years: defaultRetentionYears,
					// Modification is prohibited without BypassGovernancePermission iam permission
					Mode: s3types.ObjectLockRetentionModeGovernance,
				},
			},
		},
	})
	return trace.Wrap(awsutil.ConvertS3Error(err), "setting object lock default retention on S3 bucket")
}

// createTransientBucket is similar to createLTSBucket however object locking is not enabled and instead a lifecycle
// policy is created that cleans up transient storage:
// * Query results expire after 1 day
// * DeleteMarkers, NonCurrentVersions and IncompleteMultipartUploads are also removed
func createTransientBucket(ctx context.Context, clt createBucketClient, bucketName string, region string) error {
	err := createBucket(ctx, clt, bucketName, region, false)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "creating transient S3 bucket")
		}
	}

	_, err = clt.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: &bucketName,
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{
			Rules: []s3types.LifecycleRule{
				{
					Status: s3types.ExpirationStatusEnabled,
					ID:     aws.String("ExpireQueryResults"),
					Expiration: &s3types.LifecycleExpiration{
						Days: 1,
					},
					Filter: &s3types.LifecycleRuleFilterMemberPrefix{
						Value: "/query_results",
					},
				},
				{
					Status: s3types.ExpirationStatusEnabled,
					ID:     aws.String("ExpireNonCurrentVersionsAndDeleteMarkers"),
					NoncurrentVersionExpiration: &s3types.NoncurrentVersionExpiration{
						NewerNoncurrentVersions: 0,
						NoncurrentDays:          1,
					},
					AbortIncompleteMultipartUpload: &s3types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: 7,
					},
					Expiration: &s3types.LifecycleExpiration{
						ExpiredObjectDeleteMarker: true,
					},
					Filter: &s3types.LifecycleRuleFilterMemberPrefix{},
				},
			},
		},
	})
	return trace.Wrap(awsutil.ConvertS3Error(err), "setting lifecycle configuration on S3 bucket")
}

func createBucket(ctx context.Context, clt createBucketClient, bucketName string, region string, objectLock bool) error {
	_, err := clt.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucketName,
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		},
		ObjectLockEnabledForBucket: objectLock,
		ACL:                        s3types.BucketCannedACLPrivate,
		ObjectOwnership:            s3types.ObjectOwnershipBucketOwnerEnforced,
	})
	if err != nil {
		return trace.Wrap(awsutil.ConvertS3Error(err))
	}

	_, err = clt.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: &bucketName,
		VersioningConfiguration: &s3types.VersioningConfiguration{
			Status: s3types.BucketVersioningStatusEnabled,
		},
	})
	return trace.Wrap(awsutil.ConvertS3Error(err), "setting versioning configuration on S3 bucket")
}

// createAthenaWorkgroupClient describes the required methods to ensure and configure athena workgroups for external cloud audit
type createAthenaWorkgroupClient interface {
	// Creates a workgroup with the specified name.
	CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error)
}

// createAthenaWorkgroup creates an athena workgroup in which to run the athena sql queries
func createAthenaWorkgroup(ctx context.Context, clt createAthenaWorkgroupClient, workgroup string) error {
	_, err := clt.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name:          &workgroup,
		Configuration: &athenatypes.WorkGroupConfiguration{},
	})
	if err != nil {
		if !strings.Contains(err.Error(), "is already created") {
			return trace.Wrap(err, "creating athena workgroup")
		}
	}

	return nil
}

type createGlueInfraClient interface {
	// Creates a new database in a Data Catalog.
	CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error)
	// Creates a new table definition in the Data Catalog
	CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error)
	// Updates a metadata table in the Data Catalog.
	UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error)
}

// createGlueInfra creates necessary infrastructure for glue operations
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_awsglue.html
// Required IAM Permissions:
// * CreateDatabase
// * CreateTable
// * UpdateTable
func createGlueInfra(ctx context.Context, clt createGlueInfraClient, table, database, eventBucket string) error {
	_, err := clt.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &gluetypes.DatabaseInput{
			Name:        &database,
			Description: aws.String(databaseDescription),
		},
	})
	if err != nil {
		var aee *gluetypes.AlreadyExistsException
		if !errors.As(err, &aee) {
			return trace.Wrap(err, "creating glue database")
		}
	}

	// Currently matches table input as specified in:
	// https://github.com/gravitational/cloud/blob/22393dcc9362ec77b0a111c3cc81b65df19da0b0/pkg/tenantcontroller/athena.go#L458-L504
	// TODO: Consolidate source of truth to a single location. Preferably teleport repository
	// We do want to ensure that the table that exists has the correct table input so we'll update already existing tables
	_, err = clt.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: &database,
		TableInput:   getGlueTableInput(table, eventBucket),
	})
	if err != nil {
		var aee *gluetypes.AlreadyExistsException
		if !errors.As(err, &aee) {
			return trace.Wrap(err, "creating glue table")
		}

		_, err = clt.UpdateTable(ctx, &glue.UpdateTableInput{
			DatabaseName: &database,
			TableInput:   getGlueTableInput(table, eventBucket),
		})
		if err != nil {
			return trace.Wrap(err, "updating glue table")
		}
	}

	return nil
}

// validateAndParseS3Input parses and checks s3 input uris against are strict rules
// We currently enforce two buckets one for long term storage and one for transient short term storage
func validateAndParseS3Input(input *ecatypes.ExternalCloudAuditSpec) (string, string, error) {
	if input == nil {
		return "", "", trace.BadParameter("input is nil")
	}

	auditEventsBucket, err := url.Parse(input.AuditEventsLongTermURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing audit events URI")
	}
	if auditEventsBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("Invalid scheme for audit events bucket URI")
	}

	sessionBucket, err := url.Parse(input.SessionsRecordingsURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing session recordings URI")
	}
	if sessionBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("Invalid scheme for session bucket URI")
	}

	resultBucket, err := url.Parse(input.AthenaResultsURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing athena results URI")
	}
	if resultBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("Invalid scheme for athena results bucket URI")
	}

	if auditEventsBucket.Host != sessionBucket.Host {
		return "", "", trace.BadParameter("Audit events bucket URI must match session bucket URI")
	}

	if resultBucket.Host == auditEventsBucket.Host {
		return "", "", trace.BadParameter("Athena results bucket URI must not match audit events or session bucket URI")
	}

	return auditEventsBucket.Host, resultBucket.Host, nil
}

// getGlueTableInput returns glue table input for both creating and updating a glue table
func getGlueTableInput(table string, eventBucket string) *gluetypes.TableInput {
	location := fmt.Sprintf("s3://%s", eventBucket)

	return &gluetypes.TableInput{
		Name: &table,
		StorageDescriptor: &gluetypes.StorageDescriptor{
			Columns: []gluetypes.Column{
				{Name: aws.String("uid"), Type: aws.String("string")},
				{Name: aws.String("session_id"), Type: aws.String("string")},
				{Name: aws.String("event_type"), Type: aws.String("string")},
				{Name: aws.String("event_time"), Type: aws.String("timestamp")},
				{Name: aws.String("event_data"), Type: aws.String("string")},
				{Name: aws.String("user"), Type: aws.String("string")},
			},
			Compressed:      false,
			NumberOfBuckets: 0,
			Location:        aws.String(location),
			InputFormat:     aws.String("org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat"),
			OutputFormat:    aws.String("org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat"),
			SerdeInfo: &gluetypes.SerDeInfo{
				SerializationLibrary: aws.String("org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"),
				Parameters:           map[string]string{"serialization.format": "1"},
			},
			StoredAsSubDirectories: false,
		},
		PartitionKeys: []gluetypes.Column{{
			Name: aws.String("event_date"), Type: aws.String("date"),
		}},
		TableType: aws.String("EXTERNAL_TABLE"),
		Parameters: map[string]string{
			"EXTERNAL":                            "TRUE",
			"projection.event_date.type":          "date",
			"projection.event_date.format":        "yyyy-MM-dd",
			"projection.event_date.interval":      "1",
			"projection.event_date.interval.unit": "DAYS",
			"projection.event_date.range":         "NOW-4YEARS,NOW",
			"storage.location.template":           location + "/${event_date}/",
			"classification":                      "parquet",
			"parquet.compression":                 "SNAPPY",
			"projection.enabled":                  "true",
		},
		Retention: 0,
	}
}
