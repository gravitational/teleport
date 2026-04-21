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

package linuxdesktopv1

import (
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

func NewLinuxDesktop(name string, spec *linuxdesktopv1pb.LinuxDesktopSpec) (*linuxdesktopv1pb.LinuxDesktop, error) {
	return &linuxdesktopv1pb.LinuxDesktop{
		Kind:    types.KindLinuxDesktop,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}, nil
}

func ValidateLinuxDesktop(desktop *linuxdesktopv1pb.LinuxDesktop) error {
	switch {
	case desktop == nil:
		return trace.BadParameter("linux desktop is required")
	case desktop.GetMetadata() == nil:
		return trace.BadParameter("metadata is required")
	case desktop.GetMetadata().GetName() == "":
		return trace.BadParameter("name is required")
	case desktop.GetSpec() == nil:
		return trace.BadParameter("spec is required")
	case desktop.GetSpec().GetAddr() == "":
		return trace.BadParameter("spec.addr is required")
	case desktop.GetSpec().GetHostname() == "":
		return trace.BadParameter("spec.hostname is required")
	}
	return nil
}
