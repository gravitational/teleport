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
	"context"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestIsNetworkTimeoutError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped deadline exceeded", fmt.Errorf("sending request: %w", context.DeadlineExceeded), true},
		{"syscall timeout", syscall.ETIMEDOUT, true},
		{"network timeout", &net.DNSError{IsTimeout: true}, true},
		{"other error", fmt.Errorf("server rejected request"), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, IsNetworkTimeoutError(test.err))
		})
	}
}

func TestIsServerInfoChangedError(t *testing.T) {
	err := &serverInfoChangedError{
		expectedInfo: mcp.Implementation{
			Name:    "i-am-mcp",
			Version: "1.0.0",
		},
		currentInfo: mcp.Implementation{
			Name:    "i-am-mcp",
			Version: "1.1.0",
		},
	}
	require.True(t, IsServerInfoChangedError(err))
}
