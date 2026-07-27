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

package linuxdesktop

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Resource is a type wrapper type for YAML (un)marshaling.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the database object specification
	Spec Spec `json:"spec"`
}

type Spec struct {
	Addr     string   `json:"addr,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
	ProxyIds []string `json:"proxy_ids,omitempty"`
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
		Spec: Spec{
			Addr:     desktop.GetSpec().GetAddr(),
			Hostname: desktop.GetSpec().GetHostname(),
			ProxyIds: desktop.GetSpec().GetProxyIds(),
		},
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
		Spec: linuxdesktopv1.LinuxDesktopSpec_builder{
			Addr:     r.Spec.Addr,
			Hostname: r.Spec.Hostname,
			ProxyIds: r.Spec.ProxyIds,
		}.Build(),
	}.Build()
}
