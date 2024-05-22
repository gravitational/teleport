/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
			cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),

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
