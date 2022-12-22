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
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

// NewSlowEmitter creates an emitter that introduces an artificial
// delay before "emitting" (discarding) an audit event.
func NewSlowEmitter(delay time.Duration) events.Emitter {
	return &slowEmitter{
		delay: delay,
	}
}

// slowEmitter is an events.Emitter that introduces an artificial
// delay before emitting an event.
type slowEmitter struct {
	delay time.Duration
}

func (s *slowEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
