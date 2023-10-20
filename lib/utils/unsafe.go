// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"unsafe"

	"github.com/gravitational/trace"
)

// UnsafeSliceData is a wrapper around unsafe.SliceData
// which ensures that instead of ever returning
// "a non-nil pointer to an unspecified memory address"
// (see unsafe.SliceData documentation), an error is
// returned instead.
func UnsafeSliceData[T any](slice []T) (*T, error) {
	if slice == nil || cap(slice) > 0 {
		return unsafe.SliceData(slice), nil
	}

	return nil, trace.BadParameter("non-nil slice had a capacity of zero")
}
