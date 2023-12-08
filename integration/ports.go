/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integration

import (
	"fmt"

	"github.com/gravitational/teleport/lib/utils"
)

// ports contains tcp ports allocated for all integration tests.
// TODO: Replace all usage of `Ports` with FD-injected sockets as per
//
//	https://github.com/gravitational/teleport/pull/13346
var ports utils.PortList

func init() {
	// Allocate tcp ports for all integration tests. 5000 should be plenty.
	var err error
	ports, err = utils.GetFreeTCPPorts(5000, utils.PortStartingNumber)
	if err != nil {
		panic(fmt.Sprintf("failed to allocate tcp ports for tests: %v", err))
	}
}

// newPortValue fetches a port from the pool.
// Deprecated: Use helpers.NewListener() and friends instead.
func newPortValue() int {
	return ports.PopInt()
}

// newPortStr fetches aport from the pool as a string.
// Deprecated: Use helpers.NewListener() and friends instead.
func newPortStr() string {
	return ports.Pop()
}
