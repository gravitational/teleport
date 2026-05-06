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
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func init() {
	// Override maxS3BasedSize so we don't have to allocate 2GiB to test it.
	// Do this in init to avoid any race.
	maxS3BasedSize = (1024-10)*1024 * 4
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
			maxDirectMessageSize: (1024-10)*1024,
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
			maxDirectMessageSize: (1024-10)*1024,
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

func (m mockUploader) UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	return &transfermanager.UploadObjectOutput{}, nil
}
