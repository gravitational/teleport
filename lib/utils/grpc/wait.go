// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package grpc

import (
	"context"

	"google.golang.org/grpc/connectivity"
)

type stateGetter interface {
	GetState() connectivity.State
	WaitForStateChange(context.Context, connectivity.State) bool
}

// WaitForConnectionLoss waits for conn to leave the Ready state after
// entering it. If the conn is already in a Shutdown state, return immediately.
// If the conn is not yet in a Ready state, wait until it reaches it and then
// return after it leaves the Ready state. The return value of
// conn.WaitForStateChange is returned which indicates whether the return was
// from context cancelation (false) or a state change (true).
//
// See https://github.com/grpc/grpc/blob/master/doc/connectivity-semantics-and-api.md
// for details of the connection states and their transitions.
func WaitForConnectionLoss(ctx context.Context, conn stateGetter) bool {
	for {
		switch state := conn.GetState(); state {
		case connectivity.Shutdown:
			// If we start in Shutdown, return immediately
			return true
		case connectivity.Ready:
			// Once in Ready, we return on any state change. A transition
			// to Idle means the connection was closed. Same for
			// TransientFailure or Shutdown.
			return conn.WaitForStateChange(ctx, connectivity.Ready)
		default: // Idle, Connecting, TransientFailure
			// If we start in Connecting, Idle or TransientFailure, keep
			// checking the state until we are ready or fail/shutdown
			if !conn.WaitForStateChange(ctx, state) {
				return false // context done
			}
		}
	}
}
