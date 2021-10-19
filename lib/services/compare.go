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
	"github.com/gravitational/teleport/api/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// CompareResources compares two resources by all significant fields.
func CompareResources(resA, resB types.Resource) int {
	equal := cmp.Equal(resA, resB,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		cmpopts.IgnoreFields(types.DatabaseV3{}, "Status"),
		cmpopts.EquateEmpty())
	if equal {
		return Equal
	}
	return Different
}
