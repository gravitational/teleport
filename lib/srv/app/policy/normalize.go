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
	"errors"
	"net/url"
	"path"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeError is returned by Normalize for inputs that fail one of the
// path security rules. The Code is one of the teleport_* reasons.
type NormalizeError struct {
	Code    string
	Message string
}

func (e *NormalizeError) Error() string { return e.Message }

// IsNormalizeError reports whether err is a NormalizeError. If so it
// returns the embedded reason code.
func IsNormalizeError(err error) (string, bool) {
	var ne *NormalizeError
	if errors.As(err, &ne) {
		return ne.Code, true
	}
	return "", false
}

// Normalize parses and normalizes a request URL path.
//
// Returns the normalized path on success, or a *NormalizeError on a
// security-rule failure. Normalize never returns a non-NormalizeError
// error.
func Normalize(rawPath string) (string, error) {
	// A leading "//" causes net/url.Parse to read the next segment as
	// Host. The upstream router collapses "//x" to "/x", so feeding the
	// parser's empty Path to the engine would let "//admin" bypass an
	// "/admin" deny rule.
	if strings.HasPrefix(rawPath, "//") {
		return "", &NormalizeError{
			Code:    ReasonPathDecodeFailed,
			Message: "path must not start with multiple slashes",
		}
	}
	u, err := url.Parse(rawPath)
	if err != nil {
		return "", &NormalizeError{Code: ReasonPathDecodeFailed, Message: "invalid request path"}
	}
	raw := u.EscapedPath()
	if raw == "" {
		raw = u.Path
	}

	current := raw
	// Three rounds catches single, double, and triple percent encoding.
	// Anything still encoded after that is rejected by the post-loop
	// check rather than decoded further.
	for range 3 {
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return "", &NormalizeError{Code: ReasonPathDecodeFailed, Message: "invalid request path"}
		}
		if decoded != current {
			encodedSlashes := strings.Count(current, "%2F") + strings.Count(current, "%2f")
			rawSlashes := strings.Count(current, "/")
			decodedSlashes := strings.Count(decoded, "/")
			if encodedSlashes > 0 && decodedSlashes > rawSlashes {
				return "", &NormalizeError{
					Code:    ReasonEncodedSlashInSegment,
					Message: "encoded path separator decoded into a new segment",
				}
			}
		}
		if decoded == current {
			break
		}
		current = decoded
	}
	if strings.Contains(current, "%2F") || strings.Contains(current, "%2f") {
		return "", &NormalizeError{
			Code:    ReasonDoubleEncodedSlash,
			Message: "path contains encoded slash after 3 decode rounds",
		}
	}
	// Windows-style backends interpret "\" as a path separator. The Go
	// path package does not, so an engine-side "\admin" would not match
	// an "/admin" deny rule while the upstream still routes there.
	// Check after decoding so "%5c" cannot smuggle a backslash past
	// the gate.
	if strings.ContainsRune(current, '\\') {
		return "", &NormalizeError{
			Code:    ReasonPathDecodeFailed,
			Message: "path must not contain backslashes",
		}
	}
	// Java servlet containers and some other stacks treat ";" as a
	// matrix-parameter separator: a request to "/admin;foo/users"
	// routes to the "/admin/users" handler upstream, so feeding the
	// "admin;foo" segment to the engine would skip an "/admin/**"
	// deny rule.
	if strings.ContainsRune(current, ';') {
		return "", &NormalizeError{
			Code:    ReasonPathDecodeFailed,
			Message: "path must not contain semicolons",
		}
	}

	current = norm.NFC.String(current)

	if !strings.HasPrefix(current, "/") {
		current = "/" + current
	}
	// path.Clean collapses repeated slashes and resolves dot-segments.
	current = path.Clean(current)

	// IIS, ASP.NET, and some Java backends strip trailing dots and
	// trailing whitespace from each path segment before routing, so
	// "/admin." or "/admin. " would reach the "/admin" handler upstream
	// while skipping an "/admin" deny rule. Reject rather than strip so
	// the operator sees the bypass attempt.
	for _, seg := range strings.Split(strings.TrimPrefix(current, "/"), "/") {
		if seg == "" {
			continue
		}
		trimmed := strings.TrimRightFunc(seg, unicode.IsSpace)
		if strings.HasSuffix(trimmed, ".") || trimmed != seg {
			return "", &NormalizeError{
				Code:    ReasonPathDecodeFailed,
				Message: "path segment must not end with a dot or whitespace",
			}
		}
	}
	return current, nil
}
