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
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
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
)

// consumer is responsible for receiving messages from SQS, batching them up to
// certain size or interval, and writes to s3 as parquet file.
type consumer struct {
	logger              *log.Entry
	backend             backend.Backend
	storeLocationPrefix string
	storeLocationBucket string
	batchMaxItems       int
	batchMaxInterval    time.Duration

	collectConfig sqsCollectConfig
}

type sqsReceiver interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

type s3downloader interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error)
}

func newConsumer(cfg Config) (*consumer, error) {
	s3client := s3.NewFromConfig(*cfg.AWSConfig)
	sqsReceiver := sqs.NewFromConfig(*cfg.AWSConfig)

	collectCfg := sqsCollectConfig{
		sqsReceiver: sqsReceiver,
		queueURL:    cfg.QueueURL,
		// TODO(tobiaszheller): use s3 manager from teleport observability.
		payloadDownloader: manager.NewDownloader(s3client),
		payloadBucket:     cfg.largeEventsBucket,
		visibilityTimeout: int32(cfg.BatchMaxInterval.Seconds()),
		batchMaxItems:     cfg.BatchMaxItems,
		errHandlingFn:     errHandlingFnFromSQS(cfg.LogEntry),
		logger:            cfg.LogEntry,
	}
	err := collectCfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &consumer{
		logger:              cfg.LogEntry,
		backend:             cfg.Backend,
		storeLocationPrefix: cfg.locationS3Prefix,
		storeLocationBucket: cfg.locationS3Bucket,
		batchMaxItems:       cfg.BatchMaxItems,
		batchMaxInterval:    cfg.BatchMaxInterval,
		collectConfig:       collectCfg,
	}, nil
}

// run continuously runs batching job. It is blocking operation.
// It is stopped via canceling context.
func (c *consumer) run(ctx context.Context) {
	processBatchOfEventsWithLogging := func(context.Context) (reachedMaxBatch bool) {
		reachedMaxBatch, err := c.processBatchOfEvents(ctx)
		if err != nil {
			// Ctx.Cancel is used to stop batcher
			if ctx.Err() != nil {
				c.logger.Debug("Batcher has been stopped")
				return false
			}
			c.logger.Errorf("Batcher single run failed: %v", err)
			return false
		}
		return reachedMaxBatch
	}

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
	// TODO(tobiaszheller): we need some metrics to track it.
	// And that log message should be deleted.
	defer func() {
		if size > 0 {
			c.logger.Debugf("Batch of %d messages processed in %s", size, time.Since(start))
		}
	}()

	msgsCollector := newSqsMessagesCollector(c.collectConfig)

	readSQSCtx, readCancel := context.WithTimeout(ctx, c.batchMaxInterval)
	defer readCancel()

	// msgsCollector and writeToS3 runs concurrently, and use events channel
	// to send messages from collector to writeToS3.
	go func() {
		msgsCollector.fromSQS(readSQSCtx)
	}()
	var err error
	size, err = c.writeToS3(ctx, msgsCollector.getEventsChan())
	if err != nil {
		return false, trace.Wrap(err)
	}
	return size >= c.batchMaxItems, nil
	// TODO(tobiaszheller): delete messages from queue in next PR.
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

	logger        log.FieldLogger
	errHandlingFn func(ctx context.Context, errC chan error)
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
		cfg.logger = log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAthena,
		})
	}
	if cfg.errHandlingFn == nil {
		return trace.BadParameter("errHandlingFn is not specified")
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
	// Errors should be immediately process by error handling loop, so 10 size
	// should be enough to not cause blocking.
	errorsC := make(chan error, 10)
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
		count   int
		countMu sync.Mutex
		wg      sync.WaitGroup
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
				noOfReceived := s.receiveMessagesAndSendOnChan(wokerCtx, eventsC, errorsC)
				if noOfReceived == 0 {
					// no point of locking and checking for size if nothing was returned.
					continue
				}
				countMu.Lock()
				count += noOfReceived
				if count >= s.cfg.batchMaxItems {
					countMu.Unlock()
					cancel()
					return
				}
				countMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	close(eventsC)
}

func (s *sqsMessagesCollector) receiveMessagesAndSendOnChan(ctx context.Context, eventsC chan<- eventAndAckID, errorsC chan<- error) (size int) {
	sqsOut, err := s.cfg.sqsReceiver.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(s.cfg.queueURL),
		MaxNumberOfMessages:   maxNumberOfMessagesFromReceive,
		WaitTimeSeconds:       s.cfg.waitOnReceiveTimeout,
		VisibilityTimeout:     s.cfg.visibilityTimeout,
		MessageAttributeNames: []string{payloadTypeAttr},
	})
	if err != nil {
		// We don't need handle canceled errors anyhow.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0
		}
		errorsC <- trace.Wrap(err)

		// We don't want to retry receiving message immediately to prevent huge load
		// on CPU if calls are contantly failing.
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(s.cfg.waitOnReceiveError):
			return 0
		}
	}
	if len(sqsOut.Messages) == 0 {
		return 0
	}
	var noOfValidMessages int
	for _, msg := range sqsOut.Messages {
		event, err := s.auditEventFromSQSorS3(ctx, msg)
		if err != nil {
			errorsC <- trace.Wrap(err)
			continue
		}
		eventsC <- eventAndAckID{
			event:         event,
			receiptHandle: aws.ToString(msg.ReceiptHandle),
		}
		noOfValidMessages++
	}
	return noOfValidMessages
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

func errHandlingFnFromSQS(logger log.FieldLogger) func(ctx context.Context, errC chan error) {
	return func(ctx context.Context, errC chan error) {
		var errorsCount int

		defer func() {
			if errorsCount > maxErrorCountForLogsOnSQSReceive {
				logger.Errorf("Got %d errors from SQS collector, printed only first %d", errorsCount, maxErrorCountForLogsOnSQSReceive)
			}
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
					logger.WithError(err).Error("Failure processing SQS messages")
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

	s.cfg.logger.Debugf("Downloading %v %v [%v].", s.cfg.payloadBucket, path, versionID)

	buf := manager.NewWriteAtBuffer([]byte{})
	written, err := s.cfg.payloadDownloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket:    aws.String(s.cfg.payloadBucket),
		Key:       aws.String(path),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return nil, awsutils.ConvertS3Error(err)
	}
	if written == 0 {
		return nil, trace.NotFound("payload for %v is not found", path)
	}
	return buf.Bytes(), nil
}

// writeToS3 is not doing anything then just receiving from channel and printing
// for now. It will be changed in next PRs to actually write to S3 via parquet writer.
func (c *consumer) writeToS3(ctx context.Context, eventsChan <-chan eventAndAckID) (int, error) {
	var size int
	for {
		select {
		case <-ctx.Done():
			return size, trace.Wrap(ctx.Err())
		case eventAndAckID, ok := <-eventsChan:
			if !ok {
				return size, nil
			}
			size++
			c.logger.Debugf("Received event: %s %s", eventAndAckID.event.GetID(), eventAndAckID.event.GetType())
		}
	}
}
