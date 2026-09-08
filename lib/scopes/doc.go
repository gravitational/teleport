/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// The scopes package provides helpers for working with scoped resources and access-control
// policies. The core scope-string primitives (parsing, validation, comparison, and the
// [QualifiedName] type) live in [github.com/gravitational/teleport/api/scopes] and are
// re-exported here for convenience; see that package's documentation for the scoping model
// and usage guidance. This package additionally provides pagination cursors, backend key
// encoding, scope filtering, and feature gating for scoped resources.
package scopes
