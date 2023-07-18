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

package aws

import (
	"context"
	"errors"
	"io"
	"net/http"

	awsV2 "github.com/aws/aws-sdk-go-v2/aws"
	managerV2 "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gravitational/trace"
)

// ConvertS3Error wraps S3 error and returns trace equivalent
// It works on both sdk v1 and v2.
func ConvertS3Error(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}

	// SDK v1 errors:
	if rerr, ok := err.(awserr.RequestFailure); ok && rerr.StatusCode() == http.StatusForbidden {
		return trace.AccessDenied(rerr.Message())
	}

	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case s3.ErrCodeNoSuchKey, s3.ErrCodeNoSuchBucket, s3.ErrCodeNoSuchUpload, "NotFound":
			return trace.NotFound(aerr.Error(), args...)
		case s3.ErrCodeBucketAlreadyExists, s3.ErrCodeBucketAlreadyOwnedByYou:
			return trace.AlreadyExists(aerr.Error(), args...)
		default:
			return trace.BadParameter(aerr.Error(), args...)
		}
	}

	// SDK v2 errors:
	var noSuchKey *s3Types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return trace.NotFound(noSuchKey.Error(), args...)
	}
	var noSuchBucket *s3Types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return trace.NotFound(noSuchBucket.Error(), args...)
	}
	var noSuchUpload *s3Types.NoSuchUpload
	if errors.As(err, &noSuchUpload) {
		return trace.NotFound(noSuchUpload.Error(), args...)
	}
	var bucketAlreadyExists *s3Types.BucketAlreadyExists
	if errors.As(err, &bucketAlreadyExists) {
		return trace.AlreadyExists(bucketAlreadyExists.Error(), args...)
	}
	var bucketAlreadyOwned *s3Types.BucketAlreadyOwnedByYou
	if errors.As(err, &bucketAlreadyOwned) {
		return trace.AlreadyExists(bucketAlreadyOwned.Error(), args...)
	}
	var notFound *s3Types.NotFound
	if errors.As(err, &notFound) {
		return trace.NotFound(notFound.Error(), args...)
	}

	return err
}

// s3V2FileWriter can be used to upload data to s3 via io.WriteCloser interface.
type s3V2FileWriter struct {
	// uploadFinisherErrChan is used to wait for completed upload as well as
	// sending error message.
	uploadFinisherErrChan <-chan error
	pipeWriter            *io.PipeWriter
	pipeReader            *io.PipeReader
}

// NewS3V2FileWriter created s3V2FileWriter. Close method on writer should be called
// to make sure that reader has finished.
func NewS3V2FileWriter(ctx context.Context, s3Client managerV2.UploadAPIClient, bucket, key string, uploaderOptions []func(*managerV2.Uploader), putObjectInputOptions ...func(*s3v2.PutObjectInput)) (*s3V2FileWriter, error) {
	uploader := managerV2.NewUploader(s3Client, uploaderOptions...)
	pr, pw := io.Pipe()

	uploadParams := &s3v2.PutObjectInput{
		Bucket: awsV2.String(bucket),
		Key:    awsV2.String(key),
		Body:   pr,
	}

	for _, f := range putObjectInputOptions {
		f(uploadParams)
	}
	uploadFinisherErrChan := make(chan error)
	go func() {
		defer close(uploadFinisherErrChan)
		_, err := uploader.Upload(ctx, uploadParams)
		if err != nil {
			pr.CloseWithError(err)
		}
		uploadFinisherErrChan <- trace.Wrap(err)
	}()

	return &s3V2FileWriter{
		uploadFinisherErrChan: uploadFinisherErrChan,
		pipeWriter:            pw,
		pipeReader:            pr,
	}, nil
}

// Write bytes from in to the connected pipe.
func (s *s3V2FileWriter) Write(in []byte) (int, error) {
	bytesWritten, writeError := s.pipeWriter.Write(in)
	if writeError != nil {
		s.pipeWriter.CloseWithError(writeError)
		return bytesWritten, writeError
	}
	return bytesWritten, nil
}

// Close signals write completion and cleans up any
// open streams. Will block until pending uploads are complete.
func (s *s3V2FileWriter) Close() error {
	wCloseErr := s.pipeWriter.Close()
	// wait for reader to finish, it will be triggered by writer.Close
	readerErr := <-s.uploadFinisherErrChan
	rCloseErr := s.pipeReader.Close()
	return trace.Wrap(trace.NewAggregate(wCloseErr, readerErr, rCloseErr))
}
