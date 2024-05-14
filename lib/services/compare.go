/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

// IsEqual[T] will be used instead of cmp.Equal if a resource implements it.
type IsEqual[T any] interface {
	IsEqual(T) bool
}

// CompareResources compares two resources by all significant fields.
func CompareResources[T any](resA, resB T) int {
	var equal bool
	if hasEqual, ok := any(resA).(IsEqual[T]); ok {
		equal = hasEqual.IsEqual(resB)
	} else {
		equal = cmp.Equal(resA, resB,
			ignoreProtoXXXFields(),
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
			cmpopts.IgnoreFields(types.DatabaseV3{}, "Status"),
			cmpopts.IgnoreFields(types.UserSpecV2{}, "Status"),
			cmpopts.IgnoreFields(accesslist.AccessList{}, "Status"),
			cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
			cmpopts.IgnoreUnexported(headerv1.Metadata{}),
			// Managed by IneligibleStatusReconciler, ignored by all others.
			cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "IneligibleStatus"),
			cmpopts.EquateEmpty(),
		)
	}
	if equal {
		return Equal
	}
	return Different
}

// ignoreProtoXXXFields is a cmp.Option that ignores XXX_* fields from proto
// messages.
func ignoreProtoXXXFields() cmp.Option {
	return cmp.FilterPath(func(path cmp.Path) bool {
		if field, ok := path.Last().(cmp.StructField); ok {
			return strings.HasPrefix(field.Name(), "XXX_")
		}
		return false
	}, cmp.Ignore())
}
