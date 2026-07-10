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

package genericoidc

import (
	"math"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// isUnsafeInteger determines if a given floating point value is an integer
// (equals itself when truncated) and is too large to be uniquely represented by
// 64bit floats.
func isUnsafeInteger(f float64) bool {
	if f != math.Trunc(f) {
		// Not an integer, allow it
		return false
	}

	// If larger than 2^53-1, reject.
	return math.Abs(f) >= (1 << 53)
}

// allowClaimMatches verifies that the given claim value matches the configured
// token value. We diverge from workloadidentity's string coercion here because
// string conversion of JSON numbers/float64 will not necessarily behave
// correctly, meaning we should perform actual numeric comparison.
func allowClaimMatches(claimValue any, expected string) (bool, error) {
	switch v := claimValue.(type) {
	case string:
		return v == expected, nil
	case bool:
		expectBool, err := strconv.ParseBool(expected)
		if err != nil {
			return false, trace.BadParameter("boolean claim value requires boolean expected value, but got %q", expected)
		}

		return v == expectBool, nil
	case float64:
		expectFloat, err := strconv.ParseFloat(expected, 64)
		if err != nil {
			return false, trace.BadParameter("numeric claim value requires numeric expected value, but got %q", expected)
		}

		// Fail loudly and stop all processing by rejecting with a BadParameter
		// error. We could technically keep processing, but then users would
		// never see the error.
		if isUnsafeInteger(expectFloat) {
			return false, trace.BadParameter("integers of this size cannot be safely compared: %s", expected)
		} else if isUnsafeInteger(v) {
			return false, trace.BadParameter("claim contains an integer value too large for safe comparison: %v", v)
		}

		return v == expectFloat, nil
	default:
		return false, trace.BadParameter("claim of type %T cannot be compared against a scalar expected value", claimValue)
	}
}

// evaluateAllowAnyConditions evaluates `allow_any` conditions (not expressions)
// and returns an error if any rules could not be evaluated, or if access was
// denied due to rule failure. Note that all conditions must match; OR semantics
// are at the top `allow_any` level, not within a conditions block.
func evaluateAllowAnyConditions(conditions []*types.ProvisionTokenSpecV2GenericOIDC_Condition, claims *IDTokenClaims) error {
	for i, cond := range conditions {
		fieldParts := strings.Split(cond.Attribute, ".")
		claimValue, err := getByFields(claims.Claims, fieldParts)
		if err != nil {
			return trace.AccessDenied("required claim attribute %q not found on incoming claims", cond.Attribute)
		}

		// Note, ProvisionTokenV2's checkAndSetDefaults() (or the scoped
		// variant's oneof) ensures only one operator variant is set at creation
		// time, so we can pick any evaluation order here without exhaustive
		// checking.
		switch {
		case cond.Eq != nil:
			matches, err := allowClaimMatches(claimValue, cond.Eq.Value)
			if err != nil {
				return trace.Wrap(err, "conditions[%d]: evaluating `eq` matcher", i)
			}

			if !matches {
				return trace.AccessDenied("conditions[%d]: claim %q did not match required value", i, cond.Attribute)
			}
		case cond.NotEq != nil:
			matches, err := allowClaimMatches(claimValue, cond.NotEq.Value)
			if err != nil {
				return trace.Wrap(err, "conditions[%d]: evaluating `not_eq` matcher", i)
			}

			if matches {
				// Must *not* be ok.
				return trace.AccessDenied("conditions[%d]: claim %q cannot match specified value", i, cond.Attribute)
			}
		case cond.In != nil:
			anyMatch := false
			for _, inValue := range cond.In.Values {
				matches, err := allowClaimMatches(claimValue, inValue)
				if err != nil {
					return trace.Wrap(err, "conditions[%d]: evaluating `in` matcher", i)
				}

				if matches {
					anyMatch = true
					break
				}
			}

			if !anyMatch {
				return trace.AccessDenied("conditions[%d]: claim %q did not match any allowed value", i, cond.Attribute)
			}
		case cond.NotIn != nil:
			anyMatch := false
			for _, inValue := range cond.NotIn.Values {
				matches, err := allowClaimMatches(claimValue, inValue)
				if err != nil {
					return trace.Wrap(err, "conditions[%d]: evaluating `not_in` matcher", i)
				}

				if matches {
					anyMatch = true
					break
				}
			}

			if anyMatch {
				return trace.AccessDenied("conditions[%d]: claim %q cannot match provided value", i, cond.Attribute)
			}
		default:
			return trace.BadParameter("conditions[%d]: no supported operator specified", i)
		}
	}

	return nil
}
