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
	"net/http"

	"github.com/aws/aws-sdk-go/aws/awserr"
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
	}

	return err // Return unmodified.
}
