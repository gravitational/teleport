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

package events

import (
	"context"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/eventsclient"
)

func NewPreparer(cfg PreparerConfig) (*Preparer, error) {
	return eventsclient.NewPreparer(cfg)
}

// PreparerConfig configures an event setter
type PreparerConfig = eventsclient.PreparerConfig

// Preparer sets necessary unset fields in session events.
type Preparer = eventsclient.Preparer

// NewSessionPreparerRecorder returns a SessionPreparerRecorder that can both
// setup and record session events.
func NewSessionPreparerRecorder(setter SessionEventPreparer, recorder SessionRecorder) SessionPreparerRecorder {
	return eventsclient.NewSessionPreparerRecorder(setter, recorder)
}

// SetupAndRecordEvent will set necessary event fields for session-related
// events and record them.
func SetupAndRecordEvent(ctx context.Context, s SessionPreparerRecorder, e apievents.AuditEvent) error {
	return eventsclient.SetupAndRecordEvent(ctx, s, e)
}
