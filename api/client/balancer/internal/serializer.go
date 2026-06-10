// Copyright 2025 Gravitational, Inc
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

package internal

import (
	"sync"
)

// CallbackSerializer queues callbacks to be ran in FIFO order. This is useful
// for ordering operations outside of a mutex.
// CallbackSerializer is adapted from the implementation seen in grpc-go.
// Original source: https://github.com/grpc/grpc-go/blob/v1.75.1/internal/grpcsync/callback_serializer.go
type CallbackSerializer struct {
	mu      sync.Mutex
	c       chan func()
	buf     []func()
	closing bool
	closed  bool
}

func NewCallbackSerializer() *CallbackSerializer {
	cs := &CallbackSerializer{
		c: make(chan func(), 1),
	}
	go cs.run()
	return cs
}

func (cs *CallbackSerializer) run() {
	for cb := range cs.c {
		cs.load()
		cb()
	}
}

func (cs *CallbackSerializer) load() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if len(cs.buf) > 0 {
		select {
		case cs.c <- cs.buf[0]:
			cs.buf = cs.buf[1:]
		default:
		}
	} else if cs.closing && !cs.closed {
		cs.closed = true
		close(cs.c)
	}
}

// Put adds a function to the queue to be executed.
func (cs *CallbackSerializer) Put(fn func()) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.putLocked(fn)
}

func (cs *CallbackSerializer) putLocked(fn func()) bool {
	if cs.closing {
		return false
	}

	if len(cs.buf) == 0 {
		select {
		case cs.c <- fn:
			return true
		default:
		}
	}
	cs.buf = append(cs.buf, fn)
	return true
}

// Close begins closing the serializer. No new functions will be accepted but
// anything already in the queue will be executed.
func (cs *CallbackSerializer) Close() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closing {
		return
	}
	cs.closing = true
	if len(cs.buf) == 0 && !cs.closed {
		cs.closed = true
		close(cs.c)
	}
}

// Close begins closing the serializer. No new functions will be accepted but
// anything already in the queue will be executed.
func (cs *CallbackSerializer) CloseFn(fn func()) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closing {
		return
	}
	cs.putLocked(fn)
	cs.closing = true
	if len(cs.buf) == 0 && !cs.closed {
		cs.closed = true
		close(cs.c)
	}
}
