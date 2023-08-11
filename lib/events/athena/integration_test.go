// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/observability/tracing"
)

type athenaContext struct {
	log                *Log
	clock              clockwork.Clock
	testID             string
	database           string
	bucketForEvents    string
	bucketForTempFiles string
	tablename          string
	s3eventsLocation   string
	s3resultsLocation  string
	s3largePayloads    string
	batcherInterval    time.Duration
}

func TestIntegrationAthenaSearchSessionEventsBySessionID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := setupAthenaContext(t, ctx, athenaContextConfig{})
	auditLogger := &eventuallyConsitentAuditLogger{
		inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		queryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:                                  auditLogger,
		Clock:                                ac.clock,
		SearchSessionEvensBySessionIDTimeout: ac.batcherInterval + 10*time.Second,
	}

	eventsSuite.SearchSessionEventsBySessionID(t)
}

func TestIntegrationAthenaSessionEventsCRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := setupAthenaContext(t, ctx, athenaContextConfig{})
	auditLogger := &eventuallyConsitentAuditLogger{
		inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		queryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.SessionEventsCRUD(t)
}

func TestIntegrationAthenaEventPagination(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := setupAthenaContext(t, ctx, athenaContextConfig{})
	auditLogger := &eventuallyConsitentAuditLogger{
		inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		queryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.EventPagination(t)
}

func TestIntegrationAthenaLargeEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ac := setupAthenaContext(t, ctx, athenaContextConfig{maxBatchSize: 1})
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

// athenaContextConfig is optional config to override defaults in athena context.
type athenaContextConfig struct {
	maxBatchSize int
}

func setupAthenaContext(t *testing.T, ctx context.Context, cfg athenaContextConfig) *athenaContext {
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
	ac := &athenaContext{
		clock:              clock,
		testID:             testID,
		database:           "auditlogs_integrationtests",
		bucketForEvents:    bucketWithLocking,
		bucketForTempFiles: bucketForTemporaryFiles,
		s3eventsLocation:   fmt.Sprintf("s3://%s/%s/events", bucketWithLocking, testID),
		s3resultsLocation:  fmt.Sprintf("s3://%s/%s/results", bucketForTemporaryFiles, testID),
		s3largePayloads:    fmt.Sprintf("s3://%s/%s/large_payloads", bucketForTemporaryFiles, testID),
		tablename:          strings.ReplaceAll(testID, "-", "_"),
		batcherInterval:    10 * time.Second,
	}
	infraOut := ac.setupInfraWithCleanup(t, ctx)

	region := infraOut.region
	if region == "" {
		region = "eu-central-1"
	}

	log, err := New(ctx, Config{
		Region:           region,
		Clock:            clock,
		Database:         ac.database,
		TableName:        ac.tablename,
		TopicARN:         infraOut.topicARN,
		QueueURL:         infraOut.queueURL,
		LocationS3:       ac.s3eventsLocation,
		QueryResultsS3:   ac.s3resultsLocation,
		LargeEventsS3:    ac.s3largePayloads,
		BatchMaxInterval: ac.batcherInterval,
		BatchMaxItems:    cfg.maxBatchSize,
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

type infraOutputs struct {
	topicARN string
	queueURL string
	region   string
}

func (ac *athenaContext) setupInfraWithCleanup(t *testing.T, ctx context.Context) *infraOutputs {
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
				ObjectLockEnabledForBucket: true,
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
							Days: 1,
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
						Days: 1,
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
		Name: aws.String(ac.database),
	})
	if err != nil {
		var notFound *glueTypes.EntityNotFoundException
		if errors.As(err, &notFound) {
			_, createErr := glueClient.CreateDatabase(ctx, &glue.CreateDatabaseInput{
				DatabaseInput: &glueTypes.DatabaseInput{
					Name: aws.String(ac.database),
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
		QueryString: aws.String(fmt.Sprintf(createTableQuery, ac.tablename, ac.s3eventsLocation, ac.s3eventsLocation)),
		ResultConfiguration: &athenaTypes.ResultConfiguration{
			OutputLocation: aws.String(ac.s3resultsLocation),
		},
		QueryExecutionContext: &athenaTypes.QueryExecutionContext{
			Database: aws.String(ac.database),
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
			QueryString: aws.String(fmt.Sprintf("drop table %s;", ac.tablename)),
			ResultConfiguration: &athenaTypes.ResultConfiguration{
				OutputLocation: aws.String(ac.s3resultsLocation),
			},
			QueryExecutionContext: &athenaTypes.QueryExecutionContext{
				Database: aws.String(ac.database),
			},
		})
		assert.NoError(t, err)
	})

	return &infraOutputs{
		topicARN: aws.ToString(topicCreated.TopicArn),
		queueURL: aws.ToString(queueCreated.QueueUrl),
		region:   awsCfg.Region,
	}
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

func (ac *athenaContext) Close(t *testing.T) {
	assert.NoError(t, ac.log.Close())
}

// eventuallyConsitentAuditLogger is used to add delay before searching for events
// for eventually consistent audit loggers.
type eventuallyConsitentAuditLogger struct {
	inner events.AuditLogger

	// queryDelay specifies how long query should wait after last emit event.
	queryDelay time.Duration

	// mu protects field below.
	mu                    sync.Mutex
	emitWasAfterLastDelay bool
}

func (e *eventuallyConsitentAuditLogger) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	e.mu.Lock()
	e.emitWasAfterLastDelay = true
	e.mu.Unlock()
	return e.inner.EmitAuditEvent(ctx, in)
}

func (e *eventuallyConsitentAuditLogger) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.queryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.inner.SearchEvents(ctx, req)
}

func (e *eventuallyConsitentAuditLogger) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.emitWasAfterLastDelay {
		time.Sleep(e.queryDelay)
		// clear emit delay
		e.emitWasAfterLastDelay = false
	}
	return e.inner.SearchSessionEvents(ctx, req)
}

func (e *eventuallyConsitentAuditLogger) Close() error {
	return e.inner.Close()
}
