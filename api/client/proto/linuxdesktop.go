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

// PackLinuxDesktop packs Linux desktop in to its wire format.
func PackLinuxDesktop(desktop *linuxdesktopv1.LinuxDesktop) isPaginatedResource_Resource {
	return &PaginatedResource_LinuxDesktop{
		LinuxDesktop: &LinuxDesktop{
			Kind:     desktop.Kind,
			SubKind:  desktop.SubKind,
			Version:  desktop.Version,
			Metadata: types.Metadata153ToLegacy(desktop.Metadata),
			Addr:     desktop.Spec.Addr,
			Hostname: desktop.Spec.Hostname,
			ProxyIDs: desktop.Spec.ProxyIds,
		},
	}
}

// UnpackLinuxDesktop converts a wire-format LinuxDesktop resource back into an  linuxdesktopv1.LinuxDesktop instance.
func UnpackLinuxDesktop(src *LinuxDesktop) types.ResourceWithLabels {
	dst := &linuxdesktopv1.LinuxDesktop{
		Kind:     src.Kind,
		SubKind:  src.SubKind,
		Version:  src.Version,
		Metadata: types.LegacyTo153Metadata(src.Metadata),
		Spec: &linuxdesktopv1.LinuxDesktopSpec{
			Addr:     src.Addr,
			Hostname: src.Hostname,
			ProxyIds: src.ProxyIDs,
		},
	}
	return types.ProtoResource153ToLegacy(dst)
}
