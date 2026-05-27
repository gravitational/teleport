package directory

import (
	"errors"
	"time"

	"github.com/gravitational/teleport/lib/msgraph"
)

// IsErrGraphAPIThrottled checks if the error suggests that the Graph API
// throttled and returns a "RetryAfter" wait interval to wait before retry.
// If throttled, a duration greater than zero is expected but is not guaranteed.
func IsErrGraphAPIThrottled(err error) (time.Duration, bool) {

	// The docs says status code 429 should be enough but 50x status code was
	// also encountered during manual test. We'll check against all known
	// possible error codes.
	// https://learn.microsoft.com/en-us/graph/throttling#best-practices-to-handle-throttling
	isThrottled := func(e *msgraph.GraphError) bool {
		return e.StatusCode == 429 ||
			e.Code == msgraph.ErrCodeTooManyRequest ||
			e.Code == msgraph.ErrCodeThrottled
	}

	graphError := &msgraph.GraphError{}
	if errors.As(err, &graphError) {
		if !isThrottled(graphError) {
			return 0, false
		}

		return graphError.RetryAfter, true
	}
	return 0, false
}
