// Copyright 2022 Gravitational, Inc
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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSortByFromString(t *testing.T) {
	tests := []struct {
		in  string
		out SortBy
	}{
		{
			"hostname:asc",
			SortBy{
				Field:  "hostname",
				IsDesc: false,
			},
		},
		{
			"",
			SortBy{},
		},
		{
			"name:desc",
			SortBy{
				Field:  "name",
				IsDesc: true,
			},
		},
		{
			"hostname",
			SortBy{
				Field:  "hostname",
				IsDesc: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			out := GetSortByFromString(tt.in)
			require.Equal(t, tt.out, out)
		})
	}
}
