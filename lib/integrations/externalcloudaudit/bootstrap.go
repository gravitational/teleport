package externalcloudaudit

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"
)

const (
	DatabaseDescription = "Teleport external cloud audit events database for Athena"
)

type EnsureKMSKeyClient interface {
	// CreateKey creates a unique customer managed KMS key
	CreateKey(ctx context.Context, params *kms.CreateKeyInput, optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	// Creates a friendly name for a KMS key.
	CreateAlias(ctx context.Context, params *kms.CreateAliasInput, optFns ...func(*kms.Options)) (*kms.CreateAliasOutput, error)
}

type EnsureKMSKeyRequest struct {
	// An optional alias for the created kms key
	Alias string
}

type EnsureKMSKeyResponse struct {
	// KMSKeyID is ID of created kms key
	KMSKeyID string
}

func EnsureKMSKey(ctx context.Context, kmsClient EnsureKMSKeyClient, req EnsureKMSKeyRequest) (*EnsureKMSKeyResponse, error) {
	createOutput, err := kmsClient.CreateKey(ctx, &kms.CreateKeyInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Alias != "" {
		_, err = kmsClient.CreateAlias(ctx, &kms.CreateAliasInput{
			AliasName:   aws.String(req.Alias),
			TargetKeyId: createOutput.KeyMetadata.KeyId,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &EnsureKMSKeyResponse{
		KMSKeyID: *createOutput.KeyMetadata.KeyId,
	}, nil
}

// EnsureS3BucketClient describes the required methods to ensure s3 buckets for external cloud audit
type EnsureS3BucketClient interface {
	// CreateBucket creates a new S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	// PutBucketEncryption configures encryption and Amazon S3 Bucket Keys for an existing bucket.
	PutBucketEncryption(ctx context.Context, params *s3.PutBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.PutBucketEncryptionOutput, error)
}

// EnsureS3BucketRequest are the request parameters for creating the external cloud audit s3 buckets
type EnsureS3BucketRequest struct {
	BucketName string
	KMSKeyID   string
}

// EnsureS3Bucket for external cloud audit S3 buckets with recommended configurations
func EnsureS3Bucket(ctx context.Context, s3Client EnsureS3BucketClient, req EnsureS3BucketRequest) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:          aws.String(req.BucketName),
		ObjectOwnership: s3types.ObjectOwnershipBucketOwnerEnforced,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// If KMSKeyID isn't specified the bucket defaults to SSE-S3
	if req.KMSKeyID != "" {
		_, err = s3Client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
			Bucket: aws.String(req.BucketName),
			ServerSideEncryptionConfiguration: &s3types.ServerSideEncryptionConfiguration{
				Rules: []s3types.ServerSideEncryptionRule{
					{
						BucketKeyEnabled: true,
						ApplyServerSideEncryptionByDefault: &s3types.ServerSideEncryptionByDefault{
							SSEAlgorithm:   s3types.ServerSideEncryptionAwsKms,
							KMSMasterKeyID: aws.String(req.KMSKeyID),
						},
					},
				},
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// EnsureAthenaWorkgroupClient describes the required methods to ensure and configure athena workgroups for external cloud audit
type EnsureAthenaWorkgroupClient interface {
	// Creates a workgroup with the specified name.
	CreateWorkGroup(ctx context.Context, params *athena.CreateWorkGroupInput, optFns ...func(*athena.Options)) (*athena.CreateWorkGroupOutput, error)
}

// EnsureAthenaWorkgroupRequest are the request parameters for creating the external cloud audit s3 buckets
type EnsureAthenaWorkgroupRequest struct {
	Name string
}

// EnsureAthenaWorkgroup ensures an athena workgroup in which to run the athena sql queries in
func EnsureAthenaWorkgroup(ctx context.Context, athenaClient EnsureAthenaWorkgroupClient, req EnsureAthenaWorkgroupRequest) error {
	_, err := athenaClient.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name:          aws.String(req.Name),
		Configuration: &athenatypes.WorkGroupConfiguration{},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type EnsureGlueInfraClient interface {
	// Creates a new database in a Data Catalog.
	CreateDatabase(ctx context.Context, params *glue.CreateDatabaseInput, optFns ...func(*glue.Options)) (*glue.CreateDatabaseOutput, error)
	// Creates a new table definition in the Data Catalog
	CreateTable(ctx context.Context, params *glue.CreateTableInput, optFns ...func(*glue.Options)) (*glue.CreateTableOutput, error)
}

type EnsureGlueInfraRequest struct {
	DatabaseName string
	TableName    string
	EventBucket  string
}

// EnsureGlueInfra ensures necessary infrastructure exists, creating it if it doesn't
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_awsglue.html
// Required IAM Permissions:
// * CreateDatabase
// * CreateTable
func EnsureGlueInfra(ctx context.Context, glueClient EnsureGlueInfraClient, req EnsureGlueInfraRequest) error {
	_, err := glueClient.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &gluetypes.DatabaseInput{
			Name:        aws.String(req.DatabaseName),
			Description: aws.String(DatabaseDescription),
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	location := fmt.Sprintf("s3://%s", req.EventBucket)

	// Currently matches table input as specified in:
	// https://github.com/gravitational/cloud/blob/22393dcc9362ec77b0a111c3cc81b65df19da0b0/pkg/tenantcontroller/athena.go#L458-L504
	// TODO: Consolidate source of truth to a single location. Preferably teleport repository
	_, err = glueClient.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String(req.DatabaseName),
		TableInput: &gluetypes.TableInput{
			Name: aws.String(req.TableName),
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
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
