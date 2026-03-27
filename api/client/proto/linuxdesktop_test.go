// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proto

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestPackUnpackLinuxDesktop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desktop *linuxdesktopv1.LinuxDesktop
	}{
		{
			name: "basic linux desktop",
			desktop: &linuxdesktopv1.LinuxDesktop{
				Kind:    types.KindLinuxDesktop,
				SubKind: "",
				Version: types.V3,
				Metadata: &headerv1.Metadata{
					Name: "test-linux-desktop",
					Labels: map[string]string{
						"env":    "production",
						"region": "us-east-1",
					},
				},
				Spec: &linuxdesktopv1.LinuxDesktopSpec{
					Addr:     "192.168.1.100:22",
					Hostname: "linux-host-1",
					ProxyIds: []string{"proxy-1", "proxy-2"},
				},
			},
		},
		{
			name: "linux desktop with no labels",
			desktop: &linuxdesktopv1.LinuxDesktop{
				Kind:    types.KindLinuxDesktop,
				Version: types.V3,
				Metadata: &headerv1.Metadata{
					Name: "minimal-desktop",
				},
				Spec: &linuxdesktopv1.LinuxDesktopSpec{
					Addr:     "example.com:22",
					Hostname: "minimal-host",
				},
			},
		},
		{
			name: "linux desktop with empty proxy IDs",
			desktop: &linuxdesktopv1.LinuxDesktop{
				Kind:    types.KindLinuxDesktop,
				Version: types.V3,
				Metadata: &headerv1.Metadata{
					Name: "no-proxy-desktop",
					Labels: map[string]string{
						"test": "label",
					},
				},
				Spec: &linuxdesktopv1.LinuxDesktopSpec{
					Addr:     "172.16.0.10:22",
					Hostname: "isolated-host",
					ProxyIds: []string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pack the desktop
			packed := PackLinuxDesktop(tt.desktop)
			require.NotNil(t, packed)

			// Verify packed structure
			paginatedResource, ok := packed.(*PaginatedResource_LinuxDesktop)
			require.True(t, ok, "packed resource should be *PaginatedResource_LinuxDesktop")
			require.NotNil(t, paginatedResource.LinuxDesktop)

			// Verify wire format fields
			wireDesktop := paginatedResource.LinuxDesktop
			require.Equal(t, tt.desktop.Kind, wireDesktop.Kind)
			require.Equal(t, tt.desktop.SubKind, wireDesktop.SubKind)
			require.Equal(t, tt.desktop.Version, wireDesktop.Version)
			require.Equal(t, tt.desktop.Metadata.Name, wireDesktop.Metadata.Name)
			require.Equal(t, tt.desktop.Metadata.Labels, wireDesktop.Metadata.Labels)
			require.Equal(t, tt.desktop.Spec.Addr, wireDesktop.Addr)
			require.Equal(t, tt.desktop.Spec.Hostname, wireDesktop.Hostname)
			require.Equal(t, tt.desktop.Spec.ProxyIds, wireDesktop.ProxyIDs)

			// Unpack the desktop
			unpacked := UnpackLinuxDesktop(wireDesktop)
			require.NotNil(t, unpacked)

			// Unwrap to get the original proto type
			unwrapper, ok := unpacked.(types.Resource153UnwrapperT[*linuxdesktopv1.LinuxDesktop])
			require.True(t, ok, "unpacked resource should be unwrappable")

			unpackedDesktop := unwrapper.UnwrapT()
			require.NotNil(t, unpackedDesktop)

			// Verify round-trip equality using protocmp
			// Ignore the Expires field as it's added during metadata conversion
			if diff := cmp.Diff(tt.desktop, unpackedDesktop, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "expires")); diff != "" {
				t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
