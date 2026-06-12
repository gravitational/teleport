//go:build windows
// +build windows

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

package storage

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend/memory"
)

// NewProcessStorage returns a new instance of the process storage.
func NewProcessStorage(ctx context.Context, path string) (*ProcessStorage, error) {
	m, err := memory.New(memory.Config{
		Context:   ctx,
		EventsOff: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ProcessStorage{BackendStorage: m, stateStorage: m}, nil
}
