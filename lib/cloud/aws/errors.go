/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"errors"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/gravitational/trace"
)

// ConvertRequestFailureError converts `error` into AWS RequestFailure errors
// to trace errors. If the provided error is not an `RequestFailure` it returns
// the error without modifying it.
func ConvertRequestFailureError(err error) error {
	requestErr, ok := err.(awserr.RequestFailure)
	if !ok {
		return err
	}

	switch requestErr.StatusCode() {
	case http.StatusForbidden:
		return trace.AccessDenied(requestErr.Error())
	case http.StatusConflict:
		return trace.AlreadyExists(requestErr.Error())
	case http.StatusNotFound:
		return trace.NotFound(requestErr.Error())
	case http.StatusBadRequest:
		// Some services like memorydb, redshiftserverless may return 400 with
		// "AccessDeniedException" instead of 403.
		if strings.Contains(requestErr.Error(), redshiftserverless.ErrCodeAccessDeniedException) {
			return trace.AccessDenied(requestErr.Error())
		}
	}

	return err // Return unmodified.
}

// ConvertIAMError converts common errors from IAM clients to trace errors.
func ConvertIAMError(err error) error {
	// By error code.
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case iam.ErrCodeUnmodifiableEntityException:
			return trace.AccessDenied(awsErr.Error())

		case iam.ErrCodeNoSuchEntityException:
			return trace.NotFound(awsErr.Error())

		case iam.ErrCodeMalformedPolicyDocumentException,
			iam.ErrCodeInvalidInputException,
			iam.ErrCodeDeleteConflictException:
			return trace.BadParameter(awsErr.Error())

		case iam.ErrCodeLimitExceededException:
			return trace.LimitExceeded(awsErr.Error())
		}
	}

	// By status code.
	return ConvertRequestFailureError(err)
}

// parseMetadataClientError converts a failed instance metadata service call to a trace error.
func parseMetadataClientError(err error) error {
	var httpError interface{ HTTPStatusCode() int }
	if errors.As(err, &httpError) {
		return trace.ReadError(httpError.HTTPStatusCode(), nil)
	}
	return trace.Wrap(err)
}
