// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package scopes

import (
	"encoding/hex"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// ResourceCursorPrefix prefixes cursors for scoped resources in a
// logical resource stream.
//
// The prefix starts with '~', which is not allowed in backend-safe resource
// names and sorts after all backend-safe name bytes, preserving historical
// name-only cursors for unscoped resources while ordering scoped resources
// after unscoped resources.
const ResourceCursorPrefix = "~scoped/"

// ResourceCursorScopedStart returns the first cursor in the scoped portion of
// the logical resource stream.
func ResourceCursorScopedStart() string {
	return ResourceCursorPrefix
}

// IsScopedResourceCursor returns true if cursor is in the scoped portion of the
// logical resource stream.
func IsScopedResourceCursor(cursor string) bool {
	return strings.HasPrefix(cursor, ResourceCursorPrefix)
}

// MakeResourceCursor returns the cursor for a scoped or unscoped resource in a
// logical, lexicographically ordered resource stream.
//
// Resource cursors are intended for pagination tokens, range bounds, and
// in-memory cache indexes. They are not backend storage keys and must not be
// used to construct backend keys.
//
// Unscoped resource cursors preserve the historical name-only format:
//
//	<name>
//
// Scoped resource cursors use a synthetic prefix that cannot appear in
// backend-safe resource names and sorts after all backend-safe name bytes:
//
//	~scoped/<encoded-scope>/<name>
//
// The scope component is encoded with [EncodeForKey] so that scoped cursors
// preserve scope ordering and can safely use '/' as the cursor separator.
//
// MakeResourceCursor is infallible so that it can back key derivation with no
// error path (in-memory cache indexes, pagination cursors). A scope that
// cannot be encoded (which is only possible for invalid stored data) yields a
// degraded cursor that is deterministic, unique per scope and name, sorts after
// all valid cursors, and fails [ParseResourceCursor].
func MakeResourceCursor(scope, name string) string {
	return MakeNestedResourceCursor(QualifiedName{
		Scope: scope,
		Name:  name,
	})
}

// MakeNestedResourceCursor returns the cursor for a scoped or unscoped nested
// resource in a logical, lexicographically ordered range of nested resources.
// The motivating example is scoped access list members, where members are
// keyed under their parent list.
//
// Resource cursors are intended for pagination tokens, range bounds, and
// in-memory cache indexes. They are not backend storage keys and must not be
// used to construct backend keys.
//
// If all provided scopes are empty, all names will simply be joined with the
// separator, to maintain compatibility with existing unscoped resource cursors
// and avoid wastefully encoding multiple empty scopes:
//
//	<root-name>[/<descendent-name>]...
//
// Scoped resource cursors use a synthetic prefix that cannot appear in
// backend-safe resource names and sorts after all backend-safe name bytes.
//
//	~scoped/<encoded-root-scope>/<root-name>[/<encoded-descendent-scope>/<descendent-name>]...
//
// Each scope component is encoded with [EncodeForKey] so that scoped cursors
// preserve scope ordering and can safely use '/' as the cursor separator.
//
// MakeNestedResourceCursor is infallible so that it can back key derivation with no
// error path (in-memory cache indexes, pagination cursors). A scope that
// cannot be encoded (which is only possible for invalid stored data) yields a
// degraded cursor that is deterministic, unique per scope and name, sorts after
// all valid cursors, and fails [ParseResourceCursor].
func MakeNestedResourceCursor(root QualifiedName, descendents ...QualifiedName) string {
	hasNonEmptyScope := root.Scope != "" || slices.ContainsFunc(descendents, func(descendent QualifiedName) bool {
		return descendent.Scope != ""
	})
	if !hasNonEmptyScope {
		var sb strings.Builder
		sb.WriteString(root.Name)
		for _, descendent := range descendents {
			sb.WriteString(separator)
			sb.WriteString(descendent.Name)
		}
		return sb.String()
	}

	var sb strings.Builder
	sb.WriteString(ResourceCursorPrefix)
	sb.WriteString(EncodeForResourceCursor(root.Scope))
	sb.WriteString(separator)
	sb.WriteString(root.Name)
	for _, descendent := range descendents {
		sb.WriteString(separator)
		sb.WriteString(EncodeForResourceCursor(descendent.Scope))
		sb.WriteString(separator)
		sb.WriteString(descendent.Name)
	}
	return sb.String()
}

// EncodeForResourceCursor is infallible so that it can back key derivation with no
// error path (in-memory cache indexes, pagination cursors). A scope that
// cannot be encoded (which is only possible for invalid stored data) yields a
// degraded cursor that is deterministic, unique per scope and name, sorts after
// all valid cursors, and fails [ParseResourceCursor].
func EncodeForResourceCursor(scope string) string {
	encoded, err := EncodeForKey(scope)
	if err != nil {
		// '~' cannot appear in a valid scope encoding (which starts with '+'),
		// so degraded cursors never collide with valid cursors and sort after
		// them. The scope is hex-encoded so the cursor's scope component is a
		// single path segment regardless of the scope's contents.
		return "~invalid+" + hex.EncodeToString([]byte(scope))
	}
	return encoded
}

// MakeResourceCursorWithHost returns the cursor for a scoped or unscoped
// host-keyed resource — one keyed by (host ID, name), such as an app server —
// in a logical, lexicographically ordered resource stream:
//
//	scoped:   ~scoped/<encoded-scope>/<host-id>/<name>
//
// See [MakeResourceCursor] for cursor semantics. Host cursors are not
// parseable by [ParseResourceCursor], which rejects composite names.
func MakeResourceCursorWithHost(scope, hostID, name string) string {
	return MakeResourceCursor(scope, hostID+separator+name)
}

// ParseResourceCursor parses a resource cursor produced by [MakeResourceCursor]
// into its scope and name components.
//
// Unscoped cursors are interpreted as historical name-only cursors. Scoped
// cursors must use the scoped cursor format:
//
//	~scoped/<encoded-scope>/<name>
func ParseResourceCursor(cursor string) (QualifiedName, error) {
	encodedScopeAndName, ok := strings.CutPrefix(cursor, ResourceCursorPrefix)
	if !ok {
		return QualifiedName{Name: cursor}, nil
	}

	encodedScope, name, ok := strings.Cut(encodedScopeAndName, separator)
	if !ok {
		return QualifiedName{}, trace.BadParameter("scoped resource cursor %q missing name separator", cursor)
	}
	if encodedScope == "" {
		return QualifiedName{}, trace.BadParameter("scoped resource cursor %q has empty encoded scope", cursor)
	}
	if name == "" {
		return QualifiedName{}, trace.BadParameter("scoped resource cursor %q has empty name", cursor)
	}
	if strings.Contains(name, separator) {
		return QualifiedName{}, trace.BadParameter("scoped resource cursor %q has invalid name", cursor)
	}

	scope, err := DecodeFromKey(encodedScope)
	if err != nil {
		return QualifiedName{}, trace.Wrap(err)
	}

	return QualifiedName{Scope: scope, Name: name}, nil
}
