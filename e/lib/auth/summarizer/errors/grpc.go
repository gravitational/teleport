package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrUnlicensed is returned when a user attempts to use the Session Summarization
// feature on a Teleport cluster that does not have the appropriate license.
var ErrUnlicensed = status.Error(
	codes.Unimplemented,
	"This Teleport cluster is not licensed for Session Summarization. "+
		"Please contact your administrator.",
)
