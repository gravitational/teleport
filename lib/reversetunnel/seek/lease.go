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
	"fmt"
	"sync"
	"sync/atomic"
)

// Lease is a per-agent lease; used to manage the number of simultaneously
// connecting agents.  The SeekPool grants leases when new agents need to
// be spawned.  Agents release their leases when they halt.
type Lease struct {
	GroupHandle
	once    sync.Once
	release chan<- struct{}
	id      uint64
}

// WithProxy is used to wrap the connection-handling logic of an agent,
// ensuring that it is run if and only if no other agent is already
// handling this proxy.
func (l *Lease) WithProxy(do func(), principals ...string) (did bool) {
	return l.GroupHandle.WithProxy(do, l.ID(), principals...)
}

// ID returns the unique ID of this lease.
func (l *Lease) ID() uint64 {
	return l.id
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

func (l *Lease) String() string {
	return fmt.Sprintf("Lease(key=%v,id=%v)", l.Key(), l.id)
}

type counter struct {
	value *uint64
}

func newCounter() counter {
	return counter{
		value: new(uint64),
	}
}

func (c counter) Next() uint64 {
	return atomic.AddUint64(c.value, 1)
}

func (c counter) Load() uint64 {
	return atomic.LoadUint64(c.value)
}
