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
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/text/unicode/norm"
)

// maxPathBytes bounds the path the matcher will consider.
const maxPathBytes = 8 << 10 // 8 KiB

// Tokenize validates and splits an HTTP request path into raw segments for
// matching. The tokenizer never decodes the bytes it splits or forwards: a
// decode is used only as a throwaway validation view. A leading "/" is required
// and stripped. On any rule violation it returns an error, which the caller
// treats as teleport_invalid_request: the request is denied before any rule
// runs.
//
// The slash is the separator, and it is the only character whose encoding
// changes the matcher's grammar, so the encoded separator is the only ASCII
// escape admitted. Non-ASCII content is admitted as percent-encoded UTF-8,
// because real upstream APIs (GitHub, GitLab) carry unicode file and ref names
// that way, but only under the content checks that close the
// fold-to-a-different-segment bypass class. The steps are:
//
//  1. Reject any raw byte that cannot legally appear in a URL path, including
//     every raw byte at or above 0x80: non-ASCII must arrive percent-encoded.
//  2. Validate every "%XX" escape: admit the encoded separator (%2F/%2f) and any
//     escape whose decoded byte is at or above 0x80 (non-ASCII content). Reject
//     every other ASCII escape, such as %2E or %25, since it could fold to a
//     structural byte upstream.
//  3. Build a decode-for-validation view (%2F to "/") and on that view reject
//     consecutive slashes and a "." or ".." segment. Because the whole path is
//     checked uniformly, a ".." smuggled between encoded slashes
//     ("a%2F..%2Fadmin" to "a/../admin") and an empty inner part
//     ("a%2F%2Fb" to "a//b") are caught for free.
//  4. Validate the content of each real "/" segment: decode the non-ASCII
//     escapes once, require valid UTF-8 (kills overlong and surrogate forms),
//     require NFKC-stable (kills fullwidth and decomposed look-alikes), and
//     require every rune to be a graphic character (a letter, mark, number,
//     punctuation, or symbol). Together these guarantee a request can carry "é"
//     raw or as "%C3%A9" and always land on the same canonical segment.
//  5. Split the raw path on real "/" only, so an encoded slash stays one opaque
//     token forwarded byte-faithfully. The hex case is preserved here: an
//     encoded-slash capture binds the decoded value (see kindCaptureEncoded),
//     so "%2F" and "%2f" bind the same string without rewriting the wire bytes.
func Tokenize(path string) ([]string, error) {
	if len(path) > maxPathBytes {
		return nil, trace.BadParameter("path exceeds %d bytes", maxPathBytes)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, trace.BadParameter("path %q must start with /", path)
	}
	// Reject bytes that cannot legally appear in a raw URL path. This is the
	// RFC 3986 path set less ";": pchar plus the "/" separator and "%" for
	// encoding. It is deliberately loose, admitting "@", ":", and the sub-delims
	// other than ";", but it refuses what is definitely not a safe URL path,
	// such as a raw space, a control byte, a backslash, a raw byte at or above
	// 0x80, which must arrive percent-encoded, or the ";" matrix-parameter
	// vector.
	if err := rejectIllegalPathBytes(path); err != nil {
		return nil, trace.Wrap(err)
	}
	// Govern which percent-escapes are admitted: the encoded separator and any
	// non-ASCII content escape. A double-encoded slash ("%252F") begins "%25",
	// an ASCII escape that is not the separator, so it is rejected here, and so
	// is every other ASCII escape such as "%40" or "%2e".
	if err := validatePercentEscapes(path); err != nil {
		return nil, trace.Wrap(err)
	}

	// Build the decode-for-validation view by unescaping only the separator.
	// The structural checks run on this view, which is the byte sequence the
	// upstream sees after it decodes the slash, so a "." or ".." or "//" hidden
	// behind an encoded slash is checked the same as a raw one. The raw path is
	// what gets split and forwarded; this view is thrown away.
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

	// Validate the unicode content of each segment, so a percent-encoded
	// non-ASCII byte sequence that is overlong, not normalized, or in a
	// dangerous category is rejected before it can fold into a different
	// segment upstream.
	if err := validatePathContent(path); err != nil {
		return nil, trace.Wrap(err)
	}

	// Split the raw path on real "/" only, so an encoded slash stays one token
	// forwarded byte-faithfully. The bytes are not rewritten: an encoded-slash
	// capture binds the decoded value, so the hex case never reaches a vars
	// comparison, and the forwarded path stays exactly what the client sent.
	return strings.Split(strings.TrimPrefix(path, "/"), "/"), nil
}

// validatePercentEscapes governs which percent-escapes a request path may carry.
// It admits the encoded separator %2F/%2f and any escape whose decoded byte is
// at or above 0x80, the percent-encoded UTF-8 a unicode resource name arrives
// as. Every other ASCII escape is rejected, because an upstream could decode it
// and canonicalize the result into a structural byte the matcher never saw. A
// "%" without two following hex digits is a truncated or malformed escape and is
// rejected. The assembled non-ASCII bytes are checked for valid UTF-8 and
// normalization separately, by validatePathContent.
func validatePercentEscapes(path string) error {
	for i := 0; i < len(path); i++ {
		if path[i] != '%' {
			continue
		}
		if i+2 >= len(path) {
			return trace.BadParameter("path %q has a truncated percent-escape", path)
		}
		hi, lo := unhex(path[i+1]), unhex(path[i+2])
		if hi < 0 || lo < 0 {
			return trace.BadParameter("path %q has a malformed percent-escape %q", path, path[i:i+3])
		}
		// The encoded separator is admitted and kept as a token.
		if path[i+1] == '2' && (path[i+2] == 'F' || path[i+2] == 'f') {
			i += 2
			continue
		}
		// A non-ASCII content escape is admitted: it is decoded once and the
		// assembled bytes are validated for UTF-8, normalization, and category
		// by validatePathContent.
		if byte(hi<<4|lo) >= 0x80 {
			i += 2
			continue
		}
		return trace.BadParameter(
			"path %q contains the percent-escape %q; only the encoded separator %%2F and non-ASCII content escapes are allowed",
			path, path[i:i+3])
	}
	return nil
}

// validatePathContent validates the unicode content of each real "/" segment.
// It decodes the non-ASCII escapes once, leaving the encoded separator as a
// literal token, and on the assembled bytes requires valid UTF-8, NFKC
// stability, and a graphic category for every rune. Together these reject the
// fold-to-a-different-segment bypasses: overlong UTF-8 of "/" ("%C0%AF") fails
// the UTF-8 check, a fullwidth solidus "／" (U+FF0F) or fullwidth "ａ" (U+FF41)
// fails the NFKC check because it folds to "/" or "a", and a control, format,
// or separator rune fails the category check. A plain accented name such as
// "café" is NFKC-stable, so it is admitted.
func validatePathContent(path string) error {
	for _, seg := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		content := decodeNonASCII(seg)
		if !utf8.ValidString(content) {
			return trace.BadParameter("path %q has a segment %q that is not valid UTF-8 once decoded", path, seg)
		}
		if !norm.NFKC.IsNormalString(content) {
			return trace.BadParameter(
				"path %q has a segment %q that is not NFKC-normalized; a look-alike that folds to another character is rejected", path, seg)
		}
		for _, r := range content {
			if !isGraphicRune(r) {
				return trace.BadParameter(
					"path %q has a segment %q with the disallowed character %q; only letters, marks, numbers, punctuation, and symbols are allowed",
					path, seg, string(r))
			}
		}
	}
	return nil
}

// decodeNonASCII returns seg with every non-ASCII escape (a "%XX" whose decoded
// byte is at or above 0x80) replaced by the raw decoded byte, and every other
// byte, including the encoded separator %2F/%2f, left as is. validatePercentEscapes
// has already guaranteed the only escapes present are the separator and non-ASCII
// content, so the decoded byte sequence is the assembled UTF-8 content the
// normalization and category checks then validate.
func decodeNonASCII(seg string) string {
	if !strings.ContainsRune(seg, '%') {
		return seg
	}
	var b strings.Builder
	b.Grow(len(seg))
	for i := 0; i < len(seg); i++ {
		if seg[i] == '%' && i+2 < len(seg) {
			hi, lo := unhex(seg[i+1]), unhex(seg[i+2])
			if hi >= 0 && lo >= 0 {
				if v := byte(hi<<4 | lo); v >= 0x80 {
					b.WriteByte(v)
					i += 2
					continue
				}
			}
		}
		b.WriteByte(seg[i])
	}
	return b.String()
}

// isGraphicRune reports whether r is a graphic character the path content
// allows: a letter, mark, number, punctuation, or symbol. It deliberately
// excludes the space and separator categories that unicode.IsGraphic admits, as
// well as control, format, surrogate, private-use, and unassigned runes, so a
// zero-width or bidi-control rune cannot ride inside a segment.
func isGraphicRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) ||
		unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// unhex returns the value of a hex digit, or -1 if b is not a hex digit.
func unhex(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}

// decodeSeparators returns the decode-for-validation view of path: the encoded
// separator %2F/%2f unescaped to a real "/", with every other byte left raw.
// The structural checks run on this view, so a "." or ".." or "//" hidden
// behind an encoded slash is caught the same as a raw one.
func decodeSeparators(path string) string {
	decoded := strings.ReplaceAll(path, "%2F", "/")
	return strings.ReplaceAll(decoded, "%2f", "/")
}

// legalPathPunct is the non-alphanumeric byte set allowed in a raw URL path:
// the RFC 3986 sub-delims less ";", the unreserved marks, ":" and "@" from
// pchar, the "/" separator, and "%" for percent-encoding. The ";" is dropped
// on purpose: it is a known path-parsing-bug vector, the matrix-parameter and
// ";jsessionid" confusion where the matcher and the upstream app disagree on
// where the path ends, so a raw semicolon is rejected as an invalid request.
const legalPathPunct = "-._~" + "!$&'()*+,=" + ":@" + "/%"

// isLegalPathByte reports whether b may appear in a raw URL path under
// RFC 3986: an alphanumeric, a pchar punctuation mark, the "/" separator, or
// "%" for encoding. A byte at or above 0x80 is never legal raw: non-ASCII must
// arrive percent-encoded.
func isLegalPathByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte(legalPathPunct, b) >= 0
}

// rejectIllegalPathBytes rejects any byte that cannot appear in a raw URL path
// under RFC 3986. It validates the path as it arrives, so a "%" is admitted
// here as the start of an escape and validatePercentEscapes then governs which
// escapes are allowed.
func rejectIllegalPathBytes(path string) error {
	for i := range len(path) {
		if !isLegalPathByte(path[i]) {
			return trace.BadParameter("path %q contains an illegal URL byte %q", path, string(path[i]))
		}
	}
	return nil
}
