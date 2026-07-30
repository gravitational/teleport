package devicetrustpublicv1

import (
	"github.com/gravitational/trace"
)

var ErrAwaitingApproval = &trace.CompareFailedError{Message: "enroll pairing is awaiting approval"}
var ErrPairingClaimed = &trace.AccessDeniedError{Message: "enroll pairing claimed by another device"}
