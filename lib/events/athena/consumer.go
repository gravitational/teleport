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
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/parquet-go/parquet-go"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// maxWaitTimeOnReceiveMessageFromSQS defines how long single
	// receiveFromQueue will wait if there is no max events (10).
	maxWaitTimeOnReceiveMessageFromSQS = 5 * time.Second
	// maxNumberOfMessagesFromReceive defines how many messages single receive
	// call can return. Maximum value is 10.
	// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html
	maxNumberOfMessagesFromReceive = 10

	// maxErrorCountForLogsOnSQSReceive defines maximum number of error log messages
	// printed on receiving error from SQS receiver loop.
	maxErrorCountForLogsOnSQSReceive = 10

	// maxUniqueDaysInSingleBatch defines how many days are allowed in single batch.
	// Typically during normal operation there will be only one unique day,
	// but during a migration from another events backend, there could be a lot and using over 100 can result in huge
	// memory consumption, due to how s3 manager uploader works: https://github.com/aws/aws-sdk-go-v2/issues/1302.
	maxUniqueDaysInSingleBatch = 100
)

// consumer is responsible for receiving messages from SQS, batching them up to
// certain size or interval, and writes to s3 as parquet file.
type consumer struct {
	logger              *slog.Logger
	backend             backend.Backend
	storeLocationPrefix string
	storeLocationBucket string
	batchMaxItems       int
	batchMaxInterval    time.Duration
	consumerLockName    string

	// perDateFileParquetWriter returns file writer per date.
	// Added in config to allow testing.
	perDateFileParquetWriter func(ctx context.Context, date string) (io.WriteCloser, error)

	collectConfig sqsCollectConfig

	sqsDeleter sqsDeleter
	queueURL   string

	// observeWriteEventsError is called once for each error (including nil
	// errors) from writing events to S3.
	observeWriteEventsError func(error)

	// cancelRun is used to cancel consumer.Run
	cancelRun context.CancelFunc

	// finished is used to communicate that run (executed in background) has finished.
	// It will be closed when run has finished.
	finished chan struct{}
}

type sqsReceiver interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

type sqsDeleter interface {
	DeleteMessageBatch(ctx context.Context, params *sqs.DeleteMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error)
}

type s3downloader interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error)
}

func newConsumer(cfg Config, cancelFn context.CancelFunc) (*consumer, error) {
	// aggressively reuse connections to avoid choking up on TLS handshakes (the
	// default value for MaxIdleConnsPerHost is 2)
	sqsHTTPClient := awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		t.MaxIdleConns = defaults.HTTPMaxIdleConns
		t.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost
	})
	sqsClient := sqs.NewFromConfig(*cfg.PublisherConsumerAWSConfig,
		func(o *sqs.Options) {
			o.HTTPClient = sqsHTTPClient
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		})

	s3HTTPClient := awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		t.MaxIdleConns = defaults.HTTPMaxIdleConns
		t.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost
	})
	publisherS3Client := s3.NewFromConfig(*cfg.PublisherConsumerAWSConfig,
		func(o *s3.Options) {
			o.HTTPClient = s3HTTPClient
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		})
	storerS3Client := s3.NewFromConfig(*cfg.StorerQuerierAWSConfig,
		func(o *s3.Options) {
			o.HTTPClient = s3HTTPClient
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		})

	collectCfg := sqsCollectConfig{
		sqsReceiver: sqsClient,
		queueURL:    cfg.QueueURL,
		// TODO(nklaassen): use s3 manager from teleport observability.
		payloadDownloader: manager.NewDownloader(publisherS3Client),
		payloadBucket:     cfg.largeEventsBucket,
		visibilityTimeout: int32(cfg.BatchMaxInterval.Seconds()),
		batchMaxItems:     cfg.BatchMaxItems,
		errHandlingFn:     errHandlingFnFromSQS(&cfg),
		logger:            cfg.Logger,
		metrics:           cfg.metrics,
	}
	err := collectCfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cancelFn == nil {
		return nil, trace.BadParameter("cancelFn must be passed to consumer")
	}

	return &consumer{
		logger:              cfg.Logger,
		backend:             cfg.Backend,
		storeLocationPrefix: cfg.locationS3Prefix,
		storeLocationBucket: cfg.locationS3Bucket,
		batchMaxItems:       cfg.BatchMaxItems,
		batchMaxInterval:    cfg.BatchMaxInterval,
		consumerLockName:    cfg.ConsumerLockName,
		collectConfig:       collectCfg,
		sqsDeleter:          sqsClient,
		queueURL:            cfg.QueueURL,
		perDateFileParquetWriter: func(ctx context.Context, date string) (io.WriteCloser, error) {
			// use uuidv7 to give approximate time order to files. this isn't strictly necessary
			// but it assists in bulk event export roughly progressing in time order through a given
			// day which is what folks tend to expect.
			id, err := uuid.NewV7()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			key := fmt.Sprintf("%s/%s/%s.parquet", cfg.locationS3Prefix, date, id.String())
			fw, err := awsutils.NewS3V2FileWriter(ctx, storerS3Client, cfg.locationS3Bucket, key, nil /* uploader options */, func(poi *s3.PutObjectInput) {
				// ChecksumAlgorithm is required for putting objects when object lock is enabled.
				poi.ChecksumAlgorithm = s3Types.ChecksumAlgorithmSha256
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return fw, nil
		},
		observeWriteEventsError: cfg.ObserveWriteEventsError,
		cancelRun:               cancelFn,
		finished:                make(chan struct{}),
	}, nil
}

// run continuously runs batching job. It is blocking operation.
// It is stopped via canceling context.
func (c *consumer) run(ctx context.Context) {
	defer func() {
		close(c.finished)
		c.logger.DebugContext(ctx, "Consumer finished")
	}()
	c.runContinuouslyOnSingleAuth(ctx, c.processEventsContinuously)
}

// Close terminates the goroutine which is running [c.run]
func (c *consumer) Close() error {
	c.cancelRun()
	select {
	case <-c.finished:
		return nil
	case <-time.After(1 * time.Second):
		// ctx is use through all calls within consumer.Run so it should finished
		// very fast, within miliseconds.
		return errors.New("consumer not finished in time, returning earlier")
	}
}

// processEventsContinuously runs processBatchOfEvents continuously in a loop.
// It makes sure that the CPU won't be spammed with too many requests if something goes
// wrong with calls to the AWS API.
func (c *consumer) processEventsContinuously(ctx context.Context) {
	processBatchOfEventsWithLogging := func(context.Context) (reachedMaxBatch bool) {
		reachedMaxBatch, err := c.processBatchOfEvents(ctx)
		c.observeWriteEventsError(err)
		if err != nil {
			// Ctx.Cancel is used to stop batcher
			if ctx.Err() != nil {
				return false
			}
			c.logger.ErrorContext(ctx, "Batcher single run failed", "error", err)
			return false
		}
		return reachedMaxBatch
	}

	c.logger.DebugContext(ctx, "Processing of events started on this instance")
	defer c.logger.DebugContext(ctx, "Processing of events finished on this instance")

	// If batch took 90% of specified interval, we don't want to wait just little bit.
	// It's mainly to avoid cases when we will wait like 10ms.
	minInterval := time.Duration(float64(c.batchMaxInterval) * 0.9)

	var stop bool
	for {
		// We use helper fn [runWithMinInterval] to guarantee that we won't spam
		// CPU if processBatchOfEvents will return immediately without processing
		// any data. runWithMinInterval guarantees that if fn finished earlier,
		// it will wait reaming time of interval before proceeding.
		stop = runWithMinInterval(ctx, processBatchOfEventsWithLogging, minInterval)
		if stop {
			return
		}
	}
}

// runContinuouslyOnSingleAuth runs eventsProcessorFn continuously on single auth instance.
// Backend locking is used to make sure that only single auth is running consumer.
func (c *consumer) runContinuouslyOnSingleAuth(ctx context.Context, eventsProcessorFn func(context.Context)) {
	// for 1 minute it will be 5s sleep before retry which seems like reasonable value.
	waitTimeAfterLockingError := retryutils.SeventhJitter(c.batchMaxInterval / 12)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			lockName := []string{"athena", c.consumerLockName}
			if c.consumerLockName == "" {
				lockName = []string{"athena_lock"}
			}
			err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					Backend:            c.backend,
					LockNameComponents: lockName,
					// TTL is higher then batchMaxInterval because we want to optimize
					// for low backend writes.
					TTL: 5 * c.batchMaxInterval,
					// RetryInterval means how often instance without lock will check
					// backend if lock if ready for grab. We are fine with batchMaxInterval.
					RetryInterval: c.batchMaxInterval,
				},
			}, func(ctx context.Context) error {
				eventsProcessorFn(ctx)
				return nil
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// Ending up here means something went wrong in the backend while locking/waiting
				// for lock. What we can do is log and retry whole operation.
				c.logger.WarnContext(ctx, "Could not get consumer to run with lock", "error", err)
				select {
				// Use wait to make sure we won't spam CPU with a lot requests
				// if something goes wrong during acquire lock.
				case <-time.After(waitTimeAfterLockingError):
					continue
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// runWithMinInterval runs fn, if fn returns earlier than minInterval
// it waits reamaning time.
// Useful when we don't want to put to many pressure on CPU with constantly running fn.
func runWithMinInterval(ctx context.Context, fn func(context.Context) bool, minInterval time.Duration) (stop bool) {
	start := time.Now()
	reachedMaxBatch := fn(ctx)
	if ctx.Err() != nil {
		// stopping
		return true
	}
	if reachedMaxBatch {
		// reachedMaxBatch means that fn reached maxBatchSize. We don't want
		// to wait in that case.
		return false
	}
	elapsed := time.Since(start)
	if elapsed > minInterval {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	case <-time.After(minInterval - elapsed):
		return false
	}
}

// processBatchOfEvents creates single batch of events. It waits either up to BatchMaxInterval
// or BatchMaxItems while reading events from queue. Batch is sent to s3 as
// parquet file and at the end events are deleted from queue.
func (c *consumer) processBatchOfEvents(ctx context.Context) (reachedMaxSize bool, e error) {
	start := time.Now()
	var size int
	defer func() {
		c.collectConfig.metrics.consumerLastProcessedTimestamp.SetToCurrentTime()
		c.collectConfig.metrics.consumerBatchProcessingDuration.Observe(time.Since(start).Seconds())
	}()

	msgsCollector := newSqsMessagesCollector(c.collectConfig)

	readSQSCtx, readCancel := context.WithTimeout(ctx, c.batchMaxInterval)
	defer readCancel()

	// msgsCollector and writeToS3 runs concurrently, and use events channel
	// to send messages from collector to writeToS3.
	go func() {
		msgsCollector.fromSQS(readSQSCtx)
	}()

	toDelete, err := c.writeToS3(ctx, msgsCollector.getEventsChan(), c.perDateFileParquetWriter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	size = len(toDelete)
	return size >= c.batchMaxItems, trace.Wrap(c.deleteMessagesFromQueue(ctx, toDelete))
}

type sqsCollectConfig struct {
	sqsReceiver       sqsReceiver
	queueURL          string
	payloadBucket     string
	payloadDownloader s3downloader
	// visibilityTimeout defines how long message won't be available for other
	// receiveMessage calls. If timeout happens, and message was not deleted
	// it will return to the queue.
	visibilityTimeout int32
	// waitOnReceiveDuration defines how long single
	// receiveFromQueue will wait if there is no max events (10).
	waitOnReceiveDuration time.Duration
	// waitOnReceiveTimeout is int32 representation of waitOnReceiveDuration
	// required by AWS API.
	waitOnReceiveTimeout int32

	// waitOnReceiveError defines interval used to wait before
	// retrying receive message from SQS after getting error.
	waitOnReceiveError time.Duration

	batchMaxItems int

	// noOfWorkers defines how many workers are processing messages from queue.
	noOfWorkers int

	logger        *slog.Logger
	errHandlingFn func(ctx context.Context, errC chan error)

	metrics *athenaMetrics
}

func (cfg *sqsCollectConfig) CheckAndSetDefaults() error {
	if cfg.sqsReceiver == nil {
		return trace.BadParameter("sqsReceiver is not specified")
	}
	if cfg.queueURL == "" {
		return trace.BadParameter("queueURL is not specified")
	}
	if cfg.payloadBucket == "" {
		return trace.BadParameter("payloadBucket is not specified")
	}
	if cfg.payloadDownloader == nil {
		return trace.BadParameter("payloadDownloader is not specified")
	}
	if cfg.visibilityTimeout == 0 {
		// visibilityTimeout is timeout in seconds, so 1 minute.
		cfg.visibilityTimeout = int32(defaultBatchInterval.Seconds())
	}
	if cfg.waitOnReceiveDuration == 0 {
		cfg.waitOnReceiveDuration = maxWaitTimeOnReceiveMessageFromSQS
	}
	if cfg.waitOnReceiveTimeout != 0 {
		return trace.BadParameter("waitOnReceiveTimeout is calculated internally and should not be set")
	}
	cfg.waitOnReceiveTimeout = int32(cfg.waitOnReceiveDuration.Seconds())

	if cfg.waitOnReceiveError == 0 {
		cfg.waitOnReceiveError = 1 * time.Second
	}
	if cfg.batchMaxItems == 0 {
		cfg.batchMaxItems = defaultBatchItems
	}
	if cfg.noOfWorkers == 0 {
		cfg.noOfWorkers = 5
	}
	if cfg.logger == nil {
		cfg.logger = slog.With(teleport.ComponentKey, teleport.ComponentAthena)
	}
	if cfg.errHandlingFn == nil {
		return trace.BadParameter("errHandlingFn is not specified")
	}
	if cfg.metrics == nil {
		return trace.BadParameter("metrics is not specified")
	}
	return nil
}

// sqsMessagesCollector is responsible for collecting messages from SQS and
// writing to on channel.
type sqsMessagesCollector struct {
	cfg        sqsCollectConfig
	eventsChan chan eventAndAckID
}

// newSqsMessagesCollector returns message collector.
// Collector sends collected messages from SQS on events channel.
func newSqsMessagesCollector(cfg sqsCollectConfig) *sqsMessagesCollector {
	return &sqsMessagesCollector{
		cfg:        cfg,
		eventsChan: make(chan eventAndAckID, cfg.batchMaxItems),
	}
}

// getEventsChan returns channel which can be used to read messages from SQS.
// When collector finishes, channel will be closed.
func (s *sqsMessagesCollector) getEventsChan() <-chan eventAndAckID {
	return s.eventsChan
}

// fromSQS receives messages from SQS and sends it on eventsC channel.
// It runs until context is canceled (via timeout) or when maxItems is reached.
// MaxItems is soft limit and can happen that it will return more items then MaxItems.
func (s *sqsMessagesCollector) fromSQS(ctx context.Context) {
	errorsC := make(chan error)
	defer close(errorsC)

	// errhandle loop for receiving single event errors.
	go func() {
		s.cfg.errHandlingFn(ctx, errorsC)
	}()
	eventsC := s.eventsChan

	// wokerCtx is mechanism to stop other workers when maxItems is reached.
	wokerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		fullBatchMetadata   collectedEventsMetadata
		fullBatchMetadataMu sync.Mutex
		wg                  sync.WaitGroup
	)

	wg.Add(s.cfg.noOfWorkers)
	for i := 0; i < s.cfg.noOfWorkers; i++ {
		go func(i int) {
			defer wg.Done()
			for {
				if wokerCtx.Err() != nil {
					return
				}
				// If there is not enough time to process receiveMessage call
				// we can return immediately. It's added because if
				// receiveMessages is canceled message is marked as not
				// processed after VisibilitTimeout (equal to BatchInterval).
				if deadline, ok := wokerCtx.Deadline(); ok && time.Until(deadline) <= s.cfg.waitOnReceiveDuration {
					return
				}
				singleReceiveMetadata := s.receiveMessagesAndSendOnChan(wokerCtx, eventsC, errorsC)
				if singleReceiveMetadata.Count == 0 {
					// no point of locking and checking for size if nothing was returned.
					continue
				}

				fullBatchMetadataMu.Lock()
				fullBatchMetadata.Merge(singleReceiveMetadata)
				isOverBatch := fullBatchMetadata.Count >= s.cfg.batchMaxItems
				isOverMaximumUniqueDays := len(fullBatchMetadata.UniqueDays) > maxUniqueDaysInSingleBatch
				if isOverBatch || isOverMaximumUniqueDays {
					fullBatchMetadataMu.Unlock()
					cancel()
					s.cfg.logger.DebugContext(ctx, "Batcher aborting early", "max_size", isOverBatch, "max_unique_days", isOverMaximumUniqueDays)
					return
				}
				fullBatchMetadataMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	close(eventsC)
	if fullBatchMetadata.Count > 0 {
		s.cfg.metrics.consumerBatchCount.Add(float64(fullBatchMetadata.Count))
		s.cfg.metrics.consumerBatchSize.Observe(float64(fullBatchMetadata.Size))
		s.cfg.metrics.consumerAgeOfOldestProcessedMessage.Set(time.Since(fullBatchMetadata.OldestTimestamp).Seconds())
	} else {
		// When no messages were processed, clear gauge metric.
		s.cfg.metrics.consumerAgeOfOldestProcessedMessage.Set(0)
	}
}

type collectedEventsMetadata struct {
	// Size is total size of events.
	Size int
	// Count is number of events.
	Count int
	// OldestTimestamp is timestamp of oldest event.
	OldestTimestamp time.Time
	// UniqueDays tracks from how many days events are received in current batch.
	UniqueDays map[string]struct{}
}

// Merge combines the metadata of two collectedEventsMetadata instances.
// It updates the current instance by adding the size and count of events from another instance,
// and sets the oldestTimestamp to the oldest timestamp between the two instances.
func (c *collectedEventsMetadata) Merge(in collectedEventsMetadata) {
	c.Size += in.Size
	c.Count += in.Count
	c.mergeUniqueDays(in.UniqueDays)
	if c.OldestTimestamp.IsZero() || (!in.OldestTimestamp.IsZero() && c.OldestTimestamp.After(in.OldestTimestamp)) {
		c.OldestTimestamp = in.OldestTimestamp
	}
}

func (c *collectedEventsMetadata) mergeUniqueDays(mapToMerge map[string]struct{}) {
	if mapToMerge == nil {
		return
	}
	if c.UniqueDays == nil {
		c.UniqueDays = mapToMerge
		return
	}
	for k := range mapToMerge {
		if _, ok := c.UniqueDays[k]; !ok {
			c.UniqueDays[k] = struct{}{}
		}
	}
}

// MergeWithEvent combines collectedEventsMetadata with metadata of single event.
func (c *collectedEventsMetadata) MergeWithEvent(in apievents.AuditEvent, publishedToQueueTimestamp time.Time) {
	c.Merge(collectedEventsMetadata{
		// 1 because we are merging single event
		Count:           1,
		Size:            in.Size(),
		OldestTimestamp: publishedToQueueTimestamp,
		UniqueDays: map[string]struct{}{
			in.GetTime().Format(time.DateOnly): {},
		},
	})
}

// SDK defines invalid type for attribute names, let's just use const here.
// https://github.com/aws/aws-sdk-go-v2/issues/2124
const sentTimestampAttribute = "SentTimestamp"

func (s *sqsMessagesCollector) receiveMessagesAndSendOnChan(ctx context.Context, eventsC chan<- eventAndAckID, errorsC chan<- error) collectedEventsMetadata {
	sqsOut, err := s.cfg.sqsReceiver.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(s.cfg.queueURL),
		MaxNumberOfMessages:   maxNumberOfMessagesFromReceive,
		WaitTimeSeconds:       s.cfg.waitOnReceiveTimeout,
		VisibilityTimeout:     s.cfg.visibilityTimeout,
		MessageAttributeNames: []string{payloadTypeAttr},
		AttributeNames:        []sqsTypes.QueueAttributeName{sentTimestampAttribute},
	})
	if err != nil {
		// We don't need handle canceled errors anyhow.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return collectedEventsMetadata{}
		}
		select {
		case errorsC <- trace.Wrap(err):
		case <-ctx.Done():
			return collectedEventsMetadata{}
		}

		// We don't want to retry receiving message immediately to prevent huge load
		// on CPU if calls are contantly failing.
		select {
		case <-ctx.Done():
			return collectedEventsMetadata{}
		case <-time.After(s.cfg.waitOnReceiveError):
			return collectedEventsMetadata{}
		}
	}
	if len(sqsOut.Messages) == 0 {
		return collectedEventsMetadata{}
	}
	var singleReceiveMetadata collectedEventsMetadata
	for _, msg := range sqsOut.Messages {
		event, err := s.auditEventFromSQSorS3(ctx, msg)
		if err != nil {
			select {
			case errorsC <- trace.Wrap(err):
			case <-ctx.Done():
			}
			continue
		}
		eventsC <- eventAndAckID{
			event:         event,
			receiptHandle: aws.ToString(msg.ReceiptHandle),
		}
		messageSentTimestamp, err := getMessageSentTimestamp(msg)
		if err != nil {
			s.cfg.logger.DebugContext(ctx, "Failed to get sentTimestamp", "error", err)
		}
		singleReceiveMetadata.MergeWithEvent(event, messageSentTimestamp)
	}
	return singleReceiveMetadata
}

func getMessageSentTimestamp(msg sqsTypes.Message) (time.Time, error) {
	if msg.Attributes == nil {
		return time.Time{}, nil
	}
	attribute := msg.Attributes[sentTimestampAttribute]
	if attribute == "" {
		return time.Time{}, nil
	}
	milis, err := strconv.Atoi(attribute)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	return time.UnixMilli(int64(milis)).UTC(), nil
}

// auditEventFromSQSorS3 returns events either directly from SQS message payload
// or from s3, if event was very large.
func (s *sqsMessagesCollector) auditEventFromSQSorS3(ctx context.Context, msg sqsTypes.Message) (apievents.AuditEvent, error) {
	payloadType, err := validateSQSMessage(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var protoMarshaledOneOf []byte
	switch payloadType {
	// default case is hanlded in validateSQSMessage.
	case payloadTypeS3Based:
		protoMarshaledOneOf, err = s.downloadEventFromS3(ctx, *msg.Body)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case payloadTypeRawProtoEvent:
		protoMarshaledOneOf, err = base64.StdEncoding.DecodeString(*msg.Body)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var oneOf apievents.OneOf
	if err := oneOf.Unmarshal(protoMarshaledOneOf); err != nil {
		return nil, trace.Wrap(err)
	}
	event, err := apievents.FromOneOf(oneOf)
	return event, trace.Wrap(err)
}

func validateSQSMessage(msg sqsTypes.Message) (string, error) {
	if msg.Body == nil || msg.MessageAttributes == nil {
		// This should not happen. If it happen though, it will be retried
		// and go to dead-letter queue after max attempts.
		return "", trace.BadParameter("missing Body or MessageAttributes of msg: %v", msg)
	}
	if msg.ReceiptHandle == nil {
		return "", trace.BadParameter("missing ReceiptHandle")
	}
	v := msg.MessageAttributes[payloadTypeAttr]
	if v.StringValue == nil {
		// This should not happen. If it happen though, it will be retried
		// and go to dead-letter queue after max attempts.
		return "", trace.BadParameter("message without %q attribute", payloadTypeAttr)
	}
	payloadType := *v.StringValue
	if !slices.Contains([]string{payloadTypeRawProtoEvent, payloadTypeS3Based}, payloadType) {
		return "", trace.BadParameter("unsupported payload type %s", payloadType)
	}
	return payloadType, nil
}

type eventAndAckID struct {
	event         apievents.AuditEvent
	receiptHandle string
}

func errHandlingFnFromSQS(cfg *Config) func(ctx context.Context, errC chan error) {
	return func(ctx context.Context, errC chan error) {
		var errorsCount int

		defer func() {
			if errorsCount > maxErrorCountForLogsOnSQSReceive {
				cfg.Logger.ErrorContext(ctx, "Got errors from SQS collector", "error_count", errorsCount)
			}
			cfg.metrics.consumerNumberOfErrorsFromSQSCollect.Add(float64(errorsCount))
		}()

		for {
			select {
			case <-ctx.Done():
				// if errorsCount > maxErrorCountForLogs, log will be printed via defer.
				return
			case err, ok := <-errC:
				if !ok {
					return
				}
				errorsCount++
				if errorsCount <= maxErrorCountForLogsOnSQSReceive {
					cfg.Logger.ErrorContext(ctx, "Failure processing SQS messages", "error", err)
				}
			}
		}
	}
}

func (s *sqsMessagesCollector) downloadEventFromS3(ctx context.Context, payload string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s3Payload := &apievents.AthenaS3EventPayload{}
	if err := s3Payload.Unmarshal(decoded); err != nil {
		return nil, trace.Wrap(err)
	}

	path := s3Payload.GetPath()
	versionID := s3Payload.GetVersionId()

	s.cfg.logger.DebugContext(ctx, "Downloading event from S3", "bucket", s.cfg.payloadBucket, "path", path, "version", versionID)

	var versionIDPtr *string
	if versionID != "" {
		versionIDPtr = aws.String(versionID)
	}

	buf := manager.NewWriteAtBuffer([]byte{})
	written, err := s.cfg.payloadDownloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket:    aws.String(s.cfg.payloadBucket),
		Key:       aws.String(path),
		VersionId: versionIDPtr,
	})
	if err != nil {
		return nil, awsutils.ConvertS3Error(err)
	}
	if written == 0 {
		return nil, trace.NotFound("payload for %v is not found", path)
	}
	return buf.Bytes(), nil
}

// writeToS3 reades events from eventsCh and writes them via parquet writer
// to s3 bucket. It returns receiptHandles of elements to delete from queue.
// If error is returned, it means that messages won't be deleted from SQS,
// and events will be retried or go to dead-letter queue.
func (c *consumer) writeToS3(ctx context.Context, eventsCh <-chan eventAndAckID, newPerDateFileWriterFn func(ctx context.Context, date string) (io.WriteCloser, error)) ([]string, error) {
	toDelete := make([]string, 0, c.batchMaxItems)
	// TODO(tobiaszheller): later write in goroutine, so far it's not bottleneck.
	perDateWriter := map[string]*parquetWriter{}
eventLoop:
	for {
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case eventAndAckID, ok := <-eventsCh:
			if !ok {
				break eventLoop
			}
			pqtEvent, err := auditEventToParquet(eventAndAckID.event)
			if err != nil {
				c.logger.ErrorContext(ctx, "Could not convert event to parquet format", "error", err)
				continue
			}
			date := pqtEvent.EventTime.Format(time.DateOnly)
			pw := perDateWriter[date]
			if pw == nil {
				fw, err := newPerDateFileWriterFn(ctx, date)
				if err != nil {
					// While using s3 file writer, error is not used
					// when creating file writer.
					return nil, trace.Wrap(err)
				}
				pw, err = newParquetWriter(ctx, fw)
				if err != nil {
					// Error here means that probably something is wrong with
					// parquet schema. Returning from fn with error make sense.
					return nil, trace.Wrap(err)
				}
				perDateWriter[date] = pw
			}
			if err := pw.Write(ctx, *pqtEvent); err != nil {
				// pw.Write returns error only on flushing operation which
				// does not happen on every write.
				// So there is no easy way to say, which event caused trouble and
				// skip it.
				// It may happen that one wrong entry will cause whole batch
				// to write failure. Although it should not happen often because
				// we are validating message before. If it happen though, whole
				// batch will go to dead letter.
				// TODO(tobiaszheller): check how other parquet libs are handling it.
				// or maybe use Flush explicitly. Need to check performance.
				return nil, trace.Wrap(err)
			}

			// Elements are just added to slice here. Acknowledge happens only if whole
			// writeToS3 method succeed.
			toDelete = append(toDelete, eventAndAckID.receiptHandle)
		}
	}
	eventLoopFinishedTime := time.Now()
	defer func() {
		c.collectConfig.metrics.consumerS3parquetFlushDuration.Observe(time.Since(eventLoopFinishedTime).Seconds())
	}()
	for _, pw := range perDateWriter {
		if err := pw.Close(); err != nil {
			// Typically there will be data just for one date.
			// If we are not able to close parquet file, it make sense to retrun
			// error and retry whole batch again from SQS.

			return nil, trace.Wrap(err)
		}
	}
	return toDelete, nil
}

func newParquetWriter(ctx context.Context, fw io.WriteCloser) (*parquetWriter, error) {
	pw := parquet.NewGenericWriter[eventParquet](fw, parquet.Compression(&parquet.Snappy))
	return &parquetWriter{
		closer: fw,
		writer: pw,
	}, nil
}

type parquetWriter struct {
	closer io.Closer
	writer *parquet.GenericWriter[eventParquet]
}

func (pw *parquetWriter) Write(ctx context.Context, in eventParquet) error {
	_, err := pw.writer.Write([]eventParquet{in})
	return trace.Wrap(err)
}

func (pw *parquetWriter) Close() error {
	if err := pw.writer.Close(); err != nil {
		return trace.NewAggregate(err, pw.closer.Close())
	}
	return trace.Wrap(pw.closer.Close())
}

func (c *consumer) deleteMessagesFromQueue(ctx context.Context, handles []string) error {
	if len(handles) == 0 {
		return nil
	}
	start := time.Now()
	defer func() {
		c.collectConfig.metrics.consumerDeleteMessageDuration.Observe(time.Since(start).Seconds())
	}()
	const (
		// maxDeleteBatchSize defines maximum number of handles passed to deleteMessage endpoint, limited by AWS.
		maxDeleteBatchSize = 10
		// noOfWorkers defines number of workers which concurrently process delete batch request.
		noOfWorkers = 5
	)

	errorsCh := make(chan error, len(handles))
	workerCh := make(chan []string, noOfWorkers)

	var wg sync.WaitGroup

	// Start the worker goroutines
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for handles := range workerCh {
				entries := make([]sqsTypes.DeleteMessageBatchRequestEntry, 0, len(handles))
				for _, h := range handles {
					entries = append(entries, sqsTypes.DeleteMessageBatchRequestEntry{
						Id:            aws.String(uuid.NewString()),
						ReceiptHandle: aws.String(h),
					})
				}
				resp, err := c.sqsDeleter.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
					QueueUrl: aws.String(c.queueURL),
					Entries:  entries,
				})
				if err != nil {
					errorsCh <- trace.Wrap(err, "error on calling DeleteMessageBatch")
					continue
				}
				for _, entry := range resp.Failed {
					// TODO(tobiaszheller): come back at some point and check if there are errors that we should filter.
					// Deleting the same handle twice does not result in error.
					errorsCh <- trace.Errorf("failed to delete message with ID %s, sender fault %v: %s", aws.ToString(entry.Id), entry.SenderFault, aws.ToString(entry.Message))
				}
			}
		}()
	}

	// Batch the receipt handles and send them to the worker pool.
	for i := 0; i < len(handles); i += maxDeleteBatchSize {
		end := i + maxDeleteBatchSize
		if end > len(handles) {
			end = len(handles)
		}
		workerCh <- handles[i:end]
	}
	close(workerCh)

	wg.Wait()
	// We can close errorsCh when all goroutine has finished, now we will
	// be able to collect results.
	close(errorsCh)

	return trace.Wrap(trace.NewAggregateFromChannel(errorsCh, ctx))
}
