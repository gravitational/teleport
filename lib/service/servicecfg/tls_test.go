// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package servicecfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestTLSModeToProto(t *testing.T) {
	tests := []struct {
		name    string
		m       TLSMode
		want    types.DatabaseTLSMode
		wantErr string
	}{
		{
			name:    "verify CA",
			m:       "verify-ca",
			want:    types.DatabaseTLSMode_VERIFY_CA,
			wantErr: "",
		},
		{
			name:    "verify full",
			m:       "verify-full",
			want:    types.DatabaseTLSMode_VERIFY_FULL,
			wantErr: "",
		},
		{
			name:    "insecure",
			m:       "insecure",
			want:    types.DatabaseTLSMode_INSECURE,
			wantErr: "",
		},
		{
			name:    "empty string, use default",
			m:       "",
			want:    types.DatabaseTLSMode_VERIFY_FULL,
			wantErr: "",
		},
		{
			name:    "invalid",
			m:       "invalid",
			wantErr: `provided invalid TLS mode "invalid". Correct values are: [verify-full verify-ca insecure]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.m.ToProto()
			require.Equal(t, tt.want, got)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
