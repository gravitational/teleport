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

	"github.com/gravitational/trace"
)

// segmentRegexp is the regular expression used to validate scope segments. It allows
// alphanumeric characters, hyphens, underscores, and periods. It also requires that the
// segment starts and ends with an alphanumeric character.
var segmentRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\_\.]*[a-zA-Z0-9]$`)

const (
	// separator is the character used to separate segments in a scope and is the the value of the root scope.
	separator = "/"

	// exclusiveChildGlobSegment is the string used to indicate a wildcard match for child segments, used to construct
	// the exclusive child glob suffix.
	exclusiveChildGlobSegment = "**"

	// exclusiveChildGlobSuffix is a special suffix used in roles to indicate that the role can be
	// assigned to any *child* of a given scope, but not to the scope itself (e.g. an assignable scope of
	// `/aa/**` allows assignment to `/aa/bb`, but not to `/aa`).
	exclusiveChildGlobSuffix = separator + exclusiveChildGlobSegment

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
	breakingChars = "\\@(){}\"'%*?#$!+=|<>,;~`&[]/"
)

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
		if err := StrongValidateSegment(segment); err != nil {
			return trace.BadParameter("scope %q is invalid: %v", scope, err)
		}
	}

	if len(scope) > maxScopeSize {
		return trace.BadParameter("scope %q is too long (max characters %d)", scope, maxScopeSize)
	}

	// as an extra precaution, also run all weak checks just to be certain we didn't accidentally
	// construct a weak check that rejects something that would otherwise pass a strong check. strong
	// validation is not used in perf-critical paths, so there isn't any real downside to a little
	// defensiveness here.
	if err := WeakValidate(scope); err != nil {
		return trace.BadParameter("scope would not pass weak validation: %v", err)
	}

	return nil
}

// WeakValidate performs a weak form of validation on a scope. This is useful primarily for ensuring
// that scopes received from trusted sources haven't been altered beyond our ability to reason effectively
// about them (e.g. due to significant version drift). Prefer using [StrongValidate] for scopes received from
// external sources (e.g. user input or identity provider).
func WeakValidate(scope string) error {
	if scope == "" {
		return trace.BadParameter("scope is empty")
	}

	for segment := range DescendingSegments(scope) {
		if err := WeakValidateSegment(segment); err != nil {
			return trace.BadParameter("scope %q is invalid: %v", scope, err)
		}
	}

	return nil
}

// StrongValidateSegment checks if the scope segment is valid according to all scope formatting rules. This function
// *must* be called on all scope segment values received from user input and/or cluster-external sources (e.g.
// an identity provider). Use of this function should be avoided when checking the validity of segments
// from the control-plane in logic that may be run agent-side. Prefer [WeakValidateSegment] in those cases, which
// is more forgiving of changes to scope formatting rules.
func StrongValidateSegment(segment string) error {
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

	// as an extra precaution, also run all weak checks just to be certain we didn't accidentally
	// construct a weak check that rejects something that would otherwise pass a strong check. strong
	// validation is not used in perf-critical paths, so there isn't any real downside to a little
	// defensiveness here.
	if err := WeakValidateSegment(segment); err != nil {
		return trace.BadParameter("segment would not pass weak validation: %v", err)
	}

	return nil
}

// WeakValidateSegment performs a weak form of validation on a scope segment. This is useful primarily for ensuring
// that segments received from trusted sources haven't been altered beyond our ability to reason effectively
// about them (e.g. due to significant version drift). Prefer using [StrongValidateSegment] for segments received from
// external sources (e.g. user input or identity provider).
func WeakValidateSegment(segment string) error {
	// check for spaces and control characters
	for _, b := range []byte(segment) {
		if !isNonSpacePrintableASCII(b) {
			return trace.BadParameter("segment %q contains invalid character", segment)
		}
	}

	// check for breaking characters
	if strings.ContainsAny(segment, breakingChars) {
		return trace.BadParameter("segment %q contains invalid character", segment)
	}

	return nil
}

// isNonSpacePrintableASCII checks if a byte is a non-space printable ASCII character (i.e. a byte in the range
// [33, 126] inclusive). This is used for weak validation of scope segments and globs.
func isNonSpacePrintableASCII(b byte) bool {
	if b < 33 || b > 126 {
		return false
	}

	return true
}

// StrongValidateGlob checks if the scope glob is valid according to all scope formatting rules. This function
// *must* be called on all scope glob values received from user input and/or cluster-external sources (e.g.
// an identity provider). Use of this function should be avoided when checking the validity of scope globs
// from the control-plane in logic that may be run agent-side. Prefer [WeakValidateGlob] in those cases, which
// is more forgiving of changes to scope glob formatting rules.
func StrongValidateGlob(scope string) error {
	if scope == "" {
		return trace.BadParameter("scope glob is empty")
	}

	if scope == exclusiveChildGlobSuffix {
		// this is just a matcher for any child of root
		return nil
	}

	if err := StrongValidate(strings.TrimSuffix(scope, exclusiveChildGlobSuffix)); err != nil {
		return trace.BadParameter("scope glob %q is invalid: %v", scope, err)
	}

	// as an extra precaution, also run all weak checks just to be certain we didn't accidentally
	// construct a weak check that rejects something that would otherwise pass a strong check. strong
	// validation is not used in perf-critical paths, so there isn't any real downside to a little
	// defensiveness here.
	if err := WeakValidateGlob(scope); err != nil {
		return trace.BadParameter("scope glob would not pass weak validation: %v", err)
	}

	return nil
}

// WeakValidateGlob is a weaker form of validation for scope globs. This is useful primarily for ensuring
// that scope globs received from trusted sources haven't been altered beyond our ability to reason effectively
// about them (e.g. due to significant version drift). Prefer using [StrongValidateGlob] for globs received from
// external sources (e.g. user input or identity provider).
func WeakValidateGlob(scope string) error {
	if scope == "" {
		return trace.BadParameter("scope glob is empty")
	}

	for segment := range DescendingSegments(scope) {
		if segment == exclusiveChildGlobSegment {
			continue
		}

		if err := WeakValidateSegment(segment); err != nil {
			return trace.BadParameter("scope glob %q is invalid: %v", scope, err)
		}
	}

	return nil
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
	if lhs == "" || rhs == "" {
		// empty scopes are always orthogonal, including to one-another
		return Orthogonal
	}

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

// PolicyAssignmentScope is a helper for constructing unambiguous checks in access control logic. Prefer helpers like
// this over using the Compare function directly, as it improves readability and reduces the risk of misuse. Ex:
//
//	if scopes.PolicyAssignmentScope(subAssignment.Scope).IsSubjectToPolicyResourceScope(assignment.Scope) { ... }
//
// Note that this helper does not perform validation, and may produce unexpected results when used against
// invalid scope values.
type PolicyAssignmentScope string

// IsSubjectToPolicyResourceScope checks if this policy assignment scope is subject to the specified policy resource
// scope. This is used to validate that the individual assignments within a resource conform to the scoping of the
// overall assignment resource.
func (s PolicyAssignmentScope) IsSubjectToPolicyResourceScope(scope string) bool {
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
	return matchGlob(string(s), scope)
}

// IsSubjectToPolicyResourceScope checks if this glob exclusively matches scopes that would be subject to the
// specified policy resource scope. This is used to validate that the assignable scope globs of a role only
// permit assignment of that role to scopes that could permissibly be subject to the policies defined by that role.
func (s Glob) IsSubjectToPolicyResourceScope(scope string) bool {
	return globIsSubjectToPolicyResourceScope(string(s), scope)
}

// matchGlob implements glob matching. note that this function technically supports some limited
// matching behaviors that we don't actually currently allow the use of. e.g. `/foo/**/bar` would match
// `/foo/baz/bar` (but not  `/foo/baz/bin/bar` or `/foo/bar/`), but we only permit the use of the double-wildcard
// syntax in the trailing segment for simplicity's sake.
func matchGlob(glob string, scope string) bool {
	if glob == "" || scope == "" {
		return false
	}

	gNext, gStop := iter.Pull(DescendingSegments(glob))
	defer gStop()

	sNext, sStop := iter.Pull(DescendingSegments(scope))
	defer sStop()

	for {
		gVal, gOk := gNext()
		sVal, sOk := sNext()

		switch {
		case gOk && sOk:
			// both values have segments left to compare
			if gVal == sVal {
				// segments are equivalent, descend into the next segment
				continue
			}
			if gVal == exclusiveChildGlobSegment {
				// double-wildcard matches any segment, continue
				continue
			}
			// scopes have diverged
			return false
		case gOk && !sOk:
			// the scope is an ancestor of the glob
			return false
		case !gOk && sOk:
			// the glob is an ancestor of the scope
			return true
		case !gOk && !sOk:
			// literal match
			return true
		}
	}
}

// globIsSubjectToPolicyResourceScope checks if the glob is subject to the specified policy resource scope.
func globIsSubjectToPolicyResourceScope(glob string, scope string) bool {
	if glob == "" || scope == "" {
		return false
	}

	gNext, gStop := iter.Pull(DescendingSegments(glob))
	defer gStop()

	sNext, sStop := iter.Pull(DescendingSegments(scope))
	defer sStop()

	for {
		gVal, gOk := gNext()
		sVal, sOk := sNext()

		switch {
		case gOk && sOk:
			// both values have segments left to compare
			if gVal == sVal {
				// segments are equivalent, descend into the next segment
				continue
			}
			if gVal == exclusiveChildGlobSegment {
				// we've hit the first wildcard segment and are still descending
				// through the segments of the scope. this means that the glob may
				// match a scope orthogonal to the resource scope and therefore does
				// not conform to subjugation rules.
				return false
			}
			// scopes have diverged
			return false
		case gOk && !sOk:
			// the scope is an ancestor of the glob, anything the glob matches
			// will be subject to the scope.
			return true
		case !gOk && sOk:
			// the glob is an ancestor of the scope, and is therefore trivially not
			// subject to the scope.
			return false
		case !gOk && !sOk:
			// literal match, the glob is subject by equivalence to the scope.
			return true
		}
	}
}
