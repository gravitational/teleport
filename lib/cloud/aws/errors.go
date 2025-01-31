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
	"github.com/gravitational/trace"
)

// ConvertRequestFailureError converts AWS SDK v2 errors to trace errors.
// If the provided error is not a [awshttp.ResponseError] it returns the error
// without modifying it.
func ConvertRequestFailureError(err error) error {
	var re *awshttp.ResponseError
	if errors.As(err, &re) {
		return convertRequestFailureErrorFromStatusCode(re.HTTPStatusCode(), re)
	}
	return err
}

var (
	ecsClusterNotFoundException *ecstypes.ClusterNotFoundException
)

func convertRequestFailureErrorFromStatusCode(statusCode int, requestErr error) error {
	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied(requestErr.Error())
	case http.StatusConflict:
		return trace.AlreadyExists(requestErr.Error())
	case http.StatusNotFound:
		return trace.NotFound(requestErr.Error())
	case http.StatusBadRequest:
		// Some services like memorydb, redshiftserverless may return 400 with
		// "AccessDeniedException" instead of 403.
		if strings.Contains(requestErr.Error(), "AccessDeniedException") {
			return trace.AccessDenied(requestErr.Error())
		}

		if strings.Contains(requestErr.Error(), ecsClusterNotFoundException.ErrorCode()) {
			return trace.NotFound(requestErr.Error())
		}
	}

	return requestErr // Return unmodified.
}

// ConvertIAMError converts common errors from IAM clients to trace errors.
func ConvertIAMError(err error) error {
	if err == nil {
		return nil
	}

	var unmodifiableEntityErr *iamtypes.UnmodifiableEntityException
	if errors.As(err, &unmodifiableEntityErr) {
		return trace.AccessDenied(*unmodifiableEntityErr.Message)
	}

	var entityExistsError *iamtypes.EntityAlreadyExistsException
	if errors.As(err, &entityExistsError) {
		return trace.AlreadyExists(*entityExistsError.Message)
	}

	var entityNotFound *iamtypes.NoSuchEntityException
	if errors.As(err, &entityNotFound) {
		return trace.NotFound(*entityNotFound.Message)
	}

	var malformedPolicyDocument *iamtypes.MalformedPolicyDocumentException
	if errors.As(err, &malformedPolicyDocument) {
		return trace.BadParameter(*malformedPolicyDocument.Message)
	}

	return ConvertRequestFailureError(err)
}
