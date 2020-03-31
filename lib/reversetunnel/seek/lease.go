/*
Copyright 2020 Gravitational, Inc.

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

package seek

import (
	"sync"
)

// Lease is a per-agent lease; used to manage the number of simultaneously
// connecting agents.  The SeekPool grants leases when new agents need to
// be spawned.  Agents release their leases when they halt.
type Lease struct {
	GroupHandle
	once    sync.Once
	release chan<- struct{}
}

// Release releases the lease.
func (l *Lease) Release() {
	l.once.Do(func() {
		select {
		case l.release <- struct{}{}:
		case <-l.Done():
		}
	})
}

type leases struct {
	group   GroupHandle
	active  int
	grant   chan<- *Lease
	release chan struct{}
}
