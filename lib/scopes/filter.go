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

package scopes

import (
	"github.com/gravitational/trace"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

// IsMatchAll reports whether the given filter is a wildcard match that selects all resources.
func IsMatchAll(filter *scopesv1.Filter) bool {
	// an unspecified filter and MODE_ALL are both treated as a wildcard match at the matching layer. see
	// [MatchScope] for discussion of why these are equivalent here despite being authorized differently
	// at the API/authz layer.
	mode := filter.GetMode()
	return mode == scopesv1.Mode_MODE_UNSPECIFIED || mode == scopesv1.Mode_MODE_ALL
}

// MatchScope reports whether a resource at the given scope matches the supplied filter. This is the
// authoritative scope-matching logic used by caches/backends to decide which resources a filter selects.
//
// The empty string ("") is used to represent an unscoped resource and is treated as orthogonal to all
// scoped values (it is *not* equivalent to the root scope). As a result, the relationship-based modes
// never match unscoped resources, and MODE_UNSCOPED matches only unscoped resources.
//
// Matching logic here treats empty/unspecified filters and MODE_ALL as equivalent wildcard matchers. However,
// the API/authz defaults an empty/unspecified filter to being one of UNSCOPED or EXACT depending on the
// caller's identity. This helps ensure that outdated or un-explicit calls result in safe/conservative defaults.
//
// This function does not validate its inputs. Use [ValidateFilter] to verify that a filter is well-formed
// (e.g. that the scope and mode are mutually consistent) before relying on it.
func MatchScope(filter *scopesv1.Filter, resourceScope string) bool {
	if IsMatchAll(filter) {
		return true
	}

	mode := filter.GetMode()

	if mode == scopesv1.Mode_MODE_UNSCOPED {
		// unscoped resources only.
		return resourceScope == ""
	}

	// all remaining modes select resources by the relationship of the resource scope to the filter scope.
	rel := Compare(filter.GetScope(), resourceScope)
	switch mode {
	case scopesv1.Mode_MODE_EXACT:
		return rel == Equivalent
	case scopesv1.Mode_MODE_DESCENDANTS:
		return rel == Equivalent || rel == Descendant
	case scopesv1.Mode_MODE_ANCESTORS:
		return rel == Equivalent || rel == Ancestor
	case scopesv1.Mode_MODE_RELATIVES:
		return rel != Orthogonal
	default:
		// unknown modes match nothing.
		return false
	}
}

// ValidateFilter checks that a filter is well-formed: that its mode is recognized and that its scope is
// consistent with its mode. Relational modes (EXACT/DESCENDANTS/ANCESTORS/RELATIVES) require a
// non-empty/valid scope, while the non-relational modes (UNSCOPED/ALL) and the unspecified mode
// require an empty scope. A nil or unspecified filter is considered valid.
func ValidateFilter(filter *scopesv1.Filter) error {
	switch filter.GetMode() {
	case scopesv1.Mode_MODE_UNSPECIFIED:
		if filter.GetScope() != "" {
			return trace.BadParameter("scope filter specifies a scope %q without a mode", filter.GetScope())
		}
		return nil
	case scopesv1.Mode_MODE_EXACT,
		scopesv1.Mode_MODE_DESCENDANTS,
		scopesv1.Mode_MODE_ANCESTORS,
		scopesv1.Mode_MODE_RELATIVES:
		if filter.GetScope() == "" {
			return trace.BadParameter("scope filter mode %v requires a non-empty scope", filter.GetMode())
		}
		if err := WeakValidate(filter.GetScope()); err != nil {
			return trace.Wrap(err, "invalid scope in scope filter")
		}
		return nil
	case scopesv1.Mode_MODE_UNSCOPED, scopesv1.Mode_MODE_ALL:
		if filter.GetScope() != "" {
			return trace.BadParameter("scope filter mode %v requires an empty scope, got %q", filter.GetMode(), filter.GetScope())
		}
		return nil
	default:
		return trace.BadParameter("unknown scope filter mode %v", filter.GetMode())
	}
}
