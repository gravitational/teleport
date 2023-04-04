/*
Copyright 2022 Gravitational, Inc.

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

package ingress

import (
	"net"
	"sync"
)

// conntrack keeps track of existing net.Conn using
// a mutex for concurrent updates.
type conntrack struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// add adds the connection to the tracker returning true
// if the connection was not being tracked.
func (ct *conntrack) add(c net.Conn) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	_, exists := ct.conns[c]
	if exists {
		return false
	}
	ct.conns[c] = struct{}{}
	return true
}

// remove deletes the connection from the tracker returning
// true if the connection existed in the map
func (ct *conntrack) remove(c net.Conn) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	_, exists := ct.conns[c]
	delete(ct.conns, c)
	return exists
}
