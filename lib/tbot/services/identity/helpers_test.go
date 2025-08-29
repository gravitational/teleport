/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package identity

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

type testYAMLCase[T any] struct {
	name string
	in   T
}

func testYAML[T any](t *testing.T, tests []testYAMLCase[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			encoder := yaml.NewEncoder(b)
			encoder.SetIndent(2)
			require.NoError(t, encoder.Encode(&tt.in))

			if golden.ShouldSet() {
				golden.Set(t, b.Bytes())
			}
			require.Equal(
				t,
				string(golden.Get(t)),
				b.String(),
				"results of marshal did not match golden file, rerun tests with GOLDEN_UPDATE=1",
			)

			unmarshaled, err := internal.UnmarshalYAMLConfig[T](b)
			require.NoError(t, err)
			require.Equal(t, tt.in, *unmarshaled, "unmarshaling did not result in same object as input")
		})
	}
}

type checkAndSetDefaulter interface {
	CheckAndSetDefaults() error
}

type testCheckAndSetDefaultsCase[T checkAndSetDefaulter] struct {
	name string
	in   func() T

	// want specifies the desired state of the checkAndSetDefaulter after
	// check and set defaults has been run. If want is nil, the Output is
	// compared to its initial state.
	want    checkAndSetDefaulter
	wantErr string
}

func testCheckAndSetDefaults[T checkAndSetDefaulter](t *testing.T, tests []testCheckAndSetDefaultsCase[T]) {
	t.Helper()

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
