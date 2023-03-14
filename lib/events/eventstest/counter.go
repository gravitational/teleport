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

package eventstest

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types/events"
)

// NewCountingEmitter returns an emitter that counts the number
// of events that are emitted. It is safe for concurrent use.
func NewCountingEmitter() *CounterEmitter {
	return &CounterEmitter{}
}

type CounterEmitter struct {
	count int64
}

func (c *CounterEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	atomic.AddInt64(&c.count, 1)
	return nil
}

// Count returns the number of events that have been emitted.
// It is safe for concurrent use.
func (c *CounterEmitter) Count() int64 {
	return atomic.LoadInt64(&c.count)
}
