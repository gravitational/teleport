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

package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegionsFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		regions []string
		want    string
		wantErr string
	}{
		{
			name:    "no regions means no filter",
			regions: nil,
			want:    "",
		},
		{
			name:    "single region",
			regions: []string{"us-central1"},
			want:    `region="us-central1"`,
		},
		{
			name:    "multiple regions are joined with OR",
			regions: []string{"us-central1", "europe-west1"},
			want:    `region="us-central1" OR region="europe-west1"`,
		},
		{
			name:    "blank region is rejected",
			regions: []string{"  "},
			wantErr: "region cannot be blank",
		},
		{
			name:    "zone is rejected",
			regions: []string{"us-central1-a"},
			wantErr: `invalid region: "us-central1-a"`,
		},
		{
			name:    "uppercase region is rejected",
			regions: []string{"US-CENTRAL1"},
			wantErr: `invalid region: "US-CENTRAL1"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filter, err := regionsFilter(tt.regions)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, filter)
		})
	}
}
