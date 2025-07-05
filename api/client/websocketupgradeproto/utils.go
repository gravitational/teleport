/*
Copyright 2025 Gravitational, Inc.

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

package websocketupgradeproto

import (
	"errors"
	"io"
	"net"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// isOkNetworkErrOrTimeout checks if the error is a network error that we
// should not log.
func isOkNetworkErrOrTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Check if the error is a timeout or a temporary network error.
		return netErr.Timeout()
	}
	return isOKNetworkError(err)
}

// IsOKNetworkError returns true if the provided error received from a network
// operation is one of those that usually indicate normal connection close. If
// the error is a trace.Aggregate, all the errors must be OK network errors.
func isOKNetworkError(err error) bool {
	// trace.Aggregate contains at least one error and all the errors are
	// non-nil
	var a trace.Aggregate
	if errors.As(trace.Unwrap(err), &a) {
		for _, err := range a.Errors() {
			if !isOKNetworkError(err) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, io.EOF) || isUseOfClosedNetworkError(err) || isFailedToSendCloseNotifyError(err)
}

// isUseOfClosedNetworkError returns true if the specified error
// indicates the use of a closed network connection.
func isUseOfClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), constants.UseOfClosedNetworkConnection)
}

// isFailedToSendCloseNotifyError returns true if the provided error is the
// "tls: failed to send closeNotify".
func isFailedToSendCloseNotifyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), constants.FailedToSendCloseNotify)
}
