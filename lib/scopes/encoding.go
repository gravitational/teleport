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
	"bytes"
	"encoding/base32"

	"github.com/gravitational/trace"
)

// The scope key encoding produces a single opaque, order-preserving backend key
// segment for a scope. It is used to namespace scoped resources in the backend,
// e.g. as the <encoded_scope> component of a key like:
//
//	/scoped/<kind>/<encoded_scope>/<name>
//
// The encoding is built in two phases. First, a scope is encoded as a byte string
// with a leading discriminator byte and each segment wrapped in leading/trailing
// null separator. Ex:
//
//	unscoped ("")    -> [ unscopedDisc ]
//	root     ("/")   -> [ scopedDisc ]
//	"/a"             -> [ scopedDisc, sep, 'a', sep ]
//	"/a/b"           -> [ scopedDisc, sep, 'a', sep, sep, 'b', sep ]
//
// The raw bytes are then base32-encoded using a sort-preseving alphabet (see
// scopeKeyEncoding) to produce the final string representation.
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
//     allowed scope characters other than null.

const (
	// scopeKeyUnscopedDisc is the leading discriminator byte for the encoding of
	// an unscoped value (i.e. EncodeForKey("")). It sorts before
	// scopeKeyScopedDisc so that unscoped values sort before all scoped values.
	scopeKeyUnscopedDisc = 0x00

	// scopeKeyScopedDisc is the leading discriminator byte for the encoding of
	// any scoped value (including the root scope "/").
	scopeKeyScopedDisc = 0x01

	// scopeKeySeparator wraps each segment in the raw encoding. It must sort
	// before any byte that can appear in a segment (weak validation guarantees
	// segment bytes are >= 0x21), which is what makes the encoding order- and
	// prefix-preserving.
	scopeKeySeparator = 0x00
)

// scopeKeyEncoding is a custom base32 encoding that uses the lowercase Crockford alphabet, which is
// sort-preserving and omits ambiguous characters.
var scopeKeyEncoding = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

// EncodeForKey encodes a scope so that it will be valid for use in a single
// backend key segment, preserving sort order. If given an empty string, it
// will return a non-empty encoding that sorts before all valid encoded
// scopes.
func EncodeForKey(scope string) (string, error) {
	if scope == "" {
		return scopeKeyEncoding.EncodeToString([]byte{scopeKeyUnscopedDisc}), nil
	}

	if err := WeakValidate(scope); err != nil {
		return "", trace.Wrap(err)
	}

	raw := []byte{scopeKeyScopedDisc}
	for segment := range DescendingSegments(scope) {
		raw = append(raw, scopeKeySeparator)
		raw = append(raw, segment...)
		raw = append(raw, scopeKeySeparator)
	}

	return scopeKeyEncoding.EncodeToString(raw), nil
}

// DecodeFromKey decodes a scope encoded by [EncodeForKey].
func DecodeFromKey(encoded string) (string, error) {
	raw, err := scopeKeyEncoding.DecodeString(encoded)
	if err != nil {
		return "", trace.BadParameter("invalid encoded scope %q: %v", encoded, err)
	}

	if len(raw) == 0 {
		return "", trace.BadParameter("invalid empty encoded scope")
	}

	switch raw[0] {
	case scopeKeyUnscopedDisc:
		if len(raw) != 1 {
			return "", trace.BadParameter("malformed unscoped encoding %q", encoded)
		}
		return "", nil
	case scopeKeyScopedDisc:
		body := raw[1:]
		if len(body) == 0 {
			// scoped discriminator with no segments is the root scope.
			return separator, nil
		}

		var segments []string
		for len(body) > 0 {
			if body[0] != scopeKeySeparator {
				return "", trace.BadParameter("malformed encoded scope %q: expected separator", encoded)
			}
			body = body[1:]

			i := bytes.IndexByte(body, scopeKeySeparator)
			switch {
			case i < 0:
				return "", trace.BadParameter("malformed encoded scope %q: unterminated segment", encoded)
			case i == 0:
				return "", trace.BadParameter("malformed encoded scope %q: empty segment", encoded)
			}
			segments = append(segments, string(body[:i]))
			body = body[i+1:]
		}

		decoded := Join(segments...)
		if err := WeakValidate(decoded); err != nil {
			return "", trace.Wrap(err)
		}
		return decoded, nil
	default:
		return "", trace.BadParameter("invalid scope discriminator %d in encoded scope %q", raw[0], encoded)
	}
}
