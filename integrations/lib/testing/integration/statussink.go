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

package integration

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
)

// FakeStatusSink is a fake status sink that can be used when testing plugins.
type FakeStatusSink struct {
	status atomic.Pointer[types.PluginStatus]
}

// Emit implements the common.StatusSink interface.
func (s *FakeStatusSink) Emit(_ context.Context, status types.PluginStatus) error {
	s.status.Store(&status)
	return nil
}

// Get returns the last status stored by the plugin.
func (s *FakeStatusSink) Get() types.PluginStatus {
	status := s.status.Load()
	if status == nil {
		panic("expected status to be set, but it has not been")
	}
	return *status
}
