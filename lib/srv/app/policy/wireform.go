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

package policy

import (
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"
)

// maxPathBytes caps the wire-form request path. A request whose path
// exceeds this is rejected before any rule evaluates. The limit bounds
// per-request matching cost.
const maxPathBytes = 8 << 10 // 8 KiB

// strictEncodedReserved controls whether percent-encoded reserved
// characters (%2F, %2E, %25) are rejected. This package ships strict.
// Rejecting %25 also covers multi-encoded forms such as %252F, since
// every encoding chain begins with %25. Permissive mode
// (strictEncodedReserved set to false) is not a one-line flip: the
// dot-segment and empty-segment checks below run on the literal
// wire-form bytes, so permissive mode must also run them on the
// decoded form to keep catching %2E / %2E%2E and an encoded // as
// traversal.
const strictEncodedReserved = true

// ValidateWireform reports whether the wire-form request path is safe to
// gate against, applying the path-syntax rejection rules. A non-nil
// error means the request must be denied before any rule evaluates; the
// caller maps that to the teleport_invalid_request reason code.
//
// The path is rejected when it:
//
//   - does not start with "/", which the matcher requires of every
//     pattern and the wire form always satisfies;
//   - exceeds maxPathBytes;
//   - contains a "." or ".." path segment, which an upstream that
//     collapses dot-segments could resolve to a path the policy never
//     evaluated;
//   - contains a "//" (empty interior segment);
//   - contains a backslash, raw or percent-encoded, which a
//     backslash-normalizing upstream resolves to "/" and so to a path
//     the policy never evaluated;
//   - contains a control byte (0x00-0x1F or 0x7F), raw or
//     percent-encoded;
//   - contains an invalid percent-escape (not two hex digits);
//   - decodes to a byte sequence that is not valid UTF-8, which rejects
//     both overlong encodings (such as %C0%AF, an overlong "/" forbidden
//     by RFC 3629) and lone non-UTF-8 octets (such as a Latin-1 %E9); or
//   - contains a percent-encoded reserved character (%2F, %2E, %25)
//     while strictEncodedReserved is set.
//
// Validation reads the wire-form bytes; it does not normalize them. The
// matcher in matcher.go compares the same bytes literally.
func ValidateWireform(path string) error {
	if !strings.HasPrefix(path, "/") {
		return trace.BadParameter("request path must start with '/'")
	}
	if len(path) > maxPathBytes {
		return trace.BadParameter("request path exceeds %d bytes", maxPathBytes)
	}

	// Byte checks run on the decoded form; segment checks on the literal.
	decoded, err := decodeForValidation(path)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, b := range decoded {
		switch {
		case b < 0x20 || b == 0x7F:
			return trace.BadParameter("request path contains a control byte")
		case b == '\\':
			return trace.BadParameter("request path contains a backslash")
		}
	}
	if !utf8.Valid(decoded) {
		return trace.BadParameter("request path is not valid UTF-8")
	}

	parts := strings.Split(path, "/")
	for i, seg := range parts {
		switch {
		case seg == "." || seg == "..":
			return trace.BadParameter("request path contains a %q segment", seg)
		case seg == "" && i != 0 && i != len(parts)-1:
			return trace.BadParameter("request path contains an empty segment ('//')")
		}
	}
	return nil
}

// decodeForValidation walks path and returns its bytes with valid
// percent-escapes decoded to the single byte they denote. It rejects a
// truncated or non-hex escape, and (when strictEncodedReserved is set)
// the encoded reserved characters %2F, %2E, and %25.
func decodeForValidation(path string) ([]byte, error) {
	out := make([]byte, 0, len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c != '%' {
			out = append(out, c)
			continue
		}
		if i+2 >= len(path) || !isHex(path[i+1]) || !isHex(path[i+2]) {
			return nil, trace.BadParameter("request path contains an invalid percent-escape")
		}
		b := unhex(path[i+1])<<4 | unhex(path[i+2])
		if strictEncodedReserved && (b == '/' || b == '.' || b == '%') {
			return nil, trace.BadParameter("request path contains a percent-encoded reserved character")
		}
		out = append(out, b)
		i += 2
	}
	return out, nil
}

func isHex(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		return true
	default:
		return false
	}
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
