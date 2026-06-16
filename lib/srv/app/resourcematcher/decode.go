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
// maximally strict: reject every percent-encoded byte and do not decode. This
// keeps the matcher's view byte-identical to the request, so the agent matches
// on exactly the bytes the upstream receives.
//
// Loosening is an explicit, deliberate opt-in. AllowPercent must be set true
// before any percent-encoding is admitted at all. The author must align
// DecodeIterations with how the real upstream decodes the path, or the
// matcher's view diverges from the upstream's and a rule can match less, or
// more, than the upstream acts on.
type DecodeConfig struct {
	// AllowPercent admits percent-encoded bytes. When false, any "%" in the
	// path is rejected before a rule evaluates.
	AllowPercent bool `yaml:"allow_percent"`
	// DecodeIterations is the number of percent-decode passes applied before
	// splitting into segments. Zero leaves the path untouched. It is only
	// consulted when AllowPercent is true. Set it to match the upstream's own
	// decoding.
	DecodeIterations int `yaml:"decode_iterations"`
}

// maxPathBytes bounds the path the matcher will consider.
const maxPathBytes = 8 << 10 // 8 KiB

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
	if !cfg.AllowPercent && strings.Contains(path, "%") {
		return nil, trace.BadParameter("percent-encoding is not permitted unless allow_percent is set")
	}
	// Reject consecutive slashes outright. An empty interior segment would let
	// a greedy matcher absorb it and could split differently upstream. A
	// single trailing slash is left intact and significant: it produces a
	// trailing empty segment, so "/foo/" simply does not match the pattern
	// "/foo".
	if strings.Contains(path, "//") {
		return nil, trace.BadParameter("path %q has consecutive slashes", path)
	}

	decoded := path
	for range cfg.DecodeIterations {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return nil, trace.BadParameter("invalid percent-escape in %q", path)
		}
		decoded = next
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

// rejectUnsafe rejects bytes whose interpretation may differ between the agent
// and the upstream. This is the strict default the RFD requires before any
// rule evaluates.
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
