package internal

import "errors"

// AuditStatusError is an error wrapper that carries a custom audit
// Status.UserMessage.
type AuditStatusError struct {
	Err         error
	UserMessage string
}

// Error implements error.
func (e AuditStatusError) Error() string {
	return e.Err.Error()
}

// Unwrap adds support for errors.Unwrap (and similar methods).
func (e AuditStatusError) Unwrap() error {
	return e.Err
}

func GetUserMessage(err error) string {
	var auditErr AuditStatusError
	if errors.As(err, &auditErr) {
		return auditErr.UserMessage
	}
	return ""
}
