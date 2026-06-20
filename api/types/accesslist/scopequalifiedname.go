// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

// ScopeQualifiedName uniquely identifies a scoped or unscoped access list or
// access list member by (scope, name) pair.
type ScopeQualifiedName struct {
	// Scope is the scope of the resource, it should be empty for unscoped resources.
	Scope string
	// Name is the name of the resource.
	Name string
}
