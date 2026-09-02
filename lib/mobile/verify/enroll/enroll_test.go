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

package enroll

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestOSTypeFromModelIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		modelIdentifier string
		want            devicepb.OSType
		wantErr         string
	}{
		{
			name:            "iPhone",
			modelIdentifier: "iPhone15,2",
			want:            devicepb.OSType_OS_TYPE_IOS,
		},
		{
			name:            "iPad",
			modelIdentifier: "iPad15,7",
			want:            devicepb.OSType_OS_TYPE_IPADOS,
		},
		{
			name:            "case insensitive",
			modelIdentifier: "IPAD15,7",
			want:            devicepb.OSType_OS_TYPE_IPADOS,
		},
		{
			name:            "unsupported",
			modelIdentifier: "AppleTV14,1",
			wantErr:         `unsupported model identifier "AppleTV14,1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := osTypeFromModelIdentifier(tt.modelIdentifier)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
