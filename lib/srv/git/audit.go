/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package git

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

type gitCommandEmitter struct {
	events.StreamEmitter
	discard apievents.Emitter
}

// NewEmitter returns an emitter for Git proxy usage.
func NewEmitter(emitter events.StreamEmitter) events.StreamEmitter {
	return &gitCommandEmitter{
		StreamEmitter: emitter,
		discard:       events.NewDiscardEmitter(),
	}
}

// EmitAuditEvent overloads EmitAuditEvent to only emit Git command events.
func (e *gitCommandEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	switch event.GetType() {
	// TODO(greedy52) enable this when available:
	// case events.GitCommandEvent:
	// 	return trace.Wrap(e.emitter.EmitAuditEvent(ctx, event))
	default:
		return trace.Wrap(e.discard.EmitAuditEvent(ctx, event))
	}
}
