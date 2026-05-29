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

	"github.com/gravitational/trace"
)

// Matcher is a compiled path pattern. It matches a wire-form request
// path segment by segment and extracts any named captures. Construct one
// with Compile and reuse it across requests; Match does not mutate it.
type Matcher struct {
	pattern  string
	segments []segment
}

// segKind is the role a single pattern segment plays.
type segKind int

const (
	// segLiteral matches one path segment by exact byte comparison. The
	// leading empty segment (from the pattern's leading "/") and a
	// trailing empty segment (a significant trailing "/") are literals.
	segLiteral segKind = iota
	// segSingle is "*": exactly one non-empty segment, no capture.
	segSingle
	// segCapture is "{name}": exactly one non-empty segment, bound to
	// path.<name> for predicate evaluation.
	segCapture
	// segDoubleStar is "**": zero or more trailing segments. Valid only
	// as the final segment.
	segDoubleStar
)

type segment struct {
	kind    segKind
	literal string // set when kind == segLiteral
	capture string // set when kind == segCapture
}

// Compile parses a path pattern into a Matcher. The pattern must begin
// with "/" and uses three metacharacters, each occupying a whole
// segment:
//
//   - "*"      matches exactly one non-empty segment;
//   - "**"     matches zero or more segments, valid only as the final
//     component; and
//   - "{name}" captures exactly one non-empty segment as path.<name>.
//
// Any other segment is a literal matched by exact bytes. Compile rejects
// a pattern that does not start with "/", places "**" anywhere but last,
// repeats a capture name, uses an invalid capture name, embeds a
// metacharacter in a larger segment, or contains an interior empty
// segment ("//").
func Compile(pattern string) (*Matcher, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern %q must start with '/'", pattern)
	}

	parts := strings.Split(pattern, "/")
	segments := make([]segment, 0, len(parts))
	seen := map[string]struct{}{}
	for i, p := range parts {
		switch {
		case p == "**":
			if i != len(parts)-1 {
				return nil, trace.BadParameter("'**' is only valid as the final segment in path pattern %q", pattern)
			}
			segments = append(segments, segment{kind: segDoubleStar})
		case p == "*":
			segments = append(segments, segment{kind: segSingle})
		case len(p) >= 2 && p[0] == '{' && p[len(p)-1] == '}':
			name := p[1 : len(p)-1]
			if !validCaptureName(name) {
				return nil, trace.BadParameter("invalid capture name %q in path pattern %q", name, pattern)
			}
			if _, ok := seen[name]; ok {
				return nil, trace.BadParameter("duplicate capture name %q in path pattern %q", name, pattern)
			}
			seen[name] = struct{}{}
			segments = append(segments, segment{kind: segCapture, capture: name})
		default:
			if strings.ContainsAny(p, "*{}") {
				return nil, trace.BadParameter("a metacharacter in path pattern %q must occupy a whole segment", pattern)
			}
			if p == "" && i != 0 && i != len(parts)-1 {
				return nil, trace.BadParameter("path pattern %q contains an empty segment ('//')", pattern)
			}
			segments = append(segments, segment{kind: segLiteral, literal: p})
		}
	}
	return &Matcher{pattern: pattern, segments: segments}, nil
}

// Match reports whether path matches the pattern and returns the named
// captures. The returned map is nil when the pattern has no captures or
// the path does not match. Matching is byte-literal and case-sensitive;
// a trailing "/" is significant. The caller is responsible for passing a
// wire-form path that has already cleared ValidateWireform.
func (m *Matcher) Match(path string) (map[string]string, bool) {
	parts := strings.Split(path, "/")

	var captures map[string]string
	for i, seg := range m.segments {
		if seg.kind == segDoubleStar {
			// "**" is always the final segment and matches every
			// remaining path segment, including none.
			return captures, true
		}
		if i >= len(parts) {
			return nil, false
		}
		switch seg.kind {
		case segLiteral:
			if parts[i] != seg.literal {
				return nil, false
			}
		case segSingle:
			if parts[i] == "" {
				return nil, false
			}
		case segCapture:
			if parts[i] == "" {
				return nil, false
			}
			if captures == nil {
				captures = map[string]string{}
			}
			captures[seg.capture] = parts[i]
		}
	}

	// No "**" consumed the tail, so the path must have exactly as many
	// segments as the pattern.
	if len(parts) != len(m.segments) {
		return nil, false
	}
	return captures, true
}

// validCaptureName reports whether name is a usable capture identifier:
// a non-empty run of ASCII letters, digits, and underscores that does
// not start with a digit, so path.<name> is a valid predicate binding.
func validCaptureName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
