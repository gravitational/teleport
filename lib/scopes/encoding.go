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
	"strings"

	"github.com/gravitational/trace"
)

const (
	// + sorts before all characters that can appear in a scope (see
	// segmentRegexp), is allowed in backend keys, and is not the backend key
	// separator.
	encodedSeparator       = '+'
	encodedSeparatorString = string(encodedSeparator)
)

// EncodeForKey encodes a scope so that it will be valid for use in a single
// backend key segment, preserving sort order. If given an empty string, it
// will return a non-empty encoding that sorts before all valid encoded
// scopes.
//
// This implementation is a placeholder to unblock development, it is intended
// to be replaced with a better implementation.
func EncodeForKey(scope string) (string, error) {
	// The empty scope is encoded as a single encoded separator.
	//
	// All other scopes are encoded with an extra separator at the beginning so
	// that the root scope sorts after the empty scope.
	// They also have an extra separator added at the end to support prefix and
	// exact matches.
	if scope == "" {
		return encodedSeparatorString, nil
	}

	if err := WeakValidate(scope); err != nil {
		return "", trace.Wrap(err)
	}

	// Enforce that the scope does not contain any byte that would sort before
	// or equal to the encoded separator, which would break sorting.
	// StrongValidate would prevent this, WeakValidate may not.
	for _, b := range []byte(scope) {
		if b <= encodedSeparator {
			return "", trace.BadParameter("scope contains invalid byte %d which would break sorting", b)
		}
	}

	// Enforce that the scope does not contain empty segments, which can also
	// break sorting of composed keys. For example / and /// could encode to:
	//
	//   +++/resource
	//   +++++/resource
	//
	// and the sort order would invert depending on whether the /resource
	// suffix was present.
	//
	// Also enforce that each segment does not start with a byte that would
	// sort before the backend separator, which could break sorting of composed
	// keys. For example /a and /a/.b could encode to:
	//
	//   ++a+/resource
	//   ++a+.b+/resource
	//
	// and the sort order would be inverted because '.' sorts before '/'
	// This is only an issue at the first byte of a segment.
	for segment := range DescendingSegments(scope) {
		if len(segment) == 0 {
			return "", trace.BadParameter("scope contains empty segment which would break sorting")
		}
		if segment[0] < '/' {
			return "", trace.BadParameter("scope segment starts with invalid byte %d which would break sorting", segment[0])
		}
	}

	// NormalizeForEquality will trim a trailing separator, which may be
	// necessary for prefix/exact match queries on encoded scopes.
	encoded := NormalizeForEquality(scope)
	encoded = strings.ReplaceAll(encoded, separator, encodedSeparatorString)

	encoded = encodedSeparatorString + encoded + encodedSeparatorString
	return encoded, nil
}

// DecodeFromKey decodes a scope encoded by EncodeForKey.
func DecodeFromKey(encodedScope string) (string, error) {
	if encodedScope == encodedSeparatorString {
		return "", nil
	}

	decoded, ok := strings.CutPrefix(encodedScope, encodedSeparatorString)
	if !ok {
		return "", trace.BadParameter("encoded scope did not begin with separator")
	}
	decoded, ok = strings.CutSuffix(decoded, encodedSeparatorString)
	if !ok {
		return "", trace.BadParameter("encoded scope did not end with separator")
	}

	decoded = strings.ReplaceAll(decoded, encodedSeparatorString, separator)

	if err := WeakValidate(decoded); err != nil {
		return "", trace.Wrap(err)
	}

	return decoded, nil
}
