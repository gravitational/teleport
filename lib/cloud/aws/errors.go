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
	"errors"
	"net/http"
	"strings"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/gravitational/trace"
)

// ConvertRequestFailureError converts AWS SDK v2 errors to trace errors.
// If the provided error is not a [awshttp.ResponseError] it returns the error
// without modifying it.
func ConvertRequestFailureError(err error) error {
	if re, ok := errors.AsType[*awshttp.ResponseError](err); ok {
		return convertRequestFailureErrorFromStatusCode(re.HTTPStatusCode(), re)
	}
	return err
}

var (
	ecsClusterNotFoundException           *ecstypes.ClusterNotFoundException
	organizationsAccountNotFoundException *orgtypes.AccountNotFoundException
)

func convertRequestFailureErrorFromStatusCode(statusCode int, requestErr error) error {
	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied("%s", requestErr)
	case http.StatusConflict:
		return trace.AlreadyExists("%s", requestErr)
	case http.StatusNotFound:
		return trace.NotFound("%s", requestErr)
	case http.StatusBadRequest:
		// Some services like memorydb, redshiftserverless may return 400 with
		// "AccessDeniedException" instead of 403.
		if strings.Contains(requestErr.Error(), "AccessDeniedException") {
			return trace.AccessDenied("%s", requestErr)
		}

		if strings.Contains(requestErr.Error(), "TooManyRequestsException") {
			return trace.LimitExceeded("%s", requestErr)
		}

		if strings.Contains(requestErr.Error(), ecsClusterNotFoundException.ErrorCode()) {
			return trace.NotFound("%s", requestErr)
		}

		if strings.Contains(requestErr.Error(), organizationsAccountNotFoundException.ErrorCode()) {
			return trace.NotFound("%s", requestErr)
		}
	}

	return requestErr // Return unmodified.
}

// ConvertIAMError converts common errors from IAM clients to trace errors.
func ConvertIAMError(err error) error {
	if err == nil {
		return nil
	}

	if unmodifiableEntityErr, ok := errors.AsType[*iamtypes.UnmodifiableEntityException](err); ok {
		return trace.AccessDenied("%s", *unmodifiableEntityErr.Message)
	}

	if entityExistsError, ok := errors.AsType[*iamtypes.EntityAlreadyExistsException](err); ok {
		return trace.AlreadyExists("%s", *entityExistsError.Message)
	}

	if entityNotFound, ok := errors.AsType[*iamtypes.NoSuchEntityException](err); ok {
		return trace.NotFound("%s", *entityNotFound.Message)
	}

	if malformedPolicyDocument, ok := errors.AsType[*iamtypes.MalformedPolicyDocumentException](err); ok {
		return trace.BadParameter("%s", *malformedPolicyDocument.Message)
	}

	if limitExceededErr, ok := errors.AsType[*iamtypes.LimitExceededException](err); ok {
		return trace.LimitExceeded("%s", *limitExceededErr.Message)
	}

	return ConvertRequestFailureError(err)
}
