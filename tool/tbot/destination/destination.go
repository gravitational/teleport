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

package destination

// ModeHint is a backend-agnostic file mode hint.
type ModeHint int64

const (
	// ModeHintUnspecified hints that files should be created with default
	// (possibly insecure) permissions.
	ModeHintUnspecified ModeHint = iota

	// ModeHintSecret hints that files should be created with restricted
	// permissions, appropriate for secret data.
	ModeHintSecret
)

// Destination can persist renewable certificates.
type Destination interface {
	// Write stores data to the destination with the given name.
	Write(name string, data []byte, modeHint ModeHint) error

	// Read fetches data from the destination with a given name.
	Read(name string) ([]byte, error)
}
