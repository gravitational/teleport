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

package resourcematcher

import (
	"strings"

	"github.com/gravitational/trace"
)

// maxPathBytes bounds the path the matcher will consider.
const maxPathBytes = 8 << 10 // 8 KiB

// Tokenize validates and splits an HTTP request path into raw segments for
// matching. The tokenizer never decodes the bytes it splits or forwards:
// decoding is used only as a throwaway validation view. A leading "/" is
// required and stripped. On any rule violation it returns an error, which the
// caller treats as teleport_invalid_request: the request is denied before any
// rule runs.
//
// The slash is the separator, and it is the only character whose encoding
// changes the matcher's grammar, so it is the only encoding admitted. The steps
// are:
//
//  1. Reject any "%XX" that is not the encoded separator (%2F/%2f). After this,
//     the only encoding present anywhere is the slash.
//  2. Build a decode-for-validation view (%2F to "/") and on that view reject
//     consecutive slashes, a "." or ".." segment, and the bytes the raw form
//     already excludes. Because the whole path is checked uniformly, a ".."
//     smuggled between encoded slashes ("a%2F..%2Fadmin" to "a/../admin") and
//     an empty inner part ("a%2F%2Fb" to "a//b") are caught for free.
//  3. Split the raw path on real "/" only, so an encoded slash stays one opaque
//     token forwarded byte-faithfully.
func Tokenize(path string) ([]string, error) {
	if len(path) > maxPathBytes {
		return nil, trace.BadParameter("path exceeds %d bytes", maxPathBytes)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, trace.BadParameter("path %q must start with /", path)
	}
	// Reject bytes that cannot legally appear in a raw URL path. This is the
	// RFC 3986 path set: pchar plus the "/" separator and "%" for encoding.
	// It is deliberately loose, admitting "@", ":", and every sub-delim, but
	// it refuses what is definitely not a URL path, such as a raw space, a
	// control byte, a backslash, or a non-ASCII byte that should have been
	// percent-encoded.
	if err := rejectIllegalPathBytes(path); err != nil {
		return nil, trace.Wrap(err)
	}
	// The encoded separator is the only percent-escape allowed anywhere. A
	// double-encoded slash ("%252F") begins "%25", which is not "%2F", so it is
	// rejected here, and so is every other encoding such as "%40" or "%2e".
	if err := rejectNonSeparatorPercent(path); err != nil {
		return nil, trace.Wrap(err)
	}

	// Build the decode-for-validation view by unescaping only the separator.
	// Every safety check runs on this view, which is the byte sequence the
	// upstream sees after it decodes the slash, so a "." or ".." or "//"
	// hidden behind an encoded slash is checked the same as a raw one. The raw
	// path is what gets split and forwarded; this view is thrown away.
	decoded := decodeSeparators(path)

	// Reject consecutive slashes outright. An empty interior segment would let
	// a greedy matcher absorb it and could split differently upstream. A
	// single trailing slash is left intact and significant: it produces a
	// trailing empty segment, so "/foo/" simply does not match the pattern
	// "/foo".
	if strings.Contains(decoded, "//") {
		return nil, trace.BadParameter("path %q has consecutive slashes", path)
	}
	for _, seg := range strings.Split(strings.TrimPrefix(decoded, "/"), "/") {
		if seg == "." || seg == ".." {
			return nil, trace.BadParameter("path %q has a relative segment", path)
		}
	}

	// Split the raw path on real "/" only, so an encoded slash stays one token.
	return strings.Split(strings.TrimPrefix(path, "/"), "/"), nil
}

// rejectNonSeparatorPercent rejects any percent-escape that is not the encoded
// separator %2F or %2f. After it returns, the only encoding left anywhere in
// the path is the slash, so the decode-for-validation view need only unescape
// %2F. A "%" without two following bytes is a truncated escape and is rejected.
func rejectNonSeparatorPercent(path string) error {
	for i := 0; i < len(path); i++ {
		if path[i] != '%' {
			continue
		}
		if i+2 >= len(path) {
			return trace.BadParameter("path %q has a truncated percent-escape", path)
		}
		if path[i+1] != '2' || (path[i+2] != 'F' && path[i+2] != 'f') {
			return trace.BadParameter(
				"path %q contains the percent-escape %q; only the encoded separator %%2F is allowed",
				path, path[i:i+3])
		}
	}
	return nil
}

// decodeSeparators returns the decode-for-validation view of path: the encoded
// separator %2F/%2f unescaped to a real "/", with every other byte left raw.
// rejectNonSeparatorPercent has already guaranteed there is no other escape, so
// after this the view carries no percent byte at all.
func decodeSeparators(path string) string {
	decoded := strings.ReplaceAll(path, "%2F", "/")
	return strings.ReplaceAll(decoded, "%2f", "/")
}

// legalPathPunct is the non-alphanumeric byte set allowed in a raw URL path:
// the RFC 3986 sub-delims, the unreserved marks, ":" and "@" from pchar, the
// "/" separator, and "%" for percent-encoding.
const legalPathPunct = "-._~" + "!$&'()*+,;=" + ":@" + "/%"

// isLegalPathByte reports whether b may appear in a raw URL path under
// RFC 3986: an alphanumeric, a pchar punctuation mark, the "/" separator, or
// "%" for encoding.
func isLegalPathByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte(legalPathPunct, b) >= 0
}

// rejectIllegalPathBytes rejects any byte that cannot appear in a raw URL path
// under RFC 3986. It validates the path as it arrives, so a "%" is admitted
// here as the start of an escape and rejectNonSeparatorPercent then governs
// which escapes are allowed.
func rejectIllegalPathBytes(path string) error {
	for i := range len(path) {
		if !isLegalPathByte(path[i]) {
			return trace.BadParameter("path %q contains an illegal URL byte %q", path, string(path[i]))
		}
	}
	return nil
}
