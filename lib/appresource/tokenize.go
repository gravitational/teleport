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

// Package appresource checks whether an HTTP app request is allowed
// by a role. Roles carry allow-only rules. A rule can match on
// request path, HTTP method, and a where predicate over the user
// identity. Every field is optional.
//
// Example role fragment:
//
//	allow:
//	  app_resources:
//	    - paths:
//	        - /api/v4/user/{username}
//	      where: user.name == vars.username
//
// The app agent, role validation, and tctl will use this package.
package appresource

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/text/unicode/norm"
)

// lengthCap bounds the length of a path Tokenize accepts.
const lengthCap = 8 << 10 // 8 KiB

// legalPathPunct is the non-alphanumeric bytes allowed in a raw
// URL path. It is RFC 3986 pchar minus ";", plus "/" and "%".
//
// ";" is dropped because matrix parameters and ";jsessionid" may
// cause the matcher and the upstream app to disagree on where the
// path ends.
const legalPathPunct = "-._~!$&'()*+,=:@/%"

// Tokenize validates and splits an HTTP request path into raw segments
// for matching. The argument is the escaped path as it appears on the
// wire ([net/url.URL.EscapedPath], never the auto-decoded
// [net/url.URL.Path]). Tokenize never decodes what it splits or
// forwards. On any rule violation Tokenize rejects the path and
// returns an error.
//
// The encoded separator is the only ASCII escape allowed.
// Non-ASCII content is allowed as percent-encoded UTF-8, under
// checks that prevent fold-to-different-segment bypasses.
//
//  1. Reject any raw byte that cannot appear in a URL path,
//     including bytes at or above 0x80 (raw unicode).
//  2. Allow only the encoded separator (%2F) and escapes whose
//     decoded byte is at or above 0x80 (non-ASCII unicode). Reject
//     every other ASCII percent-encoding.
//  3. On a decode-for-validation view (%2F to "/"), reject
//     consecutive slashes and "." or ".." segments. A ".."
//     written between encoded slashes ("a%2F..%2Fadmin") is
//     rejected the same as a raw one.
//  4. The path body, with the encoded separator kept literal, must
//     decode to valid, NFKC-stable UTF-8 and contain only graphic
//     runes. "é" is allowed only as "%C3%A9".
//  5. Split on real "/" only. An encoded slash stays one opaque
//     token, hex case preserved.
func Tokenize(path string) ([]string, error) {
	if len(path) > lengthCap {
		return nil, trace.BadParameter("path length %d exceeds the %d byte limit", len(path), lengthCap)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, trace.BadParameter("path %q must start with /", path)
	}
	if err := validatePathBytes(path); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validatePercentEscapes(path); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateDecoded(path); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validatePathContent(path); err != nil {
		return nil, trace.Wrap(err)
	}
	return strings.Split(path[1:], "/"), nil
}

// validatePathBytes rejects any byte that cannot appear in a raw URL
// path under RFC 3986.
func validatePathBytes(path string) error {
	for i := range len(path) {
		if !isLegalPathByte(path[i]) {
			return trace.BadParameter("path %q contains an illegal URL byte %q", path, string(path[i]))
		}
	}
	return nil
}

// isLegalPathByte reports whether a given byte may appear in a raw
// URL path.
func isLegalPathByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte(legalPathPunct, b) >= 0
}

// validatePercentEscapes allows the encoded separator %2F and
// any escape that decodes to a non-ASCII byte. Every other ASCII
// escape is rejected, because an upstream could canonicalize it
// into a structural byte. A "%" without two hex digits is rejected
// as malformed.
func validatePercentEscapes(path string) error {
	for i := 0; i < len(path); i++ {
		if path[i] != '%' {
			continue
		}
		if i+2 >= len(path) {
			return trace.BadParameter("path %q has a truncated percent-escape", path)
		}
		if path[i+1] == '2' && (path[i+2] == 'F' || path[i+2] == 'f') {
			i += 2
			continue
		}
		v, err := strconv.ParseUint(path[i+1:i+3], 16, 8)
		if err != nil {
			return trace.BadParameter("path %q has a malformed percent-escape %q", path, path[i:i+3])
		}
		if byte(v) >= 0x80 {
			i += 2
			continue
		}
		const msg = "path %q contains the percent-escape %q; only the encoded separator %%2F and non-ASCII content escapes are allowed"
		return trace.BadParameter(msg, path, path[i:i+3])
	}
	return nil
}

// validateDecoded rejects "." and ".." segments and consecutive
// slashes on a view where the encoded separator %2F is decoded
// to "/". A trailing slash is kept, so "/foo/" tokenizes to
// ["foo", ""] and does not match "/foo".
func validateDecoded(path string) error {
	decoded := decodeSeparators(path)
	if strings.Contains(decoded, "//") {
		const msg = "path %q has consecutive slashes once the encoded separator %%2F is decoded"
		return trace.BadParameter(msg, path)
	}
	for seg := range strings.SplitSeq(decoded[1:], "/") {
		if seg == "." || seg == ".." {
			const msg = `path %q has a "." or ".." segment once the encoded separator %%2F is decoded`
			return trace.BadParameter(msg, path)
		}
	}
	return nil
}

// decodeSeparators returns the decode-for-validation view of path.
// %2F is unescaped to "/", every other byte is left raw. The ".",
// "..", and "//" checks run on this view, so any of these written
// with an encoded slash is caught the same as a raw one.
func decodeSeparators(path string) string {
	decoded := strings.ReplaceAll(path, "%2F", "/")
	return strings.ReplaceAll(decoded, "%2f", "/")
}

// validatePathContent decodes non-ASCII escapes in the path body,
// keeping the encoded separator literal, and requires valid UTF-8,
// NFKC stability, and a graphic category for every rune of the
// resulting string.
func validatePathContent(path string) error {
	content := decodeNonASCII(path[1:])
	if !utf8.ValidString(content) {
		return trace.BadParameter("path %q is not valid UTF-8 once decoded", path)
	}
	if !norm.NFKC.IsNormalString(content) {
		return trace.BadParameter("path %q is not NFKC-normalized", path)
	}
	for _, r := range content {
		if !isGraphicRune(r) {
			const msg = "path %q contains the disallowed character %q; only letters, marks, numbers, punctuation, and symbols are allowed"
			return trace.BadParameter(msg, path, string(r))
		}
	}
	return nil
}

// decodeNonASCII returns s with every non-ASCII escape (a "%XX"
// that decodes to a non-ASCII byte) replaced by its decoded byte.
// All other bytes, including the encoded separator %2F, are
// left as is.
func decodeNonASCII(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil && byte(v) >= 0x80 {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// isGraphicRune reports whether r is a letter, mark, number,
// punctuation, or symbol. Space and separator categories that
// unicode.IsGraphic allows are excluded, along with control,
// format, surrogate, private-use, and unassigned runes.
func isGraphicRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) ||
		unicode.IsPunct(r) || unicode.IsSymbol(r)
}
