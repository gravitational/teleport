/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

type testCheckAndSetDefaultsCase[T Output] struct {
	name string
	in   func() T

	// want specifies the desired state of the Output after check and set
	// defaults has been run. If want is nil, the Output is compared to its
	// initial state.
	want    Output
	wantErr string
}

func memoryDestForTest() bot.Destination {
	return &DestinationMemory{store: map[string][]byte{}}
}

func testCheckAndSetDefaults[T Output](t *testing.T, tests []testCheckAndSetDefaultsCase[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in()
			err := got.CheckAndSetDefaults()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			want := tt.want
			if want == nil {
				want = tt.in()
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}
