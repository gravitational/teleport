/*
Copyright 2025 Gravitational, Inc.

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

package events

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
)

func TestStructTrimToMaxSize(t *testing.T) {
	testCases := []struct {
		name    string
		maxSize int
		in      *Struct
		want    *Struct
	}{
		{
			name:    "Field key exceeds max limit size",
			maxSize: 10,
			in: &Struct{
				Struct: types.Struct{
					Fields: map[string]*types.Value{
						strings.Repeat("A", 100): {
							Kind: &types.Value_StringValue{
								StringValue: "A",
							},
						},
					},
				},
			},
			want: &Struct{
				Struct: types.Struct{
					Fields: map[string]*types.Value{
						strings.Repeat("A", 8): {
							Kind: &types.Value_StringValue{
								StringValue: "A",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.trimToMaxSize(tc.maxSize)
			require.True(t, reflect.DeepEqual(got, tc.want))
		})
	}
}
