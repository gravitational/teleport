/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package cache

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// generatedTestResource describes a resource-gen managed cache-enabled resource
// for test infrastructure. Per-resource init() functions register into this
// slice so that cache_test.go can wire up backends, events, and Config fields
// without manual per-resource edits.
type generatedTestResource struct {
	// kind is the resource kind constant (e.g. types.KindCookie).
	kind string
	// newBackend creates the local service backend for this resource.
	newBackend func(b backend.Backend) (any, error)
	// testEvent returns a sample resource wrapped for event map testing.
	testEvent func() types.Resource
	// setOnConfig sets the backend service on a cache.Config.
	setOnConfig func(cfg *Config, svc any)
	// compareEvent compares an expected and actual resource after event round-trip.
	compareEvent func(t *testing.T, expected, actual types.Resource)
}

var generatedTestResources []generatedTestResource

func registerGeneratedTestResource(r generatedTestResource) {
	generatedTestResources = append(generatedTestResources, r)
}
