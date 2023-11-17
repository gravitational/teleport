/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"context"

	"github.com/gravitational/trace"
)

const DestinationNopType = "nop"

// DestinationNop does nothing! Useful for odd scenarios where a destination
// has to be returned but there is none to return.
type DestinationNop struct{}

// CheckAndSetDefaults does nothing! It is necessary to implement the
// Destination interface.
func (dm *DestinationNop) CheckAndSetDefaults() error {
	return nil
}

// Init does nothing! It is necessary to implement the Destination interface.
func (dm *DestinationNop) Init(_ context.Context, subdirs []string) error {
	// Nothing to do.
	return nil
}

// Verify does nothing! It is necessary to implement the Destination interface.
func (dm *DestinationNop) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

// Write does nothing! It is necessary to implement the Destination interface.
func (dm *DestinationNop) Write(_ context.Context, name string, data []byte) error {
	// Nothing to do.
	return nil
}

// Read does nothing, it behaves as if the requested artifact could not be
// found! It is necessary to implement the Destination interface.
func (dm *DestinationNop) Read(_ context.Context, name string) ([]byte, error) {
	// Nothing to do.
	return nil, trace.NotFound("reading from a nop destination results in no data")
}

// String returns a human-readable string that describes this instance.
func (dm *DestinationNop) String() string {
	return DestinationNopType
}

// TryLock does nothing! It is necessary to implement the Destination interface.
func (dm *DestinationNop) TryLock() (func() error, error) {
	return func() error {
		return nil
	}, nil
}

// MarshalYAML enables the yaml package to correctly marshal the Destination
// as YAML including the type header.
func (dm *DestinationNop) MarshalYAML() (interface{}, error) {
	type raw DestinationNop
	return withTypeHeader((*raw)(dm), DestinationNopType)
}
