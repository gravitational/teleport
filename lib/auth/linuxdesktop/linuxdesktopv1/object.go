package linuxdesktopv1

import (
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
