package services

import (
	"context"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
)

// LinuxDesktops is the interface for managing Linux desktop resources.
type LinuxDesktops interface {
	// ListLinuxDesktops returns the Linux desktop resources.
	ListLinuxDesktops(ctx context.Context, pageSize int64, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error)
	// GetLinuxDesktop returns the Linux desktop resource by name.
	GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error)
	// CreateLinuxDesktop creates a new Linux desktop resource.
	CreateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// UpdateLinuxDesktop updates the Linux desktop resource.
	UpdateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// UpsertLinuxDesktop creates or updates the Linux desktop resource.
	UpsertLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error)
	// DeleteLinuxDesktop deletes the Linux desktop resource by name.
	DeleteLinuxDesktop(context.Context, string) error
}

// MarshalLinuxDesktop marshals the LinuxDesktop object into a JSON byte array.
func MarshalLinuxDesktop(object *linuxdesktopv1.LinuxDesktop, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalLinuxDesktop unmarshals the LinuxDesktop object from a JSON byte array.
func UnmarshalLinuxDesktop(data []byte, opts ...MarshalOption) (*linuxdesktopv1.LinuxDesktop, error) {
	return UnmarshalProtoResource[*linuxdesktopv1.LinuxDesktop](data, opts...)
}
