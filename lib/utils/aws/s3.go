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
	"errors"

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
	return err
}
