// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package databaseobject

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	databaseobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Resource is a type wrapper type for YAML (un)marshaling.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the database object specification
	Spec *databaseobjectv1.DatabaseObjectSpec `json:"spec"`
}

// UnmarshalJSON parses Resource and converts into an object.
func UnmarshalJSON(raw []byte) (*databaseobjectv1.DatabaseObject, error) {
	var resource Resource
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	return ResourceToProto(&resource), nil
}

// ProtoToResource converts a *databaseobjectv1.DatabaseObject into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(obj *databaseobjectv1.DatabaseObject) *Resource {
	r := &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:     obj.GetKind(),
			SubKind:  obj.GetSubKind(),
			Version:  obj.GetVersion(),
			Metadata: types.Resource153ToLegacy(obj).GetMetadata(),
		},
		Spec: obj.GetSpec(),
	}
	return r
}

func ResourceToProto(r *Resource) *databaseobjectv1.DatabaseObject {
	md := r.Metadata

	var expires *timestamppb.Timestamp
	if md.Expires != nil {
		expires = timestamppb.New(*md.Expires)
	}

	return &databaseobjectv1.DatabaseObject{
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
	}
}
