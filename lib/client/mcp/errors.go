/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package mcp

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// ReloginRequiredErrorMessage is the message returned to the MCP client
	// when the tsh session expired.
	ReloginRequiredErrorMessage = `It looks like your Teleport session expired,
you must relogin (using "tsh login" on a terminal) before continue using this
tool. After that, there is no need to update or relaunch the MCP client - just
try using it again.`
)

// IsLikelyTemporaryNetworkError returns true if the error is likely a temporary
// network error.
func IsLikelyTemporaryNetworkError(err error) bool {
	if trace.IsConnectionProblem(err) ||
		isTemporarySyscallNetError(err) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return false
}

func isTemporarySyscallNetError(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ECONNREFUSED)
}

// IsServerInfoChangedError returns true if the error indicates the remote MCP
// server's info has changed from previous connections. Auto-reconnection
// reports this scenario as an error case to be on the safe side in case things
// like tools have changed.
func IsServerInfoChangedError(err error) bool {
	var serverInfoChangedError *serverInfoChangedError
	return errors.As(err, &serverInfoChangedError)
}

type serverInfoChangedError struct {
	expectedInfo mcp.Implementation
	currentInfo  mcp.Implementation
}

func (e *serverInfoChangedError) Error() string {
	return fmt.Sprintf("server info has changed, expected %v, got %v", e.expectedInfo, e.currentInfo)
}
