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

package local

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestLinuxDesktopServiceCRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	service, err := NewLinuxDesktopService(backend.NewSanitizer(mem))
	require.NoError(t, err)

	desktop := newLinuxDesktop("desktop-1")
	created, err := service.CreateLinuxDesktop(ctx, proto.Clone(desktop).(*linuxdesktopv1.LinuxDesktop))
	require.NoError(t, err)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetLinuxDesktop(ctx, "desktop-1")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(created, got, protocmp.Transform()))

	out, next, err := service.ListLinuxDesktops(ctx, 10, "")
	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, out, 1)

	update := proto.Clone(created).(*linuxdesktopv1.LinuxDesktop)
	update.Spec.Hostname = "updated-host"
	updated, err := service.UpdateLinuxDesktop(ctx, update)
	require.NoError(t, err)
	require.Equal(t, "updated-host", updated.Spec.Hostname)

	require.NoError(t, service.DeleteLinuxDesktop(ctx, "desktop-1"))
	out, _, err = service.ListLinuxDesktops(ctx, 10, "")
	require.NoError(t, err)
	require.Empty(t, out)

	_, err = service.CreateLinuxDesktop(ctx, newLinuxDesktop("desktop-2"))
	require.NoError(t, err)
	_, err = service.CreateLinuxDesktop(ctx, newLinuxDesktop("desktop-3"))
	require.NoError(t, err)
	require.NoError(t, service.DeleteAllLinuxDesktops(ctx))
	out, _, err = service.ListLinuxDesktops(ctx, 10, "")
	require.NoError(t, err)
	require.Empty(t, out)
}

func newLinuxDesktop(name string) *linuxdesktopv1.LinuxDesktop {
	return &linuxdesktopv1.LinuxDesktop{
		Kind:    types.KindLinuxDesktop,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &linuxdesktopv1.LinuxDesktopSpec{
			Addr:     "127.0.0.1:22",
			Hostname: "host",
		},
	}
}
