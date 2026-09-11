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

// ErrInvalidDeviceWebToken is returned for every device web token failure that
// must stay indistinguishable to the caller: a bad, expired or already-spent
// token, a token issued for a different user or device, and a mismatched
// client IP.
//
// Both services return the same message, so a probing caller cannot tell which
// surface or stage rejected the token.
var ErrInvalidDeviceWebToken = &trace.AccessDeniedError{
	Message: "invalid device web token",
}

// ErrDeviceTrustDisabled is returned by device authentication when the cluster
// has device trust turned off.
var ErrDeviceTrustDisabled = &trace.BadParameterError{
	Message: "device trust disabled by cluster settings",
}
