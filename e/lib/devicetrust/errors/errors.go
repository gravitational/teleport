// Package errors holds Device Trust errors shared between the private and the
// public Device Trust services.
package errors

import "github.com/gravitational/trace"

// ErrInvalidDeviceEnrollToken is returned for every enrollment token failure
// that must stay indistinguishable to the caller: a bad, expired or
// already-spent token, a token minted for a different user and, on the public
// surface, a token without a bound user.
//
// Both services return the same message, so a probing caller cannot tell which
// surface or stage rejected the token.
var ErrInvalidDeviceEnrollToken = &trace.AccessDeniedError{
	Message: "invalid device enrollment token",
}
