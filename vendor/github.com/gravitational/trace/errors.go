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

package trace

import (
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
)

// NotFound returns new instance of not found error
func NotFound(message string, args ...interface{}) Error {
	return newTrace(&NotFoundError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// NotFoundError indicates that object has not been found
type NotFoundError struct {
	Message string `json:"message"`
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

// Is provides an equivalency check for NotFoundError to be used with errors.Is
func (e *NotFoundError) Is(target error) bool {
	if os.IsNotExist(target) {
		return true
	}

	err, ok := Unwrap(target).(*NotFoundError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsNotFound returns whether this error is of NotFoundError type
func IsNotFound(err error) bool {
	err = Unwrap(err)
	_, ok := err.(interface {
		IsNotFoundError() bool
	})
	if !ok {
		return os.IsNotExist(err)
	}
	return true
}

// AlreadyExists returns a new instance of AlreadyExists error
func AlreadyExists(message string, args ...interface{}) Error {
	return newTrace(&AlreadyExistsError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// AlreadyExistsError indicates that there's a duplicate object that already
// exists in the storage/system
type AlreadyExistsError struct {
	Message string `json:"message"`
}

// Error returns log friendly description of an error
func (e *AlreadyExistsError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "object already exists"
}

// IsAlreadyExistsError indicates that this error of the AlreadyExistsError type
func (AlreadyExistsError) IsAlreadyExistsError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *AlreadyExistsError) OrigError() error {
	return e
}

// Is provides an equivalency check for AlreadyExistsError to be used with errors.Is
func (e *AlreadyExistsError) Is(target error) bool {
	err, ok := Unwrap(target).(*AlreadyExistsError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsAlreadyExists returns whether this is error indicating that object
// already exists
func IsAlreadyExists(e error) bool {
	type ae interface {
		IsAlreadyExistsError() bool
	}
	_, ok := Unwrap(e).(ae)
	return ok
}

// BadParameter returns a new instance of BadParameterError
func BadParameter(message string, args ...interface{}) Error {
	return newTrace(&BadParameterError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// BadParameterError indicates that something is wrong with passed
// parameter to API method
type BadParameterError struct {
	Message string `json:"message"`
}

// Error returns log friendly description of an error
func (b *BadParameterError) Error() string {
	return b.Message
}

// OrigError returns original error (in this case this is the error itself)
func (b *BadParameterError) OrigError() error {
	return b
}

// IsBadParameterError indicates that this error is of BadParameterError type
func (b *BadParameterError) IsBadParameterError() bool {
	return true
}

// Is provides an equivalency check for BadParameterError to be used with errors.Is
func (b *BadParameterError) Is(target error) bool {
	err, ok := Unwrap(target).(*BadParameterError)
	if !ok {
		return false
	}

	return err.Message == b.Message
}

// IsBadParameter returns whether this error is of BadParameterType
func IsBadParameter(e error) bool {
	type bp interface {
		IsBadParameterError() bool
	}
	_, ok := Unwrap(e).(bp)
	return ok
}

// NotImplemented returns a new instance of NotImplementedError
func NotImplemented(message string, args ...interface{}) Error {
	return newTrace(&NotImplementedError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// NotImplementedError defines an error condition to describe the result
// of a call to an unimplemented API
type NotImplementedError struct {
	Message string `json:"message"`
}

// Error returns log friendly description of an error
func (e *NotImplementedError) Error() string {
	return e.Message
}

// OrigError returns original error
func (e *NotImplementedError) OrigError() error {
	return e
}

// IsNotImplementedError indicates that this error is of NotImplementedError type
func (e *NotImplementedError) IsNotImplementedError() bool {
	return true
}

// Is provides an equivalency check for NotImplementedError to be used with errors.Is
func (e *NotImplementedError) Is(target error) bool {
	err, ok := Unwrap(target).(*NotImplementedError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsNotImplemented returns whether this error is of NotImplementedError type
func IsNotImplemented(e error) bool {
	type ni interface {
		IsNotImplementedError() bool
	}
	err, ok := Unwrap(e).(ni)
	return ok && err.IsNotImplementedError()
}

// CompareFailed returns new instance of CompareFailedError
func CompareFailed(message string, args ...interface{}) Error {
	return newTrace(&CompareFailedError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// CompareFailedError indicates a failed comparison (e.g. bad password or hash)
type CompareFailedError struct {
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

// IsCompareFailedError indicates that this is CompareFailedError
func (e *CompareFailedError) IsCompareFailedError() bool {
	return true
}

// Is provides an equivalency check for CompareFailedError to be used with errors.Is
func (e *CompareFailedError) Is(target error) bool {
	err, ok := Unwrap(target).(*CompareFailedError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsCompareFailed detects if this error is of CompareFailed type
func IsCompareFailed(e error) bool {
	type cf interface {
		IsCompareFailedError() bool
	}
	_, ok := Unwrap(e).(cf)
	return ok
}

// AccessDenied returns new instance of AccessDeniedError
func AccessDenied(message string, args ...interface{}) Error {
	return newTrace(&AccessDeniedError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// AccessDeniedError indicates denied access
type AccessDeniedError struct {
	Message string `json:"message"`
}

// Error is debug - friendly error message
func (e *AccessDeniedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "access denied"
}

// IsAccessDeniedError indicates that this error is of AccessDeniedError type
func (e *AccessDeniedError) IsAccessDeniedError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *AccessDeniedError) OrigError() error {
	return e
}

// Is provides an equivalency check for AccessDeniedError to be used with errors.Is
func (e *AccessDeniedError) Is(target error) bool {
	err, ok := Unwrap(target).(*AccessDeniedError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsAccessDenied detects if this error is of AccessDeniedError type
func IsAccessDenied(err error) bool {
	_, ok := Unwrap(err).(interface {
		IsAccessDeniedError() bool
	})
	return ok
}

// ConvertSystemError converts system error to appropriate trace error
// if it is possible, otherwise, returns original error
func ConvertSystemError(err error) error {
	innerError := Unwrap(err)

	if os.IsExist(innerError) {
		return newTrace(&AlreadyExistsError{
			Message: innerError.Error(),
		}, 2)
	}
	if os.IsNotExist(innerError) {
		return newTrace(&NotFoundError{
			Message: innerError.Error(),
		}, 2)
	}
	if os.IsPermission(innerError) {
		return newTrace(&AccessDeniedError{
			Message: innerError.Error(),
		}, 2)
	}
	switch realErr := innerError.(type) {
	case *net.OpError:
		return newTrace(&ConnectionProblemError{
			Err: realErr,
		}, 2)
	case *os.PathError:
		message := fmt.Sprintf("failed to execute command %v error:  %v", realErr.Path, realErr.Err)
		return newTrace(&AccessDeniedError{
			Message: message,
		}, 2)
	case x509.SystemRootsError, x509.UnknownAuthorityError:
		return newTrace(&TrustError{Err: innerError}, 2)
	}
	if _, ok := innerError.(net.Error); ok {
		return newTrace(&ConnectionProblemError{
			Err: innerError,
		}, 2)
	}
	return err
}

// ConnectionProblem returns new instance of ConnectionProblemError
func ConnectionProblem(err error, message string, args ...interface{}) Error {
	return newTrace(&ConnectionProblemError{
		Message: fmt.Sprintf(message, args...),
		Err:     err,
	}, 2)
}

// ConnectionProblemError indicates a network related problem
type ConnectionProblemError struct {
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error is debug - friendly error message
func (c *ConnectionProblemError) Error() string {
	if c.Message != "" {
		return c.Message
	}
	return UserMessage(c.Err)
}

// IsConnectionProblemError indicates that this error is of ConnectionProblemError type
func (c *ConnectionProblemError) IsConnectionProblemError() bool {
	return true
}

// Unwrap returns the wrapped error if any
func (c *ConnectionProblemError) Unwrap() error {
	return c.Err
}

// OrigError returns original error
func (c *ConnectionProblemError) OrigError() error {
	if c.Err != nil {
		return c.Err
	}
	return c
}

// Is provides an equivalency check for ConnectionProblemError to be used with errors.Is
func (c *ConnectionProblemError) Is(target error) bool {
	err, ok := Unwrap(target).(*ConnectionProblemError)
	if !ok {
		return false
	}

	return err.Message == c.Message && err.Err == c.Err
}

// IsConnectionProblem returns whether this error is of ConnectionProblemError
func IsConnectionProblem(e error) bool {
	type ad interface {
		IsConnectionProblemError() bool
	}
	_, ok := Unwrap(e).(ad)
	return ok
}

// LimitExceeded returns whether new instance of LimitExceededError
func LimitExceeded(message string, args ...interface{}) Error {
	return newTrace(&LimitExceededError{
		Message: fmt.Sprintf(message, args...),
	}, 2)
}

// LimitExceededError indicates rate limit or connection limit problem
type LimitExceededError struct {
	Message string `json:"message"`
}

// Error is debug - friendly error message
func (e *LimitExceededError) Error() string {
	return e.Message
}

// IsLimitExceededError indicates that this error is of ConnectionProblem
func (e *LimitExceededError) IsLimitExceededError() bool {
	return true
}

// OrigError returns original error (in this case this is the error itself)
func (e *LimitExceededError) OrigError() error {
	return e
}

// Is provides an equivalency check for LimitExceededError to be used with errors.Is
func (e *LimitExceededError) Is(target error) bool {
	err, ok := Unwrap(target).(*LimitExceededError)
	if !ok {
		return false
	}

	return err.Message == e.Message
}

// IsLimitExceeded detects if this error is of LimitExceededError
func IsLimitExceeded(e error) bool {
	type ad interface {
		IsLimitExceededError() bool
	}
	_, ok := Unwrap(e).(ad)
	return ok
}

// Trust returns new instance of TrustError
func Trust(err error, message string, args ...interface{}) Error {
	return newTrace(&TrustError{
		Message: fmt.Sprintf(message, args...),
		Err:     err,
	}, 2)
}

// TrustError indicates trust-related validation error (e.g. untrusted cert)
type TrustError struct {
	// Err is original error
	Err     error  `json:"-"`
	Message string `json:"message"`
}

// Error returns log-friendly error description
func (t *TrustError) Error() string {
	if t.Message != "" {
		return t.Message
	}
	return UserMessage(t.Err)
}

// IsTrustError indicates that this error is of TrustError type
func (*TrustError) IsTrustError() bool {
	return true
}

// Unwrap returns the wrapped error if any
func (t *TrustError) Unwrap() error {
	return t.Err
}

// OrigError returns original error (in this case this is the error itself)
func (t *TrustError) OrigError() error {
	if t.Err != nil {
		return t.Err
	}
	return t
}

// Is provides an equivalency check for TrustError to be used with errors.Is
func (t *TrustError) Is(target error) bool {
	err, ok := Unwrap(target).(*TrustError)
	if !ok {
		return false
	}

	return err.Message == t.Message && err.Err == t.Err
}

// IsTrustError returns if this is a trust error
func IsTrustError(e error) bool {
	type te interface {
		IsTrustError() bool
	}
	_, ok := Unwrap(e).(te)
	return ok
}

// OAuth2 returns new instance of OAuth2Error
func OAuth2(code, message string, query url.Values) Error {
	return newTrace(&OAuth2Error{
		Code:    code,
		Message: message,
		Query:   query,
	}, 2)
}

// OAuth2Error defined an error used in OpenID Connect Flow (OIDC)
type OAuth2Error struct {
	Code    string     `json:"code"`
	Message string     `json:"message"`
	Query   url.Values `json:"query"`
}

//Error returns log friendly description of an error
func (o *OAuth2Error) Error() string {
	return fmt.Sprintf("OAuth2 error code=%v, message=%v", o.Code, o.Message)
}

// IsOAuth2Error returns whether this error of OAuth2Error type
func (o *OAuth2Error) IsOAuth2Error() bool {
	return true
}

// Is provides an equivalency check for OAuth2Error to be used with errors.Is
func (o *OAuth2Error) Is(target error) bool {
	err, ok := Unwrap(target).(*OAuth2Error)
	if !ok {
		return false
	}

	if err.Message != o.Message ||
		err.Code != o.Code ||
		len(err.Query) != len(o.Query) {
		return false
	}

	for k, v := range err.Query {
		for k2, v2 := range o.Query {
			if k != k2 && len(v) != len(v2) {
				return false
			}

			for i := range v {
				if v[i] != v2[i] {
					return false
				}
			}
		}
	}

	return true
}

// IsOAuth2 returns if this is a OAuth2-related error
func IsOAuth2(e error) bool {
	type oe interface {
		IsOAuth2Error() bool
	}
	_, ok := Unwrap(e).(oe)
	return ok
}

// IsEOF returns true if the passed error is io.EOF
func IsEOF(e error) bool {
	return Unwrap(e) == io.EOF
}

// Retry returns new instance of RetryError which indicates a transient error type
func Retry(err error, message string, args ...interface{}) Error {
	return newTrace(&RetryError{
		Message: fmt.Sprintf(message, args...),
		Err:     err,
	}, 2)
}

// RetryError indicates a transient error type
type RetryError struct {
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error is debug-friendly error message
func (c *RetryError) Error() string {
	if c.Message != "" {
		return c.Message
	}
	return UserMessage(c.Err)
}

// IsRetryError indicates that this error is of RetryError type
func (c *RetryError) IsRetryError() bool {
	return true
}

// Unwrap returns the wrapped error if any
func (c *RetryError) Unwrap() error {
	return c.Err
}

// OrigError returns original error (in this case this is the error itself)
func (c *RetryError) OrigError() error {
	if c.Err != nil {
		return c.Err
	}
	return c
}

// Is provides an equivalency check for RetryError to be used with errors.Is
func (c *RetryError) Is(target error) bool {
	err, ok := Unwrap(target).(*RetryError)
	if !ok {
		return false
	}

	return err.Message == c.Message && err.Err == c.Err
}

// IsRetryError returns whether this error is of ConnectionProblemError
func IsRetryError(e error) bool {
	type ad interface {
		IsRetryError() bool
	}
	_, ok := Unwrap(e).(ad)
	return ok
}
