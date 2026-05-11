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

// PathMatcher is a compiled path pattern.
type PathMatcher struct {
	pattern string
	segs    []segment
	tail    bool // pattern ends with **
}

// segment is one component of a compiled pattern.
type segment struct {
	kind segmentKind
	text string
}

type segmentKind int

const (
	segLiteral segmentKind = iota // exact "/foo"
	segStar                       // "*" - exactly one segment
	segCapture                    // "{name}" - capture one segment
)

// CompilePath compiles a pattern into a PathMatcher.
//
//   - "*" matches exactly one non-empty path segment.
//   - "**" matches zero or more segments, only allowed as the last
//     component.
//   - "{name}" captures one non-empty path segment to path.<name>.
//
// Any other use of "*" or "{" returns an error.
func CompilePath(pattern string) (*PathMatcher, error) {
	if pattern == "" {
		return nil, trace.BadParameter("empty path pattern")
	}
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern must start with '/': %q", pattern)
	}
	trimmed := strings.TrimPrefix(pattern, "/")
	// Trailing slash is normalized away in Match, but the pattern itself
	// rejects an explicit trailing slash to keep authors honest.
	if strings.HasSuffix(pattern, "/") && pattern != "/" {
		return nil, trace.BadParameter("path pattern must not end with '/': %q", pattern)
	}

	m := &PathMatcher{pattern: pattern}
	if pattern == "/" {
		return m, nil
	}
	parts := strings.Split(trimmed, "/")
	for i, p := range parts {
		switch {
		case p == "":
			return nil, trace.BadParameter("path pattern has empty segment: %q", pattern)
		case p == "**":
			if i != len(parts)-1 {
				return nil, trace.BadParameter("path pattern: '**' may only appear as the last segment: %q", pattern)
			}
			m.tail = true
		case p == "*":
			m.segs = append(m.segs, segment{kind: segStar})
		case strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}"):
			name := p[1 : len(p)-1]
			if name == "" || strings.ContainsAny(name, "{}*/") {
				return nil, trace.BadParameter("path pattern: invalid capture name in %q", pattern)
			}
			m.segs = append(m.segs, segment{kind: segCapture, text: name})
		default:
			if strings.ContainsAny(p, "*{}") {
				return nil, trace.BadParameter("path pattern: stray metacharacter in segment %q (pattern %q)", p, pattern)
			}
			m.segs = append(m.segs, segment{kind: segLiteral, text: p})
		}
	}
	return m, nil
}

// MustCompilePath compiles a pattern or panics. Intended for tests and
// for callers that supply hand-written patterns at init time.
func MustCompilePath(pattern string) *PathMatcher {
	m, err := CompilePath(pattern)
	if err != nil {
		panic(err)
	}
	return m
}

// Match reports whether path matches m and returns any captures.
// path must be normalized (starts with "/", no trailing "/" except root).
func (m *PathMatcher) Match(path string) (map[string]string, bool) {
	if m.pattern == "/" {
		if path == "/" {
			return nil, true
		}
		return nil, false
	}
	if path == "" {
		return nil, false
	}
	if path == "/" {
		// Only a tail-glob with no fixed prefix segments matches root
		// (e.g. "/**"). Concrete-segment patterns like "/foo" or
		// "/foo/**" do not.
		if m.tail && len(m.segs) == 0 {
			return nil, true
		}
		return nil, false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	switch {
	case m.tail:
		// "**" matches zero or more trailing segments. The path must
		// supply at least the prefix segments preceding the "**".
		if len(parts) < len(m.segs) {
			return nil, false
		}
	default:
		if len(parts) != len(m.segs) {
			return nil, false
		}
	}
	var caps map[string]string
	for i, seg := range m.segs {
		p := parts[i]
		if p == "" {
			return nil, false
		}
		switch seg.kind {
		case segLiteral:
			if p != seg.text {
				return nil, false
			}
		case segStar:
		case segCapture:
			if caps == nil {
				caps = map[string]string{}
			}
			caps[seg.text] = p
		}
	}
	return caps, true
}
