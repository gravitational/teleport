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
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// DecodeConfig controls how a request path is normalized before it is split
// into segments for matching. Both fields default to the zero value, which is
// maximally strict: do not decode, and reject any percent byte. This keeps the
// matcher's view byte-identical to the request, so the agent matches on
// exactly the bytes the upstream receives.
//
// Decoding runs first: DecodeIterations percent-decode passes are applied, and
// then AllowPercent governs whether any percent byte that survives those
// passes is admitted. So the two fields compose: DecodeIterations sets how deep
// to decode, AllowPercent decides whether over-encoding beyond that depth is
// tolerated.
//
//   - decode 0, allow_percent false (default): no decode, no percent at all.
//   - decode 1, allow_percent false: a single-encoded %2F decodes to "/" and
//     is admitted, but a double-encoded %252F leaves a residual "%" after one
//     pass and is rejected.
//   - decode N, allow_percent true: decode N times and admit whatever percent
//     bytes remain.
//
// Loosening is an explicit, deliberate opt-in. The author must align
// DecodeIterations with how the real upstream decodes the path, or the
// matcher's view diverges from the upstream's and a rule can match less, or
// more, than the upstream acts on.
type DecodeConfig struct {
	// AllowPercent admits percent bytes that remain after DecodeIterations
	// passes. When false, any residual "%" is rejected before a rule
	// evaluates.
	AllowPercent bool `yaml:"allow_percent"`
	// DecodeIterations is the number of percent-decode passes applied before
	// splitting into segments. Zero leaves the path untouched. Set it to match
	// the upstream's own decoding.
	DecodeIterations int `yaml:"decode_iterations"`
}

// maxPathBytes bounds the path the matcher will consider.
const maxPathBytes = 8 << 10 // 8 KiB

// maxDecodeIterations bounds the percent-decode passes a path.match call may
// request, so a malformed rule cannot drive an unbounded decode loop. A handful
// of passes covers every real upstream; more is a configuration error.
const maxDecodeIterations = 16

// Tokenize validates and splits an HTTP request path into segments for
// matching, applying cfg. The path is split on the literal "/" byte. A leading
// "/" is required and stripped. On any rule violation it returns an error,
// which the caller treats as teleport_invalid_request: the request is denied
// before any rule runs.
func Tokenize(path string, cfg DecodeConfig) ([]string, error) {
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
	// control byte, or a non-ASCII byte that should have been percent-encoded.
	if err := rejectIllegalPathBytes(path); err != nil {
		return nil, trace.Wrap(err)
	}

	// Decode first, then apply every safety check to the decoded form, since
	// that is the byte sequence that gets split and matched.
	decoded := path
	for range cfg.DecodeIterations {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return nil, trace.BadParameter("invalid percent-escape in %q", path)
		}
		decoded = next
	}

	// AllowPercent governs whatever percent bytes survive the decode passes. A
	// residual "%" with AllowPercent false means the path was encoded more
	// deeply than DecodeIterations unwound, such as a double-encoded %252F
	// under a single pass; reject it.
	if !cfg.AllowPercent && strings.Contains(decoded, "%") {
		return nil, trace.BadParameter("percent-encoding remains after %d decode pass(es) and allow_percent is not set", cfg.DecodeIterations)
	}
	// Reject consecutive slashes outright. An empty interior segment would let
	// a greedy matcher absorb it and could split differently upstream. A
	// single trailing slash is left intact and significant: it produces a
	// trailing empty segment, so "/foo/" simply does not match the pattern
	// "/foo".
	if strings.Contains(decoded, "//") {
		return nil, trace.BadParameter("path %q has consecutive slashes", path)
	}
	if err := rejectUnsafe(decoded); err != nil {
		return nil, trace.Wrap(err)
	}

	segments := strings.Split(strings.TrimPrefix(decoded, "/"), "/")
	for _, seg := range segments {
		if seg == "." || seg == ".." {
			return nil, trace.BadParameter("path %q has a relative segment", path)
		}
	}
	return segments, nil
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
// under RFC 3986. It validates the path as it arrives, before decoding, so a
// percent-encoded byte ("%XX") is admitted here and governed later by the
// decode and allow_percent rules.
func rejectIllegalPathBytes(path string) error {
	for i := range len(path) {
		if !isLegalPathByte(path[i]) {
			return trace.BadParameter("path %q contains an illegal URL byte %q", path, string(path[i]))
		}
	}
	return nil
}

// rejectUnsafe rejects bytes whose interpretation may differ between the agent
// and the upstream and that decoding can surface even when the raw path was
// legal, such as a backslash from %5C or a control byte from %00.
func rejectUnsafe(path string) error {
	if strings.Contains(path, "\\") {
		return trace.BadParameter("path contains a backslash")
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return trace.BadParameter("path contains a control byte")
		}
	}
	return nil
}
