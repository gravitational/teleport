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

package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestInstallConcurrencyLimit(t *testing.T) {
	tests := []struct {
		name   string
		params *types.InstallerParams
		want   int
	}{
		{
			name:   "nil params defaults",
			params: nil,
			want:   50,
		},
		{
			name:   "zero value defaults",
			params: &types.InstallerParams{},
			want:   50,
		},
		{
			name: "uses explicit limit",
			params: &types.InstallerParams{
				InstallConcurrencyLimit: 7,
			},
			want: 7,
		},
		{
			name: "caps at max",
			params: &types.InstallerParams{
				InstallConcurrencyLimit: 9999,
			},
			want: 2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, installConcurrencyLimit(tt.params))
		})
	}
}
