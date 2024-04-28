/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package s3sessions

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestAbortsCompletedUploads(t *testing.T) {
	t.Setenv("TELEPORT_ABORT_COMPLETED_UPLOADS", "yes")

	client := &mockS3{}
	h := &Handler{
		client: client,
		Entry:  logrus.NewEntry(&logrus.Logger{Out: io.Discard}),
	}

	upload := events.StreamUpload{
		ID:        "upload-id",
		SessionID: session.NewID(),
		Initiated: time.Now(),
	}
	parts := []events.StreamPart{
		{Number: 1, ETag: "etag1"},
	}

	ctx := context.Background()
	require.NoError(t, h.CompleteUpload(ctx, upload, parts))
	require.Len(t, client.abortedUploadIDs, 1, "1 upload should have been aborted")
	require.Equal(t, upload.ID, client.abortedUploadIDs[0])
}

type mockS3 struct {
	s3iface.S3API

	abortedUploadIDs []string
}

func (m *mockS3) CompleteMultipartUploadWithContext(
	ctx aws.Context,
	in *s3.CompleteMultipartUploadInput,
	opts ...request.Option,
) (*s3.CompleteMultipartUploadOutput, error) {
	return nil, awserr.New(s3.ErrCodeNoSuchUpload, "upload not found", errors.New("not found"))
}

func (m *mockS3) GetObjectWithContext(
	ctx aws.Context,
	in *s3.GetObjectInput,
	opts ...request.Option,
) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{
		LastModified: aws.Time(time.Now().Add(-1 * time.Hour)),
	}, nil
}

func (m *mockS3) AbortMultipartUploadWithContext(
	ctx aws.Context,
	in *s3.AbortMultipartUploadInput,
	opts ...request.Option,
) (*s3.AbortMultipartUploadOutput, error) {
	m.abortedUploadIDs = append(m.abortedUploadIDs, aws.StringValue(in.UploadId))
	return nil, nil
}
