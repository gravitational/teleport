/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestNewAutoUpdateVersion verifies validation for AutoUpdateVersion resource.
func TestNewAutoUpdateVersion(t *testing.T) {
	tests := []struct {
		name      string
		spec      *autoupdate.AutoUpdateVersionSpec
		want      *autoupdate.AutoUpdateVersion
		assertErr func(*testing.T, error, ...any)
	}{
		{
			name: "success tools autoupdate version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "1.2.3-dev",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateVersion{
				Kind:    types.KindAutoUpdateVersion,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateVersion,
				},
				Spec: &autoupdate.AutoUpdateVersionSpec{
					Tools: &autoupdate.AutoUpdateVersionSpecTools{
						TargetVersion: "1.2.3-dev",
					},
				},
			},
		},
		{
			name: "invalid empty tools version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "TargetVersion is unset")
			},
		},
		{
			name: "invalid semantic tools version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "17-0-0",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "TargetVersion is not a valid semantic version")
			},
		},
		{
			name: "invalid spec",
			spec: nil,
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "Spec is nil")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAutoUpdateVersion(tt.spec)
			tt.assertErr(t, err)
			require.Empty(t, cmp.Diff(got, tt.want, protocmp.Transform()))
		})
	}
}
