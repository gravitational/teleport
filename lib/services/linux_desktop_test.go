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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestMarshalUnmarshalLinuxDesktop(t *testing.T) {
	t.Parallel()

	desktop := &linuxdesktopv1.LinuxDesktop{
		Kind:    types.KindLinuxDesktop,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "desktop-1",
			Labels: map[string]string{
				"env": "test",
			},
		},
		Spec: &linuxdesktopv1.LinuxDesktopSpec{
			Addr:     "127.0.0.1:22",
			Hostname: "host",
		},
	}

	out, err := MarshalLinuxDesktop(desktop)
	require.NoError(t, err)

	got, err := UnmarshalLinuxDesktop(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(desktop, got))
}
