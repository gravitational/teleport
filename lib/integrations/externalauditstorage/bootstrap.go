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

package externalauditstorage

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

	eastypes "github.com/gravitational/teleport/api/types/externalauditstorage"
	awsutil "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// defaultObjectLockRetentionYears defines default object lock retention period in years for long-term s3 bucket.
	defaultObjectLockRetentionYears = 4
	// glueDatabaseDescription is the description of the glue database created by bootstrapping.
	glueDatabaseDescription = "Teleport External Audit Storage events database for Athena"
)

// BootstrapInfraParams are the input parameters for [BootstrapInfra].
type BootstrapInfraParams struct {
	Athena BootstrapAthenaClient
	Glue   BootstrapGlueClient
	S3     BootstrapS3Client

	Spec   *eastypes.ExternalAuditStorageSpec
	Region string
}

// BootstrapAthenaClient is a subset of [athena.Client] methods needed for athena bootstrap.
type BootstrapAthenaClient interface {
	// Creates a workgroup with the specified name.
	CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error)
}

// BootstrapGlueClient is a subset of [glue.Client] methods needed for glue boostrap.
type BootstrapGlueClient interface {
	// Creates a new database in a Data Catalog.
	CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error)
	// Creates a new table definition in the Data Catalog.
	CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error)
	// Updates a metadata table in the Data Catalog.
	UpdateTable(ctx context.Context, params *glue.UpdateTableInput, optFns ...func(*glue.Options)) (*glue.UpdateTableOutput, error)
}

// BootstrapS3Client is a subset of [s3.Client] methods needed to bootstrap S3 buckets.
type BootstrapS3Client interface {
	// Creates a new S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	// Places an Object Lock configuration on the specified bucket.
	PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error)
	// Sets the versioning state of an existing bucket.
	PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error)
	// Creates a new lifecycle configuration for the bucket or replaces an existing lifecycle configuration.
	PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error)
}

// BootstrapInfra bootstraps External Audit Storage infrastructure.
// We are currently very opinionated about inputs and have additional checks to ensure
// a stricter setup is created.
func BootstrapInfra(ctx context.Context, params BootstrapInfraParams) error {
	fmt.Println("\nBootstrapping External Audit Storage infrastructure")

	switch {
	case params.Athena == nil:
		return trace.BadParameter("param Athena required")
	case params.Glue == nil:
		return trace.BadParameter("param Glue required")
	case params.S3 == nil:
		return trace.BadParameter("param S3 required")
	case params.Region == "":
		return trace.BadParameter("param Region required")
	case params.Spec == nil:
		return trace.BadParameter("param Spec required")
	}

	ltsBucket, transientBucket, err := validateAndParseS3Input(params.Spec)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := createLTSBucket(ctx, params.S3, ltsBucket, params.Region); err != nil {
		return trace.Wrap(err)
	}

	if err := createTransientBucket(ctx, params.S3, transientBucket, params.Region); err != nil {
		return trace.Wrap(err)
	}

	if err := createAthenaWorkgroup(ctx, params.Athena, params.Spec.AthenaWorkgroup); err != nil {
		return trace.Wrap(err)
	}

	if err := createGlueInfra(ctx, params.Glue, params.Spec.GlueTable, params.Spec.GlueDatabase, ltsBucket); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Creates a bucket with the given name in the given region.
// Long Term Storage buckets have the following properties:
// * Bucket versioning enabled
// * Object locking enabled with Governance mode and default retention of 4 years
// * Object ownership set to BucketOwnerEnforced
// * Default SSE-S3 encryption
func createLTSBucket(ctx context.Context, clt BootstrapS3Client, bucketName string, region string) error {
	fmt.Printf("Creating long term storage S3 bucket %s\n", bucketName)
	err := createBucket(ctx, clt, bucketName, region, true)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "creating long term storage S3 bucket")
	}

	fmt.Printf("Applying object lock configuration to long term storage S3 bucket with default retention period of %d years\n", defaultObjectLockRetentionYears)
	_, err = clt.PutObjectLockConfiguration(ctx, &s3.PutObjectLockConfigurationInput{
		Bucket: &bucketName,
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
			ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
			Rule: &s3types.ObjectLockRule{
				DefaultRetention: &s3types.DefaultRetention{
					Years: aws.Int32(defaultObjectLockRetentionYears),
					// Modification is prohibited without IAM S3:BypassGovernancePermission
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
func createTransientBucket(ctx context.Context, clt BootstrapS3Client, bucketName string, region string) error {
	fmt.Printf("Creating transient storage S3 bucket %s\n", bucketName)
	err := createBucket(ctx, clt, bucketName, region, false)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "creating transient S3 bucket")
	}

	fmt.Println("Applying bucket lifecycle configuration to transient storage S3 bucket")
	_, err = clt.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: &bucketName,
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{
			Rules: []s3types.LifecycleRule{
				{
					Status: s3types.ExpirationStatusEnabled,
					ID:     aws.String("ExpireQueryResults"),
					Expiration: &s3types.LifecycleExpiration{
						Days: aws.Int32(1),
					},
					Filter: &s3types.LifecycleRuleFilterMemberPrefix{
						Value: "/query_results",
					},
				},
				{
					Status: s3types.ExpirationStatusEnabled,
					ID:     aws.String("ExpireNonCurrentVersionsAndDeleteMarkers"),
					NoncurrentVersionExpiration: &s3types.NoncurrentVersionExpiration{
						NoncurrentDays: aws.Int32(1),
					},
					AbortIncompleteMultipartUpload: &s3types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: aws.Int32(7),
					},
					Expiration: &s3types.LifecycleExpiration{
						ExpiredObjectDeleteMarker: aws.Bool(true),
					},
					Filter: &s3types.LifecycleRuleFilterMemberPrefix{},
				},
			},
		},
	})
	return trace.Wrap(awsutil.ConvertS3Error(err), "setting lifecycle configuration on S3 bucket")
}

func createBucket(ctx context.Context, clt BootstrapS3Client, bucketName string, region string, objectLock bool) error {
	_, err := clt.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                     &bucketName,
		CreateBucketConfiguration:  awsutil.CreateBucketConfiguration(region),
		ObjectLockEnabledForBucket: &objectLock,
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

// createAthenaWorkgroup creates an athena workgroup in which to run athena sql queries.
func createAthenaWorkgroup(ctx context.Context, clt BootstrapAthenaClient, workgroup string) error {
	fmt.Printf("Creating Athena workgroup %s\n", workgroup)
	_, err := clt.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name:          &workgroup,
		Configuration: &athenatypes.WorkGroupConfiguration{},
	})
	if err != nil && !strings.Contains(err.Error(), "is already created") {
		return trace.Wrap(err, "creating Athena workgroup")
	}

	return nil
}

// createGlueInfra creates necessary infrastructure for glue operations.
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_awsglue.html
// Required IAM Permissions:
// * CreateDatabase
// * CreateTable
// * UpdateTable
func createGlueInfra(ctx context.Context, clt BootstrapGlueClient, table, database, eventBucket string) error {
	fmt.Printf("Creating Glue database %s\n", database)
	_, err := clt.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &gluetypes.DatabaseInput{
			Name:        &database,
			Description: aws.String(glueDatabaseDescription),
		},
	})
	if err != nil {
		var aee *gluetypes.AlreadyExistsException
		if !errors.As(err, &aee) {
			return trace.Wrap(err, "creating Glue database")
		}
	}

	// Currently matches table input as specified in:
	// https://github.com/gravitational/cloud/blob/22393dcc9362ec77b0a111c3cc81b65df19da0b0/pkg/tenantcontroller/athena.go#L458-L504
	// TODO(logand22): Consolidate source of truth to a single location. Preferably teleport repository.
	// We do want to ensure that the table that exists has the correct table input so we'll update already existing tables.
	fmt.Printf("Creating Glue table %s\n", table)
	_, err = clt.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: &database,
		TableInput:   getGlueTableInput(table, eventBucket),
	})
	if err != nil {
		var aee *gluetypes.AlreadyExistsException
		if !errors.As(err, &aee) {
			return trace.Wrap(err, "creating Glue table")
		}

		_, err = clt.UpdateTable(ctx, &glue.UpdateTableInput{
			DatabaseName: &database,
			TableInput:   getGlueTableInput(table, eventBucket),
		})
		if err != nil {
			return trace.Wrap(err, "updating Glue table")
		}
	}

	return nil
}

// validateAndParseS3Input parses and checks s3 input uris against our strict rules.
// We currently enforce two buckets one for long term storage and one for transient short term storage.
func validateAndParseS3Input(input *eastypes.ExternalAuditStorageSpec) (auditHost, resultHost string, err error) {
	auditEventsBucket, err := url.Parse(input.AuditEventsLongTermURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing audit events URI")
	}
	if auditEventsBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("invalid scheme for audit events bucket URI")
	}
	auditHost = auditEventsBucket.Host

	sessionBucket, err := url.Parse(input.SessionRecordingsURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing session recordings URI")
	}
	if sessionBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("invalid scheme for session bucket URI")
	}
	sessionHost := sessionBucket.Host

	resultBucket, err := url.Parse(input.AthenaResultsURI)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing athena results URI")
	}
	if resultBucket.Scheme != "s3" {
		return "", "", trace.BadParameter("invalid scheme for athena results bucket URI")
	}
	resultHost = resultBucket.Host

	if auditHost != sessionHost {
		return "", "", trace.BadParameter("audit events bucket URI must match session bucket URI")
	}

	if resultHost == auditHost {
		return "", "", trace.BadParameter("athena results bucket URI must not match audit events or session bucket URI")
	}

	return auditHost, resultHost, nil
}

// getGlueTableInput returns glue table input for both creating and updating a glue table.
func getGlueTableInput(table string, eventBucket string) *gluetypes.TableInput {
	location := fmt.Sprintf("s3://%s/events", eventBucket)

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
