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
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

// PackLinuxDesktop packs Linux desktop into its wire format.
func PackLinuxDesktop(desktop *linuxdesktopv1.LinuxDesktop) isPaginatedResource_Resource {
	return &PaginatedResource_LinuxDesktop{
		LinuxDesktop: &LinuxDesktop{
			Kind:     desktop.GetKind(),
			SubKind:  desktop.GetSubKind(),
			Version:  desktop.GetVersion(),
			Metadata: types.Metadata153ToLegacy(desktop.GetMetadata()),
			Addr:     desktop.GetSpec().GetAddr(),
			Hostname: desktop.GetSpec().GetHostname(),
			ProxyIds: desktop.GetSpec().GetProxyIds(),
		},
	}
}

// UnpackLinuxDesktop converts a wire-format LinuxDesktop resource back into an  linuxdesktopv1.LinuxDesktop instance.
func UnpackLinuxDesktop(src *LinuxDesktop) types.ResourceWithLabels {
	spec := &linuxdesktopv1.LinuxDesktopSpec{}
	spec.SetAddr(src.Addr)
	spec.SetHostname(src.Hostname)
	spec.SetProxyIds(src.ProxyIds)

	dst := &linuxdesktopv1.LinuxDesktop{}
	dst.SetKind(src.Kind)
	dst.SetSubKind(src.SubKind)
	dst.SetVersion(src.Version)
	dst.SetMetadata(types.LegacyTo153Metadata(src.Metadata))
	dst.SetSpec(spec)
	return types.ProtoResource153ToLegacy(dst)
}
