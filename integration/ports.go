// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
