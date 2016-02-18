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
package teleport

import (
	"fmt"

	"github.com/gravitational/trace"
)

// AlreadyAcquiredError is returned when lock has been acquired
type AlreadyAcquiredError struct {
	trace.Traces `json:"traces"`
	Message      string `json:"message"`
}

// IsAlreadyAcquiredError returns true to indicate that this is AlreadyAcquiredError
func (e *AlreadyAcquiredError) IsAlreadyAcquiredError() bool {
	return true
}

// Error returns log friendly description
func (e *AlreadyAcquiredError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "lock is already aquired"
}

// OrigError returns original error (in this case this is the error itself)
func (e *AlreadyAcquiredError) OrigError() error {
	return e
}

// NotFoundError indicates that object has not been found
type NotFoundError struct {
	trace.Traces `json:"traces"`
	Message      string `json:"message"`
}

// IsNotFoundError returns true to indicate that is NotFoundError
func (e *NotFoundError) IsNotFoundError() bool {
	return true
}

// Error returns log friendly description of an error
func (e *NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "object not found"
}

// OrigError returns original error (in this case this is the error itself)
func (e *NotFoundError) OrigError() error {
	return e
}

// IsNotFound returns whether this error indicates that the object is not found
func IsNotFound(e error) bool {
	type nf interface {
		IsNotFoundError() bool
	}
	_, ok := e.(nf)
	return ok
}

// AlreadyExistsError indicates that there's a duplicate object that already
// exists in the storage/system
type AlreadyExistsError struct {
	trace.Traces `json:"traces"`
	// Message is user-friendly error message
	Message string `json:"message"`
}

// Error returns log-friendly error description
func (n *AlreadyExistsError) Error() string {
	if n.Message != "" {
		return n.Message
	}
	return "object already exists"
}

// IsAlreadyExistsError indicates that this error is of AlreadyExists kind
func (AlreadyExistsError) IsAlreadyExistsError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *AlreadyExistsError) OrigError() error {
	return e
}

// IsAlreadyExists returns if this is error indicating that object
// already exists
func IsAlreadyExists(e error) bool {
	type ae interface {
		IsAlreadyExistsError() bool
	}
	_, ok := e.(ae)
	return ok
}

// MissingParameterError indicates that one of the parameters was missing
type MissingParameterError struct {
	trace.Traces `json:"traces"`
	// Param is the name of the missing parameter
	Param string
}

// Error returns log-friendly description of the error
func (m *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter '%v'", m.Param)
}

// IsMissingParameterError indicates that this error is of MissingParameter type
func (m *MissingParameterError) IsMissingParameterError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *MissingParameterError) OrigError() error {
	return e
}

// IsMissingParameter detects if this error is of MissingParameter kind
func IsMissingParameter(e error) bool {
	type ae interface {
		IsMissingParameterError() bool
	}
	_, ok := e.(ae)
	return ok
}

// BadParameter returns a new instance of BadParameterError
func BadParameter(name, message string) *BadParameterError {
	return &BadParameterError{
		Param: name,
		Err:   message,
	}
}

// BadParameterError indicates that something is wrong with passed
// parameter to API method
type BadParameterError struct {
	trace.Traces
	Param string `json:"param"`
	Err   string `json:"message"`
}

// Error returrns debug friendly message
func (b *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter '%v', %v", b.Param, b.Err)
}

// OrigError returns original error (in this case this is the error itself)
func (b *BadParameterError) OrigError() error {
	return b
}

// IsBadParameterError indicates that error is of bad parameter type
func (b *BadParameterError) IsBadParameterError() bool {
	return true
}

// IsBadParameter detects if this error is of BadParameter kind
func IsBadParameter(e error) bool {
	type bp interface {
		IsBadParameterError() bool
	}
	_, ok := e.(bp)
	return ok
}

// CompareFailedError indicates that compare failed (e.g wrong password or hash)
type CompareFailedError struct {
	trace.Traces
	// Message is user-friendly error message
	Message string `json:"message"`
}

// Error is debug - friendly message
func (e *CompareFailedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "compare failed"
}

// OrigError returns original error (in this case this is the error itself)
func (e *CompareFailedError) OrigError() error {
	return e
}

// IsCompareFailed detects if this error is of CompareFailed kind
func IsCompareFailed(e error) bool {
	type cf interface {
		IsCompareFailedError() bool
	}
	_, ok := e.(cf)
	return ok
}

// ReadonlyError indicates that some backend can only read, not write
type ReadonlyError struct {
	trace.Traces
	Message string `json:"message"`
}

// Error is debug - friendly error message
func (e *ReadonlyError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "can't modify data in readonly mode"
}

// IsReadonlyError indicates that this error is of Readonly type
func (e *ReadonlyError) IsReadonlyError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *ReadonlyError) OrigError() error {
	return e
}

// IsReadonly detects if this error is of ReadonlyError
func IsReadonly(e error) bool {
	type ro interface {
		IsReadonlyError() bool
	}
	_, ok := e.(ro)
	return ok
}
