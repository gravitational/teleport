// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package accesslist

import (
	"reflect"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

// topLevelRoleConditionMismatches walks each field of the "got" and "want"
// conditions, compares value and returns a list of mismatched values as
// JSON field names.
//
// This function treats nils and empty values as equivalent eg:
//
//	nil == []string{} or nil == &types.AccessRequestConditions{}
//
// However, since only the top level fields are traversed, nested empty objects
// are considered mismatches eg:
//
//	 &types.AccessRequestConditions{
//		Reason: &types.AccessRequestConditionsReason{},
//	 }
//
// This was a trade off accepted to keep this function simple.
//
// DeepEqual is used where element ordering will matter. But there are assumptions
// made when using this func:
//   - for deny conditions, ANY non-empty deny field is unsupported
//   - for allow conditions, the supported fields from "got" is copied into "want" so
//     ordering will be the same
func topLevelRoleConditionMismatches(want types.RoleConditions, got types.RoleConditions) []string {
	gotValue := reflect.ValueOf(got)
	wantValue := reflect.ValueOf(want)
	gotType := gotValue.Type()

	var mismatches []string
	for i := 0; i < gotValue.NumField(); i++ {
		gotTypeField := gotType.Field(i)

		// Ignore protobuf fields ("got" roles are grpc fetched)
		if strings.HasPrefix(gotTypeField.Name, "XXX_") {
			continue
		}

		gotField := gotValue.Field(i)
		wantField := wantValue.Field(i)
		if isEmptyField(gotField) && isEmptyField(wantField) {
			continue
		}

		if reflect.DeepEqual(gotField.Interface(), wantField.Interface()) {
			continue
		}

		mismatches = append(mismatches, jsonFieldName(gotTypeField))
	}

	return mismatches
}

func nonEmptyGrantFields(got accesslist.Grants) []string {
	gotValue := reflect.ValueOf(got)
	gotType := gotValue.Type()

	mismatches := []string{}
	for i := 0; i < gotValue.NumField(); i++ {
		field := gotType.Field(i)
		gotField := gotValue.Field(i)

		// Role field is checked outside of this func for better
		// error message handling, so it's skipped here.
		if field.Name == "Roles" {
			continue
		}
		if !isEmptyField(gotField) {
			mismatches = append(mismatches, jsonFieldName(field))
		}
	}

	return mismatches
}

func isEmptyField(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Pointer:
		return v.IsNil() || v.Elem().IsZero()
	default:
		return v.IsZero()
	}
}

func jsonFieldName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" || name == "-" {
		return field.Name
	}
	return name
}
