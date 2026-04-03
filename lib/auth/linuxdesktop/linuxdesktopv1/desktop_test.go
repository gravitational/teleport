/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package linuxdesktopv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNewLinuxDesktop(t *testing.T) {
	t.Parallel()

	spec := &linuxdesktopv1pb.LinuxDesktopSpec{
		Addr:     "127.0.0.1:22",
		Hostname: "desktop-1",
	}
	desktop, err := NewLinuxDesktop("desktop-1", spec)
	require.NoError(t, err)
	require.Equal(t, types.KindLinuxDesktop, desktop.GetKind())
	require.Equal(t, types.V1, desktop.GetVersion())
	require.Equal(t, "desktop-1", desktop.GetMetadata().GetName())
	require.Equal(t, spec, desktop.GetSpec())
}

func TestValidateLinuxDesktop(t *testing.T) {
	t.Parallel()

	valid := &linuxdesktopv1pb.LinuxDesktop{
		Kind:    types.KindLinuxDesktop,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "desktop-1",
		},
		Spec: &linuxdesktopv1pb.LinuxDesktopSpec{
			Addr:     "127.0.0.1:22",
			Hostname: "host",
		},
	}

	tests := []struct {
		name    string
		desktop *linuxdesktopv1pb.LinuxDesktop
		wantErr bool
	}{
		{
			name:    "valid",
			desktop: valid,
		},
		{
			name:    "nil desktop",
			desktop: nil,
			wantErr: true,
		},
		{
			name: "missing metadata",
			desktop: &linuxdesktopv1pb.LinuxDesktop{
				Spec: valid.Spec,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			desktop: &linuxdesktopv1pb.LinuxDesktop{
				Metadata: &headerv1.Metadata{},
				Spec:     valid.Spec,
			},
			wantErr: true,
		},
		{
			name: "missing spec",
			desktop: &linuxdesktopv1pb.LinuxDesktop{
				Metadata: valid.Metadata,
			},
			wantErr: true,
		},
		{
			name: "missing addr",
			desktop: &linuxdesktopv1pb.LinuxDesktop{
				Metadata: valid.Metadata,
				Spec: &linuxdesktopv1pb.LinuxDesktopSpec{
					Hostname: "host",
				},
			},
			wantErr: true,
		},
		{
			name: "missing hostname",
			desktop: &linuxdesktopv1pb.LinuxDesktop{
				Metadata: valid.Metadata,
				Spec: &linuxdesktopv1pb.LinuxDesktopSpec{
					Addr: "127.0.0.1:22",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLinuxDesktop(tt.desktop)
			if tt.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				return
			}
			require.NoError(t, err)
		})
	}
}
