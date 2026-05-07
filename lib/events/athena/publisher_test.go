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
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type mockSQSGetQueueAttributer struct {
	attrs map[string]string
	err   error
}

func (m *mockSQSGetQueueAttributer) GetQueueAttributes(_ context.Context, _ *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &sqs.GetQueueAttributesOutput{Attributes: m.attrs}, nil
}

func init() {
	// Override maxS3BasedSize so we don't have to allocate 2GiB to test it.
	// Do this in init to avoid any race.
	maxS3BasedSize = (1024 - 10) * 1024 * 4
}

// TODO(tobiaszheller): Those UT just cover basic stuff. When we will have consumer
// there will be UT which will cover whole flow of message with encoding/decoding.
func Test_EmitAuditEvent(t *testing.T) {
	veryLongString := strings.Repeat("t", maxS3BasedSize+1)
	tests := []struct {
		name                 string
		in                   apievents.AuditEvent
		publishErrors        []error
		uploader             s3uploader
		maxDirectMessageSize int
		wantCheck            func(t *testing.T, out []fakeQueueMessage)
		wantErrorMsg         string
	}{
		{
			name: "valid publish",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
				},
			},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.False(t, out[0].s3Based)
			},
		},
		{
			name: "should succeed after some retries",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
				},
			},
			publishErrors: []error{
				errors.New("error1"), errors.New("error2"),
			},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.False(t, out[0].s3Based)
			},
		},
		{
			name: "big message via s3",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
					Code: strings.Repeat("d", 2*maxSNSDirectMessageSize),
				},
			},
			uploader: mockUploader{},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.True(t, out[0].s3Based)
			},
		},
		{
			// A message between the SNS and SQS direct-send limits should go
			// directly to the queue (no S3 hop) when MaxDirectMessageSize is
			// the larger SQS threshold.
			name: "medium message sent directly with SQS limit",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
					Code: strings.Repeat("d", 2*maxSNSDirectMessageSize),
				},
			},
			maxDirectMessageSize: (1024 - 10) * 1024,
			uploader:             mockUploader{},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.False(t, out[0].s3Based)
			},
		},
		{
			// A message larger than the SQS direct-send limit but smaller than
			// maxS3BasedSize must fall back to S3.
			name: "large message exceeds SQS limit falls back to S3",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
					Code: strings.Repeat("d", 2*(1024-10)*1024),
				},
			},
			maxDirectMessageSize: (1024 - 10) * 1024,
			uploader:             mockUploader{},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.True(t, out[0].s3Based)
			},
		},
		{
			name: "very big untrimmable event",
			in: &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
					Code: veryLongString,
				},
			},
			uploader:     mockUploader{},
			wantErrorMsg: "message too large to publish",
		},
		{
			name: "very big trimmable event",
			in: &apievents.DatabaseSessionQuery{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
				},
				DatabaseQuery: veryLongString,
			},
			uploader: mockUploader{},
			wantCheck: func(t *testing.T, out []fakeQueueMessage) {
				require.Len(t, out, 1)
				require.True(t, out[0].s3Based)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fq := newFakeQueue()
			p := NewPublisher(PublisherConfig{
				MessagePublisher:     fq,
				Uploader:             tt.uploader,
				MaxDirectMessageSize: tt.maxDirectMessageSize,
			})
			err := p.EmitAuditEvent(context.Background(), tt.in)
			if tt.wantErrorMsg != "" {
				require.ErrorContains(t, err, tt.wantErrorMsg)
				return
			}
			require.NoError(t, err)
			out := fq.dequeue()
			tt.wantCheck(t, out)
		})
	}
}

type mockUploader struct{}

func (m mockUploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	return &manager.UploadOutput{}, nil
}

func Test_sqsMaxDirectMessageSize(t *testing.T) {
	const overhead = 10 * 1024
	const queueURL = "https://sqs.us-east-1.amazonaws.com/123456789/test-queue"

	maxSizeAttr := func(v int) map[string]string {
		return map[string]string{string(sqstypes.QueueAttributeNameMaximumMessageSize): strconv.Itoa(v)}
	}

	tests := []struct {
		name     string
		mock     *mockSQSGetQueueAttributer
		wantSize int
	}{
		{
			name:     "returns attribute value minus overhead",
			mock:     &mockSQSGetQueueAttributer{attrs: maxSizeAttr(262144)},
			wantSize: 262144 - overhead,
		},
		{
			name:     "falls back to SNS limit when API call fails",
			mock:     &mockSQSGetQueueAttributer{err: errors.New("connection refused")},
			wantSize: maxSNSDirectMessageSize,
		},
		{
			name:     "falls back to SNS limit when attribute is missing",
			mock:     &mockSQSGetQueueAttributer{attrs: map[string]string{}},
			wantSize: maxSNSDirectMessageSize,
		},
		{
			name:     "falls back to SNS limit when value is not a number",
			mock:     &mockSQSGetQueueAttributer{attrs: map[string]string{string(sqstypes.QueueAttributeNameMaximumMessageSize): "not-a-number"}},
			wantSize: maxSNSDirectMessageSize,
		},
		{
			name:     "falls back to SNS limit when value is too small to subtract overhead",
			mock:     &mockSQSGetQueueAttributer{attrs: maxSizeAttr(overhead)},
			wantSize: maxSNSDirectMessageSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqsMaxDirectMessageSize(context.Background(), tt.mock, queueURL, slog.Default())
			assert.Equal(t, tt.wantSize, got)
		})
	}
}
