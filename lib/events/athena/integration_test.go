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

package athena

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	glueTypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/observability/tracing"
)

func TestIntegrationAthenaSearchSessionEventsBySessionID(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaSearchSessionEventsBySessionID(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaSearchSessionEventsBySessionID(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaSearchSessionEventsBySessionID(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:                                  auditLogger,
		Clock:                                ac.clock,
		SearchSessionEvensBySessionIDTimeout: ac.batcherInterval + 10*time.Second,
	}

	eventsSuite.SearchSessionEventsBySessionID(t)
}

func TestIntegrationAthenaSessionEventsCRUD(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaSessionEventsCRUD(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaSessionEventsCRUD(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaSessionEventsCRUD(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.SessionEventsCRUD(t)
}

func TestIntegrationAthenaEventExport(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaEventExport(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaEventExport(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaEventExport(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.EventExport(t)
}

func TestIntegrationAthenaEventPagination(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaEventPagination(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaEventPagination(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaEventPagination(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.EventPagination(t)
}

func TestIntegrationAthenaLargeEvents(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaLargeEvents(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaLargeEvents(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaLargeEvents(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{
		MaxBatchSize: 1,
		BypassSNS:    bypassSNS,
	})
	in := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Index: 2,
			Type:  events.SessionStartEvent,
			ID:    uuid.NewString(),
			Code:  strings.Repeat("d", 200000),
			Time:  ac.clock.Now().UTC(),
		},
	}
	err := ac.log.EmitAuditEvent(ctx, in)
	require.NoError(t, err)

	var history []apievents.AuditEvent
	// We have batch time 10s, and 5s for upload and additional buffer for s3 download
	err = retryutils.RetryStaticFor(time.Second*20, time.Second*2, func() error {
		history, _, err = ac.log.SearchEvents(ctx, events.SearchEventsRequest{
			From:  ac.clock.Now().UTC().Add(-1 * time.Minute),
			To:    ac.clock.Now().UTC(),
			Limit: 10,
			Order: types.EventOrderDescending,
		})
		if err != nil {
			return err
		}
		if len(history) == 0 {
			return errors.New("events not propagated yet")
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Empty(t, cmp.Diff(in, history[0]))
}

// createTableQuery is query used to create athena table using parquet on s3.
// Right now only hardcoded in integration tests, in future it may be moved
// to athena main file if we decide to create table on demand from teleport.
var createTableQuery = `
CREATE EXTERNAL TABLE %s (
	uid string,
	session_id string,
	event_type string,
	event_time timestamp,
	user string,
	event_data string
  )
  PARTITIONED BY (
   event_date DATE
  )
  ROW FORMAT SERDE 'org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe'
  STORED AS INPUTFORMAT 'org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat'
  OUTPUTFORMAT 'org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat'
  LOCATION "%s/"
  TBLPROPERTIES (
   "projection.enabled" = "true",
   "projection.event_date.type" = "date",
   "projection.event_date.format" = "yyyy-MM-dd",
   "projection.event_date.range" = "NOW-4YEARS,NOW",
   "projection.event_date.interval" = "1",
   "projection.event_date.interval.unit" = "DAYS",
   "storage.location.template" = "%s/${event_date}/",
   "classification" = "parquet",
   "parquet.compression" = "SNAPPY"
  )
`

type AthenaContext struct {
	log                *Log
	clock              clockwork.Clock
	testID             string
	Database           string
	bucketForEvents    string
	bucketForTempFiles string
	TableName          string
	s3eventsLocation   string
	S3ResultsLocation  string
	s3largePayloads    string
	batcherInterval    time.Duration
}

func (a *AthenaContext) GetLog() *Log {
	return a.log
}

// AthenaContextConfig is optional config to override defaults in athena context.
type AthenaContextConfig struct {
	MaxBatchSize int
	BypassSNS    bool
}

type InfraOutputs struct {
	TopicARN string
	QueueURL string
	Region   string
}

func (ac *AthenaContext) Close(t *testing.T) {
	assert.NoError(t, ac.log.Close())
}

// EventuallyConsistentAuditLogger is used to add delay before searching for events
// for eventually consistent audit loggers.
type EventuallyConsistentAuditLogger struct {
	Inner events.AuditLogger

	// QueryDelay specifies how long query should wait after last emit event.
	QueryDelay time.Duration

	// mu protects field below.
	mu                    sync.Mutex
	emitWasAfterLastDelay bool
}

func (e *EventuallyConsistentAuditLogger) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	e.mu.Lock()
	e.emitWasAfterLastDelay = true
	e.mu.Unlock()
	return e.Inner.EmitAuditEvent(ctx, in)
}

func (e *EventuallyConsistentAuditLogger) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.QueryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.Inner.SearchEvents(ctx, req)
}

func (e *EventuallyConsistentAuditLogger) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.QueryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.Inner.ExportUnstructuredEvents(ctx, req)
}

func (e *EventuallyConsistentAuditLogger) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.QueryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.Inner.GetEventExportChunks(ctx, req)
}

func (e *EventuallyConsistentAuditLogger) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.QueryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.Inner.SearchSessionEvents(ctx, req)
}

func (e *EventuallyConsistentAuditLogger) Close() error {
	return e.Inner.Close()
}

func SetupAthenaContext(t *testing.T, ctx context.Context, cfg AthenaContextConfig) *AthenaContext {
	testEnabled := os.Getenv(teleport.AWSRunTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping AWS-dependent test suite.")
	}

	testID := fmt.Sprintf("auditlogs-integrationtests-%v", uuid.New().String())

	clock := clockwork.NewRealClock()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, backend.Close())
	})
	bucketWithLocking := "auditlogs-integrationtests-locking"
	bucketForTemporaryFiles := "auditlogs-integrationtests"
	ac := &AthenaContext{
		clock:              clock,
		testID:             testID,
		Database:           "auditlogs_integrationtests",
		bucketForEvents:    bucketWithLocking,
		bucketForTempFiles: bucketForTemporaryFiles,
		s3eventsLocation:   fmt.Sprintf("s3://%s/%s/events", bucketWithLocking, testID),
		S3ResultsLocation:  fmt.Sprintf("s3://%s/%s/results", bucketForTemporaryFiles, testID),
		s3largePayloads:    fmt.Sprintf("s3://%s/%s/large_payloads", bucketForTemporaryFiles, testID),
		TableName:          strings.ReplaceAll(testID, "-", "_"),
		batcherInterval:    10 * time.Second,
	}
	infraOut := ac.setupInfraWithCleanup(t, ctx)

	region := infraOut.Region
	if region == "" {
		region = "eu-central-1"
	}

	topicARN := infraOut.TopicARN
	if cfg.BypassSNS {
		topicARN = topicARNBypass
	}
	log, err := New(ctx, Config{
		Region:           region,
		Clock:            clock,
		Database:         ac.Database,
		TableName:        ac.TableName,
		TopicARN:         topicARN,
		QueueURL:         infraOut.QueueURL,
		LocationS3:       ac.s3eventsLocation,
		QueryResultsS3:   ac.S3ResultsLocation,
		LargeEventsS3:    ac.s3largePayloads,
		BatchMaxInterval: ac.batcherInterval,
		BatchMaxItems:    cfg.MaxBatchSize,
		Backend:          backend,
		Workgroup:        "primary",
	})
	require.NoError(t, err)

	ac.log = log
	t.Cleanup(func() {
		ac.Close(t)
	})

	t.Logf("Initialized Athena test suite %q\n", testID)

	return ac
}

func (ac *AthenaContext) setupInfraWithCleanup(t *testing.T, ctx context.Context) *InfraOutputs {
	const timeoutDurationOnCleanup = 1 * time.Minute

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	require.NoError(t, err)

	// Create SNS topic and set cleanup fn.
	snsClient := sns.NewFromConfig(awsCfg)
	topicCreated, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(ac.testID),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), timeoutDurationOnCleanup)
		defer cancel()
		_, err = snsClient.DeleteTopic(cleanupCtx, &sns.DeleteTopicInput{
			TopicArn: topicCreated.TopicArn,
		})
		assert.NoError(t, err)
	})

	// Create SQS queue and set cleanup fn.
	sqsClient := sqs.NewFromConfig(awsCfg)
	queueCreated, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(ac.testID),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), timeoutDurationOnCleanup)
		defer cancel()
		_, err := sqsClient.DeleteQueue(cleanupCtx, &sqs.DeleteQueueInput{
			QueueUrl: queueCreated.QueueUrl,
		})
		assert.NoError(t, err)
	})

	// Set created queue as subscriber to topic and use valid permissions.
	queueAttr, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       queueCreated.QueueUrl,
		AttributeNames: []sqsTypes.QueueAttributeName{sqsTypes.QueueAttributeNameQueueArn},
	})
	require.NoError(t, err)
	queueArn := queueAttr.Attributes["QueueArn"]
	type StatementEntry struct {
		Effect    string
		Action    []string
		Resource  string
		Principal map[string]string
		Condition map[string]map[string]string
	}
	type PolicyDocument struct {
		Version   string
		Statement []StatementEntry
	}
	sqsAccessPolicy := PolicyDocument{
		Version: "2012-10-17",
		Statement: []StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"SQS:SendMessage"},
				Resource: queueArn,
				Principal: map[string]string{
					"AWS": "*",
				},
				Condition: map[string]map[string]string{
					"ArnLike": {
						"aws:SourceArn": *topicCreated.TopicArn,
					},
				},
			},
		},
	}
	marshaledPolicy, err := json.Marshal(sqsAccessPolicy)
	require.NoError(t, err)
	_, err = sqsClient.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		Attributes: map[string]string{
			"Policy": string(marshaledPolicy),
		},
		QueueUrl: queueCreated.QueueUrl,
	})
	require.NoError(t, err)
	_, err = snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: topicCreated.TopicArn,
		Protocol: aws.String("sqs"),
		Attributes: map[string]string{
			"RawMessageDelivery": "true",
		},
		Endpoint: aws.String(queueArn),
	})
	require.NoError(t, err)

	// Create bucket for long term storage if not exists. Bucket will have object locking which
	// prevents from deleting objects, that's why it can exists before.
	// Retention period will take care of cleanup of files.
	s3Client := s3.NewFromConfig(awsCfg)
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(ac.bucketForEvents),
	})
	if err != nil {
		var notFound *s3Types.NotFound
		if errors.As(err, &notFound) {
			_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket:                     aws.String(ac.bucketForEvents),
				ObjectLockEnabledForBucket: aws.Bool(true),
				CreateBucketConfiguration: &s3Types.CreateBucketConfiguration{
					LocationConstraint: s3Types.BucketLocationConstraint(awsCfg.Region),
				},
			})
			require.NoError(t, err)
			_, err = s3Client.PutObjectLockConfiguration(ctx, &s3.PutObjectLockConfigurationInput{
				Bucket: aws.String(ac.bucketForEvents),
				ObjectLockConfiguration: &s3Types.ObjectLockConfiguration{
					ObjectLockEnabled: s3Types.ObjectLockEnabledEnabled,
					Rule: &s3Types.ObjectLockRule{
						DefaultRetention: &s3Types.DefaultRetention{
							Days: aws.Int32(1),
							Mode: s3Types.ObjectLockRetentionModeGovernance,
						},
					},
				},
			})
			require.NoError(t, err)
		} else {
			assert.Fail(t, "unexpected err", err)
		}
	}

	// Create bucket if not exists for temporary files (large payloads and query results).
	// Retention period will take care of cleanup of files.
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(ac.bucketForTempFiles),
	})
	if err != nil {
		var notFound *s3Types.NotFound
		if errors.As(err, &notFound) {
			_, createErr := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket: aws.String(ac.bucketForTempFiles),
				CreateBucketConfiguration: &s3Types.CreateBucketConfiguration{
					LocationConstraint: s3Types.BucketLocationConstraint(awsCfg.Region),
				},
			})
			require.NoError(t, createErr)
		} else {
			assert.Fail(t, "unexpected err", err)
		}
	}
	_, err = s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(ac.bucketForTempFiles),
		LifecycleConfiguration: &s3Types.BucketLifecycleConfiguration{
			Rules: []s3Types.LifecycleRule{
				{
					Status: s3Types.ExpirationStatusEnabled,
					Expiration: &s3Types.LifecycleExpiration{
						Days: aws.Int32(1),
					},
					// Prefix is required field, empty means set to whole bucket.
					Prefix: aws.String(""),
				},
			},
		},
	})
	require.NoError(t, err)

	// Create glue db if not exists
	glueClient := glue.NewFromConfig(awsCfg)
	_, err = glueClient.GetDatabase(ctx, &glue.GetDatabaseInput{
		Name: aws.String(ac.Database),
	})
	if err != nil {
		var notFound *glueTypes.EntityNotFoundException
		if errors.As(err, &notFound) {
			_, createErr := glueClient.CreateDatabase(ctx, &glue.CreateDatabaseInput{
				DatabaseInput: &glueTypes.DatabaseInput{
					Name: aws.String(ac.Database),
				},
			})
			require.NoError(t, createErr)
		} else {
			assert.Fail(t, "unexpected err: %v", err)
		}
	}

	// Create athena table
	athenaClient := athena.NewFromConfig(awsCfg)
	startQueryExecResp, err := athenaClient.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(fmt.Sprintf(createTableQuery, ac.TableName, ac.s3eventsLocation, ac.s3eventsLocation)),
		ResultConfiguration: &athenaTypes.ResultConfiguration{
			OutputLocation: aws.String(ac.S3ResultsLocation),
		},
		QueryExecutionContext: &athenaTypes.QueryExecutionContext{
			Database: aws.String(ac.Database),
		},
	})
	require.NoError(t, err)
	// querier is just used here to get helper fn waitForSuccess.
	q := querier{
		athenaClient: athenaClient,
		querierConfig: querierConfig{
			getQueryResultsInterval: 100 * time.Millisecond,
			clock:                   ac.clock,
			tracer:                  tracing.NoopTracer(teleport.ComponentAthena),
		},
	}
	err = q.waitForSuccess(ctx, aws.ToString(startQueryExecResp.QueryExecutionId))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), timeoutDurationOnCleanup)
		defer cancel()
		_, err = athenaClient.StartQueryExecution(cleanupCtx, &athena.StartQueryExecutionInput{
			QueryString: aws.String(fmt.Sprintf("drop table %s;", ac.TableName)),
			ResultConfiguration: &athenaTypes.ResultConfiguration{
				OutputLocation: aws.String(ac.S3ResultsLocation),
			},
			QueryExecutionContext: &athenaTypes.QueryExecutionContext{
				Database: aws.String(ac.Database),
			},
		})
		assert.NoError(t, err)
	})

	return &InfraOutputs{
		TopicARN: aws.ToString(topicCreated.TopicArn),
		QueueURL: aws.ToString(queueCreated.QueueUrl),
		Region:   awsCfg.Region,
	}
}
