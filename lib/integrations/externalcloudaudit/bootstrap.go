package externalcloudaudit

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/gravitational/trace"
)


const (
	DatabaseDescription = "Teleport external cloud audit events database for Athena"
)

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
