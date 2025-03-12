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

package aws

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	managerv2 "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
)

// ConvertS3Error wraps S3 error and returns trace equivalent
// It works on both sdk v1 and v2.
func ConvertS3Error(err error) error {
	if err == nil {
		return nil
	}

	// SDK v1 errors:
	var rerr awserr.RequestFailure
	if errors.As(err, &rerr) && rerr.StatusCode() == http.StatusForbidden {
		return trace.AccessDenied("%s", rerr.Message())
	}

	var aerr awserr.Error
	if errors.As(err, &aerr) {
		switch aerr.Code() {
		case s3.ErrCodeNoSuchKey, s3.ErrCodeNoSuchBucket, s3.ErrCodeNoSuchUpload, "NotFound":
			return trace.NotFound("%s", aerr)
		case s3.ErrCodeBucketAlreadyExists, s3.ErrCodeBucketAlreadyOwnedByYou:
			return trace.AlreadyExists("%s", aerr)
		default:
			return trace.BadParameter("%s", aerr)
		}
	}

	// SDK v2 errors:
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return trace.NotFound("%s", noSuchKey)
	}
	var noSuchBucket *s3types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return trace.NotFound("%s", noSuchBucket)
	}
	var noSuchUpload *s3types.NoSuchUpload
	if errors.As(err, &noSuchUpload) {
		return trace.NotFound("%s", noSuchUpload)
	}
	var bucketAlreadyExists *s3types.BucketAlreadyExists
	if errors.As(err, &bucketAlreadyExists) {
		return trace.AlreadyExists("%s", bucketAlreadyExists.Error())
	}
	var bucketAlreadyOwned *s3types.BucketAlreadyOwnedByYou
	if errors.As(err, &bucketAlreadyOwned) {
		return trace.AlreadyExists("%s", bucketAlreadyOwned.Error())
	}
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return trace.NotFound("%s", notFound)
	}

	var opError *smithy.OperationError
	if errors.As(err, &opError) && strings.Contains(opError.Err.Error(), "FIPS") {
		return trace.BadParameter("%s", opError)
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
func NewS3V2FileWriter(ctx context.Context, s3Client managerv2.UploadAPIClient, bucket, key string, uploaderOptions []func(*managerv2.Uploader), putObjectInputOptions ...func(*s3v2.PutObjectInput)) (*s3V2FileWriter, error) {
	uploader := managerv2.NewUploader(s3Client, uploaderOptions...)
	pr, pw := io.Pipe()

	uploadParams := &s3v2.PutObjectInput{
		Bucket: awsv2.String(bucket),
		Key:    awsv2.String(key),
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

// CreateBucketConfiguration creates the default CreateBucketConfiguration.
func CreateBucketConfiguration(region string) *s3types.CreateBucketConfiguration {
	// No location constraint wanted for us-east-1 because it is the default and
	// AWS has decided, in all their infinite wisdom, that the CreateBucket API
	// should fail if you explicitly pass the default location constraint.
	if region == "us-east-1" {
		return nil
	}
	return &s3types.CreateBucketConfiguration{
		LocationConstraint: s3types.BucketLocationConstraint(region),
	}
}
