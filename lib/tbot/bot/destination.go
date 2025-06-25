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

package bot

import (
	"context"
	"fmt"
)

// Destination can persist renewable certificates.
type Destination interface {
	// Init attempts to initialize this destination for use. Init should be
	// idempotent and may write informational log messages if resources are
	// created.
	// This must be called before Read or Write.
	Init(ctx context.Context, subdirs []string) error

	// Verify is run before renewals to check for any potential problems with
	// the destination. These errors may be informational (logged warnings) or
	// return an error that may potentially terminate the process.
	Verify(keys []string) error

	// Write stores data to the destination with the given name.
	Write(ctx context.Context, name string, data []byte) error

	// Read fetches data from the destination with a given name.
	Read(ctx context.Context, name string) ([]byte, error)

	// TryLock attempts to lock a destination. This is non-blocking, and will
	// return an error if it is not possible to lock the destination.
	// TryLock should be used to lock a destination so it cannot be used by
	// multiple processes of tbot concurrently.
	TryLock() (func() error, error)

	// CheckAndSetDefaults validates the configuration and sets any defaults.
	//
	// This must be called before other methods on Destination can be called.
	CheckAndSetDefaults() error

	// MarshalYAML enables the yaml package to correctly marshal the Destination
	// as YAML including the type header.
	MarshalYAML() (any, error)

	// IsPersistent indicates whether this destination is persistent.
	// This is true for most production destinations, but will be false for
	// Nop or Memory destinations.
	IsPersistent() bool

	// Stringer so that Destination's implements fmt.Stringer which allows for
	// better logging.
	fmt.Stringer
}
