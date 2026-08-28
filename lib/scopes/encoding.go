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

	"github.com/gravitational/trace"
	"rsc.io/ordered"
)

// The scope key encoding produces a single opaque, order-preserving backend key
// segment for a scope. It is used to namespace scoped resources in the backend,
// e.g. as the <encoded_scope> component of a key like:
//
//	/scoped/<kind>/<encoded_scope>/<name>
//
// The encoding is built in two phases. First, the scope is encoded as an
// [ordered] sequence consisting of a leading discriminant that distinguishes
// scoped from unscoped values, followed by one string element per scope segment:
//
//	unscoped ("")    -> ordered.Encode(unscopedDisc)
//	root     ("/")   -> ordered.Encode(scopedDisc)
//	"/a"             -> ordered.Encode(scopedDisc, "a")
//	"/a/b"           -> ordered.Encode(scopedDisc, "a", "b")
//
// This encoding was chosen to satisfy several properties simultaneously:
//
//   - Order-preserving: a plain byte-sort of the encoded values reproduces
//     the same sort order as the [Sort] function.
//
//  - Scope prefixing: An encoded scope S is a string prefix of any encoded child scope,
//    but is *not* a prefix of a scope that is a sibling of S with the same leading segment
//    characters (e.g. EncodeForKey("/staging") is a prefix of EncodeForKey("/staging/west")
//    but not of EncodeForKey("/stagingwest")).
//
//  - Exact prefixing in backend: Appending the backend separator to the encoded scope allows
//    backend range queries to retrieve *exactly* the set of keys with that exact scope prefix,
//    without ambiguity.
//
//   - Forward-compatible: The encoding scheme can theoretically handle any future extension to
//     allowed scope characters.

const (
	// scopeKeyUnscopedDisc is the leading discriminant for the encoding of an
	// unscoped value (i.e. EncodeForKey("")). It sorts before
	// scopeKeyScopedDisc so that unscoped values sort before all scoped values.
	scopeKeyUnscopedDisc = 0

	// scopeKeyScopedDisc is the leading discriminant for the encoding of any
	// scoped value (including the root scope "/").
	scopeKeyScopedDisc = 1
)

// EncodeForKey encodes a scope so that it will be valid for use in a single
// backend key segment, preserving sort order. If given an empty string, it
// will return a non-empty encoding that sorts before all valid encoded
// scopes.
func EncodeForKey(scope string) (string, error) {
	if scope == "" {
		return hex.EncodeToString(ordered.Encode(scopeKeyUnscopedDisc)), nil
	}

	if err := WeakValidate(scope); err != nil {
		return "", trace.Wrap(err)
	}

	raw := ordered.Encode(scopeKeyScopedDisc)
	for segment := range DescendingSegments(scope) {
		raw = ordered.Append(raw, segment)
	}

	return hex.EncodeToString(raw), nil
}

// DecodeFromKey decodes a scope encoded by [EncodeForKey].
func DecodeFromKey(encoded string) (string, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", trace.BadParameter("invalid encoded scope %q: %v", encoded, err)
	}

	if len(raw) == 0 {
		return "", trace.BadParameter("invalid empty encoded scope")
	}

	var disc int
	rest, err := ordered.DecodePrefix(raw, &disc)
	if err != nil {
		return "", trace.BadParameter("malformed encoded scope %q: %v", encoded, err)
	}

	switch disc {
	case scopeKeyUnscopedDisc:
		if len(rest) != 0 {
			return "", trace.BadParameter("malformed unscoped encoding %q: unexpected trailing data", encoded)
		}
		return "", nil
	case scopeKeyScopedDisc:
		var segments []string
		for len(rest) > 0 {
			var segment string
			if rest, err = ordered.DecodePrefix(rest, &segment); err != nil {
				return "", trace.BadParameter("malformed encoded scope %q: %v", encoded, err)
			}
			segments = append(segments, segment)
		}

		decoded := Join(segments...)
		if err := WeakValidate(decoded); err != nil {
			return "", trace.Wrap(err)
		}
		return decoded, nil
	default:
		return "", trace.BadParameter("invalid scope discriminant %d in encoded scope %q", disc, encoded)
	}
}
