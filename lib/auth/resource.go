/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// MarshalConfig specifies marshalling options
type MarshalConfig struct {
	// Version specifies particular version we should marshal resources with
	Version string

	// SkipValidation is used to skip schema validation.
	SkipValidation bool

	// ID is a record ID to assign
	ID int64

	// PreserveResourceID preserves resource IDs in resource
	// specs when marshaling
	PreserveResourceID bool

	// Expires is an optional expiry time
	Expires time.Time
}

// GetVersion returns explicitly provided version or sets latest as default
func (m *MarshalConfig) GetVersion() string {
	if m.Version == "" {
		return types.V2
	}
	return m.Version
}

// MarshalOption sets marshalling option
type MarshalOption func(c *MarshalConfig) error
