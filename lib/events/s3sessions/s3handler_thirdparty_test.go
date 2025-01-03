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

package s3sessions

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
)

// TestThirdpartyStreams tests various streaming upload scenarios
// implemented by third party backends using fake backend
func TestThirdpartyStreams(t *testing.T) {
	t.Parallel()

	var timeSource gofakes3.TimeSource
	backend := s3mem.New(s3mem.WithTimeSource(timeSource))
	faker := gofakes3.New(backend, gofakes3.WithLogger(gofakes3.GlobalLog()))
	server := httptest.NewServer(faker.Server())
	defer server.Close()

	bucketName := fmt.Sprintf("teleport-test-%v", uuid.New().String())

	config := aws.Config{
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, nil
		}),
		Region:       "us-west-1",
		BaseEndpoint: aws.String(server.URL),
	}

	s3Client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Create the bucket.
	_, err := s3Client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	require.NoError(t, err)

	handler, err := NewHandler(context.Background(), Config{
		Region:                      "us-west-1",
		Path:                        "/test/",
		Bucket:                      bucketName,
		Endpoint:                    server.URL,
		DisableServerSideEncryption: true,
		CredentialsProvider: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, nil
		}),
	})
	require.NoError(t, err)

	defer func() {
		if err := handler.deleteBucket(context.Background()); err != nil {
			t.Fatalf("Failed to delete bucket: %#v", trace.DebugReport(err))
		}
	}()

	// Stream with handler and many parts
	t.Run("StreamManyParts", func(t *testing.T) {
		test.Stream(t, handler)
	})
	t.Run("StreamWithPadding", func(t *testing.T) {
		test.StreamWithPadding(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("StreamEmpty", func(t *testing.T) {
		test.StreamEmpty(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
