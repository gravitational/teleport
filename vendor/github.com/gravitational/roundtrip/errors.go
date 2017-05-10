/*
Copyright 2015 Gravitational, Inc.

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

package roundtrip

import (
	"net/url"

	"github.com/gravitational/trace"
)

// AccessDeniedError is returned whenever access is denied
type AccessDeniedError struct {
	trace.Traces
	Message string `json:"message"`
}

// Error returns user-friendly description of this error
func (a *AccessDeniedError) Error() string {
	return a.Message
}

// IsAccessDeniedError indicates that this error belongs
// to access denied class of errors
func (a *AccessDeniedError) IsAccessDeniedError() bool {
	return true
}

// ParameterError indicates error in parameter
type ParameterError struct {
	trace.Traces
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Error returns user-friendly description of this error
func (a *ParameterError) Error() string {
	return a.Message
}

// IsParameterError indicates that this error belongs
// to class of errors related to bad parameters
func (a *ParameterError) IsParameterError() bool {
	return true
}

// ConvertResponse converts http error to internal error type
// based on HTTP response code and HTTP body contents
func ConvertResponse(re *Response, err error) (*Response, error) {
	if err != nil {
		if uerr, ok := err.(*url.Error); ok && uerr != nil && uerr.Err != nil {
			return nil, trace.Wrap(uerr.Err)
		}
		return nil, trace.Wrap(err)
	}
	return re, trace.ReadError(re.Code(), re.Bytes())
}
