/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
)

// ConvertResponseError converts `error` into Azure Response error.
// to trace error. If the provided error is not a `ResponseError` it returns.
// the error without modifying it.
func ConvertResponseError(err error) error {
	if err == nil {
		return nil
	}

	switch v := err.(type) {
	case *azcore.ResponseError:
		switch v.StatusCode {
		case http.StatusForbidden:
			return trace.AccessDenied(v.Error())
		case http.StatusConflict:
			return trace.AlreadyExists(v.Error())
		case http.StatusNotFound:
			return trace.NotFound(v.Error())
		}

	case *azidentity.AuthenticationFailedError:
		return trace.AccessDenied(v.Error())

	}
	return err // Return unmodified.
}
