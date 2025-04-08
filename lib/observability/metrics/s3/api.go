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

package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

type APIMetrics struct {
	s3iface.S3API
}

func NewAPIMetrics(api s3iface.S3API) (*APIMetrics, error) {
	if err := metrics.RegisterPrometheusCollectors(s3Collectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	return &APIMetrics{S3API: api}, nil
}

func (m *APIMetrics) ListObjectVersionsPagesWithContext(ctx context.Context, input *s3.ListObjectVersionsInput, f func(*s3.ListObjectVersionsOutput, bool) bool, opts ...request.Option) error {
	start := time.Now()
	err := m.S3API.ListObjectVersionsPagesWithContext(ctx, input, f, opts...)

	recordMetrics("list_object_versions_pages", err, time.Since(start).Seconds())
	return err
}

func (m *APIMetrics) ListObjectVersionsWithContext(ctx context.Context, input *s3.ListObjectVersionsInput, opts ...request.Option) (*s3.ListObjectVersionsOutput, error) {
	start := time.Now()
	output, err := m.S3API.ListObjectVersionsWithContext(ctx, input, opts...)

	recordMetrics("list_object_versions", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) DeleteObjectWithContext(ctx context.Context, input *s3.DeleteObjectInput, opts ...request.Option) (*s3.DeleteObjectOutput, error) {
	start := time.Now()
	output, err := m.S3API.DeleteObjectWithContext(ctx, input, opts...)

	recordMetrics("delete_object", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) DeleteBucketWithContext(ctx context.Context, input *s3.DeleteBucketInput, opts ...request.Option) (*s3.DeleteBucketOutput, error) {
	start := time.Now()
	output, err := m.S3API.DeleteBucketWithContext(ctx, input, opts...)

	recordMetrics("delete_bucket", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) HeadBucketWithContext(ctx context.Context, input *s3.HeadBucketInput, opts ...request.Option) (*s3.HeadBucketOutput, error) {
	start := time.Now()
	output, err := m.S3API.HeadBucketWithContext(ctx, input, opts...)

	recordMetrics("head_bucket", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) CreateBucketWithContext(ctx context.Context, input *s3.CreateBucketInput, opts ...request.Option) (*s3.CreateBucketOutput, error) {
	start := time.Now()
	output, err := m.S3API.CreateBucketWithContext(ctx, input, opts...)

	recordMetrics("create_bucket", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) PutBucketVersioningWithContext(ctx context.Context, input *s3.PutBucketVersioningInput, opts ...request.Option) (*s3.PutBucketVersioningOutput, error) {
	start := time.Now()
	output, err := m.S3API.PutBucketVersioningWithContext(ctx, input, opts...)

	recordMetrics("put_bucket_versioning", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) PutBucketEncryptionWithContext(ctx context.Context, input *s3.PutBucketEncryptionInput, opts ...request.Option) (*s3.PutBucketEncryptionOutput, error) {
	start := time.Now()
	output, err := m.S3API.PutBucketEncryptionWithContext(ctx, input, opts...)

	recordMetrics("put_bucket_encryption", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) CreateMultipartUploadWithContext(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...request.Option) (*s3.CreateMultipartUploadOutput, error) {
	start := time.Now()
	output, err := m.S3API.CreateMultipartUploadWithContext(ctx, input, opts...)

	recordMetrics("create_multipart_upload", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) UploadPartWithContext(ctx context.Context, input *s3.UploadPartInput, opts ...request.Option) (*s3.UploadPartOutput, error) {
	start := time.Now()
	output, err := m.S3API.UploadPartWithContext(ctx, input, opts...)

	recordMetrics("upload_part", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) AbortMultipartUploadWithContext(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...request.Option) (*s3.AbortMultipartUploadOutput, error) {
	start := time.Now()
	output, err := m.S3API.AbortMultipartUploadWithContext(ctx, input, opts...)

	recordMetrics("abort_multipart_upload", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) CompleteMultipartUploadWithContext(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...request.Option) (*s3.CompleteMultipartUploadOutput, error) {
	start := time.Now()
	output, err := m.S3API.CompleteMultipartUploadWithContext(ctx, input, opts...)

	recordMetrics("complete_multipart_upload", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) ListPartsWithContext(ctx context.Context, input *s3.ListPartsInput, opts ...request.Option) (*s3.ListPartsOutput, error) {
	start := time.Now()
	output, err := m.S3API.ListPartsWithContext(ctx, input, opts...)

	recordMetrics("list_parts", err, time.Since(start).Seconds())
	return output, err
}

func (m *APIMetrics) ListMultipartUploadsWithContext(ctx context.Context, input *s3.ListMultipartUploadsInput, opts ...request.Option) (*s3.ListMultipartUploadsOutput, error) {
	start := time.Now()
	output, err := m.S3API.ListMultipartUploadsWithContext(ctx, input, opts...)

	recordMetrics("list_multipart_uploads", err, time.Since(start).Seconds())
	return output, err
}
