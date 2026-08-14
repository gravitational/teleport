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

func getUserMessage(err error) string {
	var auditErr auditStatusError
	if errors.As(err, &auditErr) {
		return auditErr.UserMessage
	}
	return ""
}
