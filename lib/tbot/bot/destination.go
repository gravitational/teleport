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

package bot

// Destination can persist renewable certificates.
type Destination interface {
	// Init attempts to initialize this destination for writing. Init should be
	// idempotent and may write informational log messages if resources are
	// created.
	Init(subdirs []string) error

	// Verify is run before renewals to check for any potential problems with
	// the destination. These errors may be informational (logged warnings) or
	// return an error that may potentially terminate the process.
	Verify(keys []string) error

	// Write stores data to the destination with the given name.
	Write(name string, data []byte) error

	// Read fetches data from the destination with a given name.
	Read(name string) ([]byte, error)

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
	MarshalYAML() (interface{}, error)
}
