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
	"strings"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// validateFieldRulesContainsAnyRule ensures `must_match_fields` contains at
// least one true value assertion that isn't just nested structs. Generic rules
// can be used to evaluate nested structures, so this recurses on the struct
// field to find at least one valid, supported value comparison.
func validateFieldRulesContainsAnyRule(str *types.Struct) (bool, error) {
	for _, v := range str.Fields {
		switch f := v.Kind.(type) {
		case *gogotypes.Value_BoolValue:
			return true, nil
		case *gogotypes.Value_NumberValue:
			return true, nil
		case *gogotypes.Value_StringValue:
			return true, nil
		case *gogotypes.Value_NullValue:
			// Don't treat nulls as sufficient to meet our "one actual value
			// comparison" rule, since they can only assert nonexistence.
			continue
		case *gogotypes.Value_ListValue:
			// List values not currently supported, skip.
			return false, trace.BadParameter("list fields cannot be used in `must_match_fields`")
		case *gogotypes.Value_StructValue:
			// recurse and check the child values to see if there are any actual
			// rules.
			hasAny, err := validateFieldRulesContainsAnyRule(types.NewStruct(f.StructValue))
			if err != nil {
				return false, trace.Wrap(err)
			} else if hasAny {
				return true, nil
			}

			// no default branch, fails closed if nothing matches
		}
	}

	return false, nil
}

// evaluateFieldRules evaluates `must_match_fields` rules defined in the given
// arbitrary spec struct and compares them against the given claims.
func evaluateFieldRules(specStruct *types.Struct, claimStruct map[string]any, path ...string) error {
	if specStruct == nil {
		// Nothing to do
		return nil
	}

	for specKey, specValue := range specStruct.GetFields() {
		fieldPath := append(append([]string{}, path...), specKey)
		identifier := strings.Join(fieldPath, ".")

		claimValue, ok := claimStruct[specKey]
		if !ok {
			if _, isNull := specValue.Kind.(*gogotypes.Value_NullValue); isNull {
				// rules called for this field to be null, so allow the missing
				// field explicitly
				continue
			}

			return trace.CompareFailed("claims missing expected key: %v", identifier)
		}

		switch spec := specValue.Kind.(type) {
		case *gogotypes.Value_BoolValue:
			claimBool, ok := claimValue.(bool)
			if !ok {
				return trace.CompareFailed(
					"field must be a boolean but got %T: %v",
					claimValue, identifier)
			}

			if spec.BoolValue != claimBool {
				return trace.CompareFailed(
					"incorrect value in claim: %v must be %v but got %v",
					identifier, spec.BoolValue, claimBool)
			}
		case *gogotypes.Value_NumberValue:
			claimFloat, ok := claimValue.(float64)
			if !ok {
				return trace.CompareFailed(
					"field must be a number but got %T: %v",
					claimValue, identifier)
			}

			// Hard fail if either side of the comparison is too large to be
			// reliably compared.
			if isUnsafeInteger(claimFloat) {
				return trace.BadParameter("claim contains an integer too large to be safely compared: %s", identifier)
			} else if isUnsafeInteger(spec.NumberValue) {
				return trace.BadParameter("field rule cannot safely compare integers of this size: %s", identifier)
			}

			if spec.NumberValue != claimFloat {
				return trace.CompareFailed(
					"incorrect value in claim: %v must be %v but got %v",
					identifier, spec.NumberValue, claimFloat)
			}
		case *gogotypes.Value_StringValue:
			claimString, ok := claimValue.(string)
			if !ok {
				return trace.CompareFailed(
					"field must be a string but got %T: %v",
					claimValue, identifier)
			}

			if spec.StringValue != claimString {
				return trace.CompareFailed(
					"incorrect value in claim: %v must be %q but got %v",
					identifier, spec.StringValue, claimString)
			}
		case *gogotypes.Value_NullValue:
			// Slightly special handling: we'll accept either a null or a
			// nonexistent value; `null` here can be used to assert no value on
			// the incoming JWT. We won't bother with JS-style truthy values
			// beyond accepting literal nulls, at least for now, so "" / 0 will
			// count as value-ful.

			if claimValue != nil {
				return trace.CompareFailed(
					"incorrect value in claim: %v must be null or unset but had a value",
					identifier)
			}
		case *gogotypes.Value_ListValue:
			return trace.BadParameter("list comparison in %v is not supported, token rules cannot be evaluated", identifier)
		case *gogotypes.Value_StructValue:
			claimNest, ok := claimValue.(map[string]any)
			if !ok {
				return trace.CompareFailed(
					"field must be a nested struct but got %T: %v",
					claimValue, identifier)
			}

			// recurse on the child field
			if err := evaluateFieldRules(types.NewStruct(spec.StructValue), claimNest, fieldPath...); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("unsupported comparison field type %T", spec)
		}
	}

	return nil
}
