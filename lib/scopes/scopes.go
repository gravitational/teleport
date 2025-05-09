/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package scopes

import (
	"fmt"
	"iter"
	"regexp"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

// segmentRegexp is the regular expression used to validate scope segments. It allows
// alphanumeric characters, hyphens, underscores, and periods. It also requires that the
// segment starts and ends with an alphanumeric character.
var segmentRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\_\.]*[a-zA-Z0-9]$`)

const (
	// separator is the character used to separate segments in a scope and is the the value of the root scope.
	separator = "/"

	// exclusiveChildGlobSuffix is a special suffix used in roles to indicate that the role can be
	// assigned to any *child* of a given scope, but not to the scope itself (e.g. an assignable scope of
	// `/aa/**` allows assignment to `/aa/bb`, but not to `/aa`).
	exclusiveChildGlobSuffix = separator + "**"

	// maxScopeSize is the maximum size of a scope, including separators.
	maxScopeSize = 64

	// maxSegmentSize is the maximum size of a segment, excluding separators.
	maxSegmentSize = 32

	// minSegmentSize is the minimum size of a segment, excluding separators.
	minSegmentSize = 2

	// breakingChars is a special set of characters that we explicitly consider to be invalid
	// members even when performing looser/weaker validation. This it intended to be an additional
	// guardrail against erroneous interpretation of future extensions to scope syntax by outdated
	// agents in the event of improper cross-version compat logic.
	breakingChars = "\\@(){}\"'%*?#$!+=|<>,;~`&[]"
)

// ValidateSegment checks if a given scope segment is valid. Generally, one of either [StrongValidate] or
// [WeakValidate] should be called against the entire scope, but this function is useful for validating segments
// prior to performing variable interpolation or other operations that involve building scopes from
// external input. Among other things, using this function in that context is an easy way to prevent
// multi-segment injection in places where a single segment is expected.
func ValidateSegment(segment string) error {
	if segment == "" {
		return trace.BadParameter("segment is empty")
	}

	if len(segment) < minSegmentSize {
		return trace.BadParameter("segment %q is too short (min characters %d)", segment, minSegmentSize)
	}

	if !segmentRegexp.MatchString(segment) {
		return trace.BadParameter("segment %q is malformed", segment)
	}

	if len(segment) > maxSegmentSize {
		return trace.BadParameter("segment %q is too long (max characters %d)", segment, maxSegmentSize)
	}

	return nil
}

// StrongValidate checks if the scope is valid according to all scope formatting rules. This function
// *must* be called on all scope values received from user input and/or cluster-external sources (e.g.
// an identity provider). Use of this function should be avoided when checking the validity of scopes
// from the control-plane in logic that may be run agent-side. Prefer [WeakValidate] in those cases, which
// is more forgiving of changes to scope formatting rules.
func StrongValidate(scope string) error {
	if scope == "" {
		return trace.BadParameter("scope is empty")
	}

	if !strings.HasPrefix(scope, separator) {
		return trace.BadParameter("scope %q is missing required prefix %q", scope, separator)
	}

	if scope != separator && strings.HasSuffix(scope, separator) {
		return trace.BadParameter("scope %q has dangling separator %q", scope, separator)
	}

	for segment := range DescendingSegments(scope) {
		if err := ValidateSegment(segment); err != nil {
			return trace.BadParameter("scope %q is invalid: %v", scope, err)
		}
	}

	if len(scope) > maxScopeSize {
		return trace.BadParameter("scope %q is too long (max characters %d)", scope, maxScopeSize)
	}

	return nil
}

// WeakValidate performs a weak form of validation on a scope. This is useful primarily for ensuring
// that scopes received from trusted sources haven't been altered beyond our ability to reason effectively
// about them (e.g. due to significant version drift). Prefer using [StrongValidate] for scopes received from
// external sources (e.g. user input or identity provider).
func WeakValidate(scope string) error {
	for segment := range DescendingSegments(scope) {
		// check for spaces and control characters
		for _, r := range segment {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return trace.BadParameter("scope %q contains invalid segment %q (whitespace or control character)", scope, segment)
			}
		}

		// check for breaking characters
		if strings.ContainsAny(segment, breakingChars) {
			return trace.BadParameter("scope %q contains invalid segment %q (invalid character)", scope, segment)
		}
	}

	return nil
}

// ValidateGlob checks if a scope glob is valid. A scope glob is a special type of scope that
// may be either a literal or a very simplistic matcher.
func ValidateGlob(scope string) error {
	if scope == "" {
		return trace.BadParameter("scope is empty")
	}

	if scope == exclusiveChildGlobSuffix {
		// this is just a matcher for any child of root
		return nil
	}

	return StrongValidate(strings.TrimSuffix(scope, exclusiveChildGlobSuffix))
}

// DescendingSegments produces an iterator over the segments of a scope in descending order.
// e.g. `DescendingSegments("/a/b/c")` will result in an iterator that returns `a`, `b`, and
// `c` in that order. `DescendingSegments("/")` will return an empty iterator.
//
// Note that this function does not perform validation and is deliberately more relaxed about
// its inputs than our validation functions allow.
func DescendingSegments(scope string) iter.Seq[string] {
	if scope == "" || scope == separator {
		return func(yield func(string) bool) {}
	}
	return strings.SplitSeq(strings.TrimSuffix(strings.TrimPrefix(scope, "/"), "/"), "/")
}

// Join joins the given segments into a single scope string. Note that this function
// does not perform validation and will produce invalid scopes if one or more segments
// are invalid.
func Join(segments ...string) string {
	scope := separator + strings.Join(segments, separator)

	// a tricky bit of how scope splitting/joining works is that an empty scope component
	// as the last component is represented by `/aa//` not `/aa/`, because we chose not to assign trailing
	// separators any meaning (i.e. `/aa/` does not imply the existence of an empty scope segment
	// after `aa`).  Trailing separators and empty scope segments are both considered invalid, but adding
	// this behavior ensures that our splitting/joining logic cannot cannot change the meaning of a scope
	// value when applied to one-another's outputs. If we don't add this logic, then
	// DescendingSegments(Join([]string{"aa", ""}...)) will produce []string{"aa"}. We could just as easily amend
	// `DescendingSegments` to emit an empty segment in the event of a trailing separator, but
	// we judged that to be the higher risk behavior since that would effectively "move" a value to a new
	// scope in the event of a trailing separator making it past validation. Our chosen behavior only "moves" an assignment
	// in the event of a double-separator getting through, which is a much more visually obvious and intuitively incorrect
	// value, and therefore less likely to be hit during processing of user input.
	if len(segments) > 0 && segments[len(segments)-1] == "" {
		scope += separator
	}

	return scope
}

// Relationship describes the relationship between two scopes, as determined by the [Compare] function. Note
// that direct use of this type in access-control logic is discouraged, as it is easier to accidentally
// misuse than the provided helpers (e.g. [PolicyScope]).
type Relationship int

const (
	// Orhogonal indicates that the scopes are divergents/unrelated (e.g. '/foo' and '/bar').
	Orthogonal Relationship = iota
	// Equivalent indicates that the scopes are equal. Some non-equal scope strings are still
	// considered equivalent by [Compare] (e.g. 'foo' and '/foo/'), though the canonical representations
	// (e.g. '/foo') will also have string equality.
	Equivalent
	// Ancestor indicates that one scope is an ancestor of another (including multi-level ancestors,
	// e.g. '/foo' is an ancestor of '/foo/bar' and '/foo/bar/bin', and '/' is an ancestor to all
	// other scope values).
	Ancestor
	// Descendant indicates that one scope is a descendant of another (including multi-level descendants,
	// e.g. '/foo/bar/bin' is a descendant of '/foo/bar' and '/foo', and all other scope values are
	// descendants of '/').
	Descendant
)

// String returns the human-readable representation of the relationship.
func (rel Relationship) String() string {
	switch rel {
	case Orthogonal:
		return "Orthogonal"
	case Equivalent:
		return "Equivalent"
	case Ancestor:
		return "Ancestor"
	case Descendant:
		return "Descendant"
	default:
		return fmt.Sprintf("Unknown(%d)", rel)
	}
}

// Compare compares the relationship between two scopes. The returned value is the
// relationship of the second scope to the first. I.e. if Compare(x, y) returns Ancestor,
// then y is an ancestor of x.
//
// Returns:
//
//	Compare(/aa, /aa)    => Equivalent
//	Compare(/aa, /bb)    => Orthogonal
//	Compare(/aa, /aa/bb) => Descendant
//	Compare(/aa/bb, /aa) => Ancestor
//
// Prefer using one of the provided helpers (e.g. [PolicyScope]) over using this function directly,
// as direct usage of this function can lead to ambiguity and accidental misuse.
//
// Note that this function does not perform validation, and may return unexpected results when
// called against invalid scope values.
func Compare(lhs, rhs string) Relationship {
	lNext, lStop := iter.Pull(DescendingSegments(lhs))
	defer lStop()

	rNext, rStop := iter.Pull(DescendingSegments(rhs))
	defer rStop()

	for {
		lVal, lOk := lNext()
		rVal, rOk := rNext()

		switch {
		case lOk && rOk:
			// both scopes have segments left to compare
			if lVal == rVal {
				// scopes are still equivalent at this level, continue processing
				continue
			}
			// scopes have diverged
			return Orthogonal
		case lOk && !rOk:
			// the right hand side scope is an ancestor of the left hand side scope
			return Ancestor
		case !lOk && rOk:
			// the left hand side scope is an ancestor of the right hand side scope
			return Descendant
		case !lOk && !rOk:
			// scopes are equivalent
			return Equivalent
		}
	}
}

// PolicyScope is a helper for constructing unambiguous checks in access control logic. Prefer helpers like
// this over using the Compare function directly, as it improves readability and reduces the risk of misuse. Ex:
//
//	if scopes.PolicyScope(roleAssignmentScope).AppliesToResourceScope(nodeScope) { ... }
//
// Note that this helper does not perform validation, and may produce unexpected results when used against
// invalid scope values.
type PolicyScope string

// AppliesToResourceScope checks if a resource in the specified scope would be subject to this policy scope.
func (s PolicyScope) AppliesToResourceScope(scope string) bool {
	rel := Compare(string(s), scope)
	return rel == Equivalent || rel == Descendant
}

// ResourceScope is a helper for constructing unambiguous checks in access control logic. Prefer helpers like
// this over using the Compare function directly, as it improves readability and reduces the risk of misuse. Ex:
//
//	if scopes.ResourceScope(nodeScope).IsSubjectToPolicyScope(roleAssignmentScope) { ... }
//
// Note that this helper does not perform validation, and may produce unexpected results when used against
// invalid scope values.
type ResourceScope string

// IsSubjectToPolicyScope checks if this resource scope is subject to the specified policy scope.
func (s ResourceScope) IsSubjectToPolicyScope(scope string) bool {
	rel := Compare(string(s), scope)
	return rel == Equivalent || rel == Ancestor
}

// Glob is a helper for matching scope globs against scopes. This is currently used to support exactly
// one piece of special syntax, the use of `/component/**` to indicate that a role can be assigned to any child of
// the specified scope, but not to the scope itself. Ex:
//
//	if scopes.Glob(assignableScope).Matches(assignmentScope) { ... }
//
// Note that this helper does not perform validation, and may produce unexpected results when used against
// invalid scope or glob values.
type Glob string

// Matches checks if the given scope matches this scope glob.
func (s Glob) Matches(scope string) bool {
	glob := string(s)
	var exclusiveChildMatcher bool
	if strings.HasSuffix(glob, exclusiveChildGlobSuffix) {
		glob = strings.TrimSuffix(glob, exclusiveChildGlobSuffix)
		exclusiveChildMatcher = true
	}

	rel := Compare(glob, scope)
	if exclusiveChildMatcher {
		return rel == Descendant
	} else {
		return rel == Equivalent
	}
}
