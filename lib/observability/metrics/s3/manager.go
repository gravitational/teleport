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
	"io"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

type UploadAPIMetrics struct {
	s3manageriface.UploaderAPI
}

func NewUploadAPIMetrics(api s3manageriface.UploaderAPI) (*UploadAPIMetrics, error) {
	if err := metrics.RegisterPrometheusCollectors(s3Collectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	return &UploadAPIMetrics{UploaderAPI: api}, nil
}

func (m *UploadAPIMetrics) UploadWithContext(ctx context.Context, input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	start := time.Now()
	output, err := m.UploaderAPI.UploadWithContext(ctx, input, opts...)

	recordMetrics("upload", err, time.Since(start).Seconds())

	return output, err
}

type DownloadAPIMetrics struct {
	s3manageriface.DownloaderAPI
}

func NewDownloadAPIMetrics(api s3manageriface.DownloaderAPI) (*DownloadAPIMetrics, error) {
	if err := metrics.RegisterPrometheusCollectors(s3Collectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	return &DownloadAPIMetrics{DownloaderAPI: api}, nil
}

func (m *DownloadAPIMetrics) DownloadWithContext(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, opts ...func(*s3manager.Downloader)) (int64, error) {
	start := time.Now()
	n, err := m.DownloaderAPI.DownloadWithContext(ctx, w, input, opts...)

	recordMetrics("download", err, time.Since(start).Seconds())

	return n, err
}
