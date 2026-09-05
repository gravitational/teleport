package devicetrustv1

import "errors"

// auditStatusError is an error wrapper that carries a custom audit
// Status.UserMessage.
type auditStatusError struct {
	Err         error
	UserMessage string
}

// Error implements error.
func (e auditStatusError) Error() string {
	return e.Err.Error()
}

// Unwrap adds support for errors.Unwrap (and similar methods).
func (e auditStatusError) Unwrap() error {
	return e.Err
}

// UserMessage returns the custom audit Status.UserMessage carried by err, or
// an empty string.
//
// TODO(ravicious): Consider moving this to e/lib/devicetrust/errors and making
// use of auditStatusError within e/lib/devicetrust/devicetrustpublicv1.
// For now, it is exported so that the public Device Trust service can audit
// ceremony outcomes the same way the private handlers do.
func UserMessage(err error) string {
	var auditErr auditStatusError
	if errors.As(err, &auditErr) {
		return auditErr.UserMessage
	}
	return ""
}
