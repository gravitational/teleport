package linuxdesktop

import (
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Resource is a type wrapper type for YAML (un)marshaling.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the database object specification
	Spec *linuxdesktopv1.LinuxDesktopSpec `json:"spec"`
}

// UnmarshalJSON parses Resource and converts into an object.
func UnmarshalJSON(raw []byte) (*linuxdesktopv1.LinuxDesktop, error) {
	var resource Resource
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	return ResourceToProto(&resource), nil
}

// ProtoToResource converts a *dbobjectimportrulev1.DatabaseObjectImportRule into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(desktop *linuxdesktopv1.LinuxDesktop) *Resource {
	r := &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:     desktop.GetKind(),
			SubKind:  desktop.GetSubKind(),
			Version:  desktop.GetVersion(),
			Metadata: types.Resource153ToLegacy(desktop).GetMetadata(),
		},
		Spec: desktop.GetSpec(),
	}
	return r
}

func ResourceToProto(r *Resource) *linuxdesktopv1.LinuxDesktop {
	md := r.Metadata

	var expires *timestamppb.Timestamp
	if md.Expires != nil {
		expires = timestamppb.New(*md.Expires)
	}

	return linuxdesktopv1.LinuxDesktop_builder{
		Kind:    r.GetKind(),
		SubKind: r.GetSubKind(),
		Version: r.GetVersion(),
		Metadata: &headerv1.Metadata{
			Name:        md.Name,
			Description: md.Description,
			Namespace:   defaults.Namespace,
			Labels:      md.Labels,
			Expires:     expires,
			Revision:    md.Revision,
		},
		Spec: r.Spec,
	}.Build()
}
