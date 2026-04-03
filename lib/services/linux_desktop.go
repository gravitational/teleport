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
	"context"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
)

// LinuxDesktops is the interface for managing Linux desktop resources.
type LinuxDesktops interface {
	LinuxDesktopGetter
	// CreateLinuxDesktop creates a new Linux desktop resource.
	CreateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// UpdateLinuxDesktop updates the Linux desktop resource.
	UpdateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// UpsertLinuxDesktop updates the Linux desktop resource or create one if needed.
	UpsertLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// DeleteLinuxDesktop deletes the Linux desktop resource by name.
	DeleteLinuxDesktop(context.Context, string) error
}

type LinuxDesktopGetter interface {
	// ListLinuxDesktops returns the Linux desktop resources.
	ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error)
	// GetLinuxDesktop returns the Linux desktop resource by name.
	GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error)
}

// MarshalLinuxDesktop marshals the LinuxDesktop object into a JSON byte array.
func MarshalLinuxDesktop(object *linuxdesktopv1.LinuxDesktop, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalLinuxDesktop unmarshals the LinuxDesktop object from a JSON byte array.
func UnmarshalLinuxDesktop(data []byte, opts ...MarshalOption) (*linuxdesktopv1.LinuxDesktop, error) {
	return UnmarshalProtoResource[*linuxdesktopv1.LinuxDesktop](data, opts...)
}
