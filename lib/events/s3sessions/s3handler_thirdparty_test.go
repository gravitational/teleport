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
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/session"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
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

	// Providers without object versioning, such as Cloudflare R2, reject
	// ListObjectVersions and PutBucketVersioning. With DisableVersioning set
	// the handler must not call either of them, and it must complete uploads
	// conditionally so that a recording is never replaced.
	t.Run("DisableVersioning", func(t *testing.T) {
		unversioned := *handler
		unversioned.DisableVersioning = true
		unversioned.client = unversionedClient{handler.client}

		t.Run("Stream", func(t *testing.T) {
			test.Stream(t, &unversioned)
		})
		t.Run("StreamWithPadding", func(t *testing.T) {
			test.StreamWithPadding(t, &unversioned)
		})
		t.Run("UploadDownload", func(t *testing.T) {
			test.UploadDownload(t, &unversioned)
		})
		t.Run("UploadDownloadMetadata", func(t *testing.T) {
			test.UploadDownloadMetadata(t, &unversioned)
		})
		t.Run("UploadDownloadThumbnail", func(t *testing.T) {
			test.UploadDownloadThumbnail(t, &unversioned)
		})
		t.Run("DownloadNotFound", func(t *testing.T) {
			test.DownloadNotFound(t, &unversioned)
		})
		t.Run("StreamEmpty", func(t *testing.T) {
			test.StreamEmpty(t, &unversioned)
		})
		t.Run("CompleteUploadDoesNotOverwrite", func(t *testing.T) {
			ctx := t.Context()
			sid := session.NewID()
			completeUpload := func(content string) {
				upload, err := unversioned.CreateUpload(ctx, sid)
				require.NoError(t, err)
				part, err := unversioned.UploadPart(ctx, *upload, 1, strings.NewReader(content))
				require.NoError(t, err)
				require.NoError(t, unversioned.CompleteUpload(ctx, *upload, []events.StreamPart{*part}))
			}
			completeUpload("first")
			// A second upload for the same session, such as one the upload
			// completer finishes after the stream was resumed, must be aborted
			// instead of replacing the recording.
			completeUpload("second")

			rc, err := unversioned.StreamSessionRecording(ctx, sid)
			require.NoError(t, err)
			defer rc.Close()
			data, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.Equal(t, "first", string(data))

			uploads, err := unversioned.ListUploads(ctx)
			require.NoError(t, err)
			for _, upload := range uploads {
				require.NotEqual(t, sid, upload.SessionID, "the second upload was not aborted")
			}
		})
		t.Run("DownloadFailsAfterOverwrite", func(t *testing.T) {
			ctx := t.Context()
			sid := session.NewID()
			_, err := unversioned.UploadPendingSummary(ctx, sid, strings.NewReader("pending"))
			require.NoError(t, err)
			rc, err := unversioned.StreamSessionSummary(ctx, sid)
			require.NoError(t, err)
			defer rc.Close()
			// The range reads are pinned to the object HeadObject saw, so an
			// overwrite in between fails the download instead of mixing the
			// two objects.
			_, err = unversioned.UploadPendingSummary(ctx, sid, strings.NewReader("replaced"))
			require.NoError(t, err)
			_, err = io.ReadAll(rc)
			require.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)
		})
		t.Run("EnsureBucket", func(t *testing.T) {
			created := unversioned
			created.Bucket = fmt.Sprintf("teleport-test-%v", uuid.New().String())
			require.NoError(t, created.ensureBucket(t.Context()))
			t.Cleanup(func() {
				_, err := handler.client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
					Bucket: aws.String(created.Bucket),
				})
				require.NoError(t, err)
			})
			_, err := handler.client.HeadBucket(t.Context(), &s3.HeadBucketInput{
				Bucket: aws.String(created.Bucket),
			})
			require.NoError(t, err)
		})
	})

	// Without DisableVersioning the same client must fail, which shows that the
	// subtests above pass only because the handler skips the versioning calls.
	t.Run("VersioningRequired", func(t *testing.T) {
		versioned := *handler
		versioned.client = unversionedClient{handler.client}
		_, err := versioned.StreamSessionRecording(t.Context(), session.NewID())
		var apiErr smithy.APIError
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, "NotImplemented", apiErr.ErrorCode())
	})
}

// unversionedClient mimics an S3-compatible service without object
// versioning, such as Cloudflare R2: it rejects the versioning calls and
// honors If-None-Match on CompleteMultipartUpload and If-Match on GetObject,
// which gofakes3 ignores.
type unversionedClient struct {
	s3Client
}

func (unversionedClient) ListObjectVersions(context.Context, *s3.ListObjectVersionsInput, ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	return nil, notImplementedError("ListObjectVersions")
}

func (unversionedClient) PutBucketVersioning(context.Context, *s3.PutBucketVersioningInput, ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	return nil, notImplementedError("PutBucketVersioning")
}

func (c unversionedClient) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	if aws.ToString(params.IfNoneMatch) == "*" {
		_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: params.Bucket, Key: params.Key})
		err = awsutils.ConvertS3Error(err)
		switch {
		case err == nil:
			// R2 treats an upload whose completion failed a precondition as
			// aborted, so the handler's own abort gets NoSuchUpload.
			if _, err := c.s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket: params.Bucket, Key: params.Key, UploadId: params.UploadId,
			}); err != nil {
				return nil, err
			}
			return nil, &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "object already exists"}
		case !trace.IsNotFound(err):
			return nil, err
		}
	}
	return c.s3Client.CompleteMultipartUpload(ctx, params, optFns...)
}

func (c unversionedClient) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if params.IfMatch != nil {
		head, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: params.Bucket, Key: params.Key})
		if err != nil {
			return nil, err
		}
		if aws.ToString(head.ETag) != aws.ToString(params.IfMatch) {
			return nil, &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "object has changed"}
		}
	}
	return c.s3Client.GetObject(ctx, params, optFns...)
}

// notImplementedError is the error the AWS SDK returns for the HTTP 501
// response R2 sends for an unimplemented operation.
func notImplementedError(operation string) error {
	return &smithy.GenericAPIError{Code: "NotImplemented", Message: operation + " is not implemented"}
}
