/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSQSPollEvents(t *testing.T) {
	t.Parallel()

	fakeSQSQueue := newFakeQueue()
	fakeS3Bucket := newFakeS3Bucket(
		"bucket4/key3",
	)
	s := &Server{
		Config: &Config{
			Log: slog.Default(),
		},
	}

	ctx, cancel := context.WithCancel(t.Context())

	eventsC := make(chan payloadChannelMessage)
	errC := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.pollEventsFromSQSFilesImpl(
			ctx,
			"accountID",
			fakeSQSQueue,
			fakeS3Bucket,
			&types.AccessGraphAWSSyncCloudTrailLogs{
				SQSQueue: "queueURL",
				Region:   "us-east-1",
			},
			eventsC,
		)
		errC <- err
	}()

	publish := func(bucket, id string, keys ...string) {
		err := fakeSQSQueue.Publish(ctx, sqsFileEvent{
			S3Bucket:    bucket,
			S3ObjectKey: keys,
		}, id)
		require.NoError(t, err)
	}

	publish("bucket1", "messageID1", "key1")

	select {
	case <-ctx.Done():
		cancel()
		return
	case err := <-errC:
		require.Fail(t, "unexpected error", err)
	case msg := <-eventsC:
		require.Equal(t, []byte("bucket1/key1"), msg.payload)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.ElementsMatch(t, []string{"messageID1"}, fakeSQSQueue.getDeletedMessages())
	}, time.Second*5, time.Millisecond, "expected all messages to be deleted")

	publish("bucket2", "messageID2", "key2", "key3")
	publish("bucket3", "messageID3", "key4")
	var messages [][]byte
	for range 3 {
		select {
		case <-ctx.Done():
			cancel()
			return
		case err := <-errC:
			require.Fail(t, "unexpected error", err)
		case msg := <-eventsC:
			messages = append(messages, msg.payload)
		}
	}

	require.Len(t, messages, 3)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.ElementsMatch(t, []string{"messageID1", "messageID2", "messageID3"}, fakeSQSQueue.getDeletedMessages())
	}, time.Second*5, time.Millisecond, "expected all messages to be deleted")

	// Simulate a failure to get an object from S3 if only one key fails.
	//The message shouldn't be deleted from the queue.
	publish("bucket4", "messageID4", "key1", "key2", "key3")

	// Check that the files were downloaded.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for _, file := range []string{"bucket4/key1", "bucket4/key2", "bucket4/key3"} {
			assert.Contains(t, fakeS3Bucket.getDownloadedFiles(), file)
		}
	}, time.Second*5, time.Millisecond, "expected all files to be downloaded")

	require.Never(t, func() bool {
		// Check that the message is not deleted from the queue.
		return slices.Contains(fakeSQSQueue.getDeletedMessages(), "messageID4")
	}, time.Second*2, time.Millisecond, "expected message to not be deleted from the queue")

	// Clean up the goroutine.
	cancel()
	wg.Wait()
}

// fakeQueue is used to fake SNS+SQS combination on AWS.
type fakeQueue struct {
	// publishErrors is chain of error returns on Publish method.
	// Errors are returned from start to end and removed, one-by-one, on each
	// invocation of the Publish method.
	// If the slice is empty, Publish runs normally.
	publishErrors   []error
	mu              sync.Mutex
	msgs            []fakeQueueMessage
	deletedMessages []string
}

type fakeQueueMessage struct {
	payload   string
	messageID string
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{}
}

func (f *fakeQueue) Publish(ctx context.Context, payload sqsFileEvent, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.publishErrors) > 0 {
		err := f.publishErrors[0]
		f.publishErrors = f.publishErrors[1:]
		return err
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return trace.Wrap(err)
	}
	f.msgs = append(f.msgs, fakeQueueMessage{
		payload:   string(jsonBody),
		messageID: id,
	})
	return nil
}

func (f *fakeQueue) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	msgs := f.dequeue()
	if len(msgs) == 0 {
		return &sqs.ReceiveMessageOutput{}, nil
	}
	out := make([]sqstypes.Message, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, sqstypes.Message{
			Body:          &msg.payload,
			ReceiptHandle: aws.String(msg.messageID),
		})
	}
	return &sqs.ReceiveMessageOutput{
		Messages: out,
	}, nil
}

func (f *fakeQueue) DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedMessages = append(f.deletedMessages, *params.ReceiptHandle)
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *fakeQueue) getDeletedMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.deletedMessages)
}

func (f *fakeQueue) dequeue() []fakeQueueMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	batchSize := 10
	if len(f.msgs) == 0 {
		return nil
	}
	if len(f.msgs) < batchSize {
		batchSize = len(f.msgs)
	}
	items := f.msgs[:batchSize]
	f.msgs = f.msgs[batchSize:]
	return items
}

type fakeS3Bucket struct {
	filesWithErr    map[string]struct{}
	mu              sync.Mutex
	downloadedFiles []string
}

func newFakeS3Bucket(filesWithErr ...string) *fakeS3Bucket {
	fileErrsMap := make(map[string]struct{}, len(filesWithErr))
	for _, fileErr := range filesWithErr {
		fileErrsMap[fileErr] = struct{}{}
	}
	return &fakeS3Bucket{
		filesWithErr: fileErrsMap,
	}
}

func (f *fakeS3Bucket) getDownloadedFiles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.downloadedFiles)
}

func (f *fakeS3Bucket) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file := filepath.Join(*params.Bucket, *params.Key)
	f.downloadedFiles = append(f.downloadedFiles, file)
	if _, ok := f.filesWithErr[file]; ok {
		return nil, trace.BadParameter("fake error")
	}
	b := io.NopCloser(bytes.NewBufferString(file))
	return &s3.GetObjectOutput{
		Body: b,
	}, nil
}
