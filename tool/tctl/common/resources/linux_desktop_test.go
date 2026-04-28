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

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

func makeLinuxDesktop(name, addr, hostname string, labels map[string]string) *linuxdesktopv1.LinuxDesktop {
	return &linuxdesktopv1.LinuxDesktop{
		Kind:    types.KindLinuxDesktop,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:   name,
			Labels: labels,
		},
		Spec: &linuxdesktopv1.LinuxDesktopSpec{
			Addr:     addr,
			Hostname: hostname,
		},
	}
}

func TestLinuxDesktopCollection_WriteText(t *testing.T) {
	desktops := []*linuxdesktopv1.LinuxDesktop{
		makeLinuxDesktop("desktop-1", "192.168.1.100:22", "prod-host-1", map[string]string{
			"env": "prod",
		}),
		makeLinuxDesktop("desktop-2", "192.168.1.101:22", "dev-host-1", map[string]string{
			"env": "dev",
		}),
		makeLinuxDesktop("desktop-3", "192.168.1.102:22", "test-host-1", map[string]string{
			"env": "test",
		}),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Address", "Hostname", "Labels"},
		[]string{"desktop-1", "192.168.1.100:22", "prod-host-1", "env=prod"},
		[]string{"desktop-2", "192.168.1.101:22", "dev-host-1", "env=dev"},
		[]string{"desktop-3", "192.168.1.102:22", "test-host-1", "env=test"},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, &linuxDesktopCollection{desktops: desktops}, formatted, formatted)
}

func TestLinuxDesktopCollection_Resources(t *testing.T) {
	t.Parallel()

	desktop := makeLinuxDesktop("test-desktop", "127.0.0.1:22", "test-host", map[string]string{
		"env": "test",
	})

	collection := &linuxDesktopCollection{desktops: []*linuxdesktopv1.LinuxDesktop{desktop}}
	resources := collection.Resources()

	require.Len(t, resources, 1)
	require.Equal(t, types.KindLinuxDesktop, resources[0].GetKind())
	require.Equal(t, "test-desktop", resources[0].GetName())
}

func TestLinuxDesktopHandler(t *testing.T) {
	t.Parallel()

	handler := linuxDesktopHandler()

	require.NotNil(t, handler.getHandler)
	require.NotNil(t, handler.createHandler)
	require.NotNil(t, handler.updateHandler)
	require.NotNil(t, handler.deleteHandler)
	require.False(t, handler.singleton)
	require.False(t, handler.mfaRequired)
	require.Equal(t, "A Linux remote desktop protected by Teleport", handler.description)
}
