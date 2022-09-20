// Copyright 2022 Gravitational, Inc
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

package s3

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

type UploadAPIMetrics struct {
	s3manageriface.UploaderAPI
}

func NewUploadAPIMetrics(api s3manageriface.UploaderAPI) (*UploadAPIMetrics, error) {
	if err := utils.RegisterPrometheusCollectors(s3Collectors...); err != nil {
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
	if err := utils.RegisterPrometheusCollectors(s3Collectors...); err != nil {
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
