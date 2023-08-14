/*
Copyright 2023 Gravitational, Inc.

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

package model

import (
	"fmt"
)

// invalidOutputError represents an error caused by the output of an LLM.
// These may be used automatically by the agent loop to attempt to correct an output until it is valid.
type invalidOutputError struct {
	coarse string
	detail string
}

// newInvalidOutputErrorWithParseError creates a new invalidOutputError assuming a JSON parse error.
func newInvalidOutputErrorWithParseError(err error) *invalidOutputError {
	return &invalidOutputError{
		coarse: "json parse error",
		detail: err.Error(),
	}
}

// Error returns a string representation of the error. This is used to satisfy the error interface.
func (o *invalidOutputError) Error() string {
	return fmt.Sprintf("%v: %v", o.coarse, o.detail)
}
