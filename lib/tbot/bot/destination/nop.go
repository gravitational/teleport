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

package destination

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const NopType = "nop"

// Nop does nothing! Useful for odd scenarios where a destination
// has to be returned but there is none to return.
type Nop struct{}

// CheckAndSetDefaults does nothing! It is necessary to implement the
// Destination interface.
func (dm *Nop) CheckAndSetDefaults() error {
	return nil
}

// Init does nothing! It is necessary to implement the Destination interface.
func (dm *Nop) Init(_ context.Context, subdirs []string) error {
	// Nothing to do.
	return nil
}

// Verify does nothing! It is necessary to implement the Destination interface.
func (dm *Nop) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

// Write does nothing! It is necessary to implement the Destination interface.
func (dm *Nop) Write(_ context.Context, name string, data []byte) error {
	// Nothing to do.
	return nil
}

// Read does nothing, it behaves as if the requested artifact could not be
// found! It is necessary to implement the Destination interface.
func (dm *Nop) Read(_ context.Context, name string) ([]byte, error) {
	// Nothing to do.
	return nil, trace.NotFound("reading from a nop destination results in no data")
}

// String returns a human-readable string that describes this instance.
func (dm *Nop) String() string {
	return NopType
}

// TryLock does nothing! It is necessary to implement the Destination interface.
func (dm *Nop) TryLock() (func() error, error) {
	return func() error {
		return nil
	}, nil
}

// MarshalYAML enables the yaml package to correctly marshal the Destination
// as YAML including the type header.
func (dm *Nop) MarshalYAML() (any, error) {
	type raw Nop
	return encoding.WithTypeHeader((*raw)(dm), NopType)
}

func (dm *Nop) IsPersistent() bool {
	return false
}
