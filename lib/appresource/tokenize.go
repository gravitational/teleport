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
package appresource

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/text/unicode/norm"
)

// maxPathLength bounds the length of a path Tokenize accepts.
const maxPathLength = 8 << 10 // 8 KiB

// legalPathPunct is the non-alphanumeric bytes allowed in a raw
// URL path. It is RFC 3986 pchar except for ";", plus "/" and "%".
//
// ";" is dropped because matrix parameters and ";jsessionid" may
// cause the matcher and the upstream app to disagree on where the
// path ends.
const legalPathPunct = "-._~!$&'()*+,=:@/%"

// Tokenize validates an HTTP request path and splits it on a real
// "/" into the encoded segments a role's path rules match against, so
// an encoded slash stays inside one segment. Pass
// [net/url.URL.EscapedPath], the encoded path sent to the upstream
// app, not the already-decoded [net/url.URL.Path].
//
// Tokenize accepts a path that starts with "/", stays under 8 KiB,
// and holds only the path characters RFC 3986 allows, except for ";".
// Anything else has to be sent percent-encoded, and the only escapes
// allowed are the separator %2F ("/"), the space %20 (" "), and the
// UTF-8 bytes of non-ASCII text. Tokenize also rejects a path an
// upstream app could read as a different path than the one a role
// matched, such as "/a/../b" or "/files/secret." on a server that
// trims a trailing dot.
func Tokenize(path string) ([]string, error) {
	if len(path) > maxPathLength {
		return nil, trace.BadParameter("path length %d exceeds the %d byte limit", len(path), maxPathLength)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, trace.BadParameter("path %q must start with /", clip(path))
	}
	if err := validateRawBytes(path); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateDecoded(path); err != nil {
		return nil, trace.Wrap(err)
	}
	return strings.Split(path[1:], "/"), nil
}

// validateRawBytes rejects any byte that cannot appear in a URL path
// under RFC 3986, any invalid percent-escape, and every escape except
// %2F ("/"), %20 (" "), or one that decodes to a non-ASCII byte.
func validateRawBytes(path string) error {
	for i := 0; i < len(path); i++ {
		if !isLegalPathByte(path[i]) {
			return trace.BadParameter("path %q contains an illegal URL byte %q", clip(path), path[i:i+1])
		}
		if path[i] != '%' {
			continue
		}
		if i+2 >= len(path) {
			return trace.BadParameter("path %q has a truncated percent-escape", clip(path))
		}
		v, err := strconv.ParseUint(path[i+1:i+3], 16, 8)
		if err != nil {
			return trace.BadParameter("path %q has a malformed percent-escape %q", clip(path), path[i:i+3])
		}
		if !isAllowedEscape(byte(v)) {
			const msg = "path %q contains the percent-escape %q; only the encoded separator %%2F, the encoded space %%20, and non-ASCII content escapes are allowed"
			return trace.BadParameter(msg, clip(path), path[i:i+3])
		}
		i += 2
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

// isAllowedEscape reports whether a percent-escape decoding to b may
// appear in a path. [validateRawBytes] accepts exactly the escapes
// [decode] resolves, which are the separator, the space, and
// non-ASCII content.
func isAllowedEscape(b byte) bool {
	return b == '/' || b == ' ' || b >= 0x80
}

// validateDecoded checks the decoded validation view of the path. It
// rejects consecutive slashes, "." and ".." segments, and any content
// that is not NFKC-stable graphic UTF-8.
func validateDecoded(path string) error {
	decoded := decode(path)
	if strings.Contains(decoded, "//") {
		const msg = "path %q has consecutive slashes once the encoded separator %%2F is decoded"
		return trace.BadParameter(msg, clip(path))
	}
	if !utf8.ValidString(decoded) {
		return trace.BadParameter("path %q is not valid UTF-8 once decoded", clip(path))
	}
	for seg := range strings.SplitSeq(decoded[1:], "/") {
		if err := rejectDotSegment(seg); err != nil {
			return trace.Wrap(err)
		}
		if err := rejectLeadingMark(seg); err != nil {
			return trace.Wrap(err)
		}
		if err := rejectEdgeSpace(seg); err != nil {
			return trace.Wrap(err)
		}
		if err := rejectTrailingDot(seg); err != nil {
			return trace.Wrap(err)
		}
	}
	if !norm.NFKC.IsNormalString(decoded) {
		return trace.BadParameter("path %q is not NFKC-normalized", clip(path))
	}
	for _, r := range decoded {
		if !isGraphicRune(r) {
			const msg = "path %q contains the disallowed character %q; only letters, marks, numbers, punctuation, symbols, and the encoded space %%20 are allowed"
			return trace.BadParameter(msg, clip(path), string(r))
		}
	}
	return nil
}

// decode returns the decoded validation view of path s, resolving
// only valid escapes. %2F and %2f become "/", %20 a space, and a
// non-ASCII escape its byte. The view exposes a structural byte
// written as an escape, so "/x%2F..%2Fadmin" is rejected for the same
// reason ".." is rejected in "/x/../admin".
func decode(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil && isAllowedEscape(byte(v)) {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// rejectDotSegment rejects a segment made of only dots and spaces
// that has at least one dot. "." and ".." are traversal segments,
// and an upstream that strips spaces or extra dots would resolve
// forms like ". ." or "..." the same way.
func rejectDotSegment(seg string) error {
	if strings.Trim(seg, ". ") == "" && strings.Contains(seg, ".") {
		const msg = `segment %q is only dots and spaces; an upstream could resolve it as "." or ".."`
		return trace.BadParameter(msg, clip(seg))
	}
	return nil
}

// rejectLeadingMark rejects a segment whose first character composes
// onto the character before it, so "/a/%CC%87b", where %CC%87 is
// U+0307 combining dot above, looks like "/a/b". RFC 5891 bans the
// same form at the start of a domain label.
func rejectLeadingMark(seg string) error {
	if seg == "" || seg[0] < utf8.RuneSelf {
		return nil
	}
	r, _ := utf8.DecodeRuneInString(seg)
	// Some composing characters are not marks, so both checks are needed.
	if unicode.IsMark(r) || !norm.NFKC.PropertiesString(seg).BoundaryBefore() {
		const msg = "segment %q starts with %q, which composes onto the character before it; it must follow a base character"
		return trace.BadParameter(msg, clip(seg), string(r))
	}
	return nil
}

// rejectEdgeSpace rejects a segment whose first or last rune is a
// space. An upstream that trims the segment would see a different
// one, so "..%20" would trim to ".." and "secret%20" to "secret".
func rejectEdgeSpace(seg string) error {
	if strings.HasPrefix(seg, " ") || strings.HasSuffix(seg, " ") {
		const msg = "segment %q starts or ends with a space; a space must be between other characters"
		return trace.BadParameter(msg, clip(seg))
	}
	return nil
}

// rejectTrailingDot rejects a segment whose trailing run of dots and
// spaces contains a dot. IIS and Windows trim trailing dots and
// spaces, so an upstream would resolve "secret." and "secret%20." as
// "secret", a segment the matcher never saw. A leading dot stays
// allowed because paths such as "/.well-known" depend on it.
func rejectTrailingDot(seg string) error {
	trimmed := strings.TrimRight(seg, ". ")
	if strings.Contains(seg[len(trimmed):], ".") {
		const msg = "segment %q ends with dots and spaces; an upstream could trim it to %q"
		return trace.BadParameter(msg, clip(seg), clip(trimmed))
	}
	return nil
}

// isGraphicRune reports whether r is U+0020, a letter, mark, number,
// punctuation, or symbol. U+0020 is allowed because it only reaches
// the decoded view as %20, and [rejectEdgeSpace] keeps it off the
// segment edges. Every other space and separator rune that
// unicode.IsGraphic allows is excluded, along with control, format,
// surrogate, private-use, and unassigned runes. This is the only
// check that rejects the NFKC-stable ones among them, such as U+1680
// ogham space mark.
func isGraphicRune(r rune) bool {
	return r == ' ' || unicode.IsLetter(r) || unicode.IsMark(r) ||
		unicode.IsNumber(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// clip shortens s for use in an error message. A rejected path can be
// kilobytes long, and the message survives into logs and audit events.
func clip(s string) string {
	const limit = 256
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
