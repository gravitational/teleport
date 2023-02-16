// Copyright 2023 Gravitational, Inc
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

package device

import (
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Resource is a wrapper around devicepb.Device that implements types.Resource.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the device specification
	Spec *devicepb.Device `json:"spec"`
}

// CheckAndSetDefaults sanity checks Resource fields to catch simple errors, and
// sets default values for all fields with defaults.
func (r *Resource) CheckAndSetDefaults() error {
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Kind == "" {
		r.Kind = types.KindDevice
	} else if r.Kind != types.KindDevice {
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, types.KindDevice)
	}
	if r.Version == "" {
		r.Version = types.V1
	} else if r.Version != types.V1 {
		return trace.BadParameter("unsupported resource version %q, %q is currently the only supported version", r.Version, types.V1)
	}
	if r.Spec.ApiVersion == "" {
		r.Spec.ApiVersion = types.V1
	} else if r.Spec.ApiVersion != types.V1 {
		return trace.BadParameter("unsupported resource version %q, %q is currently the only supported version", r.Version, types.V1)
	}
	if r.Metadata.Name == "" {
		return trace.BadParameter("device must have a name")
	}
	if r.Spec == nil {
		return trace.BadParameter("device must have a spec")
	}

	return nil
}

// UnmarshalDevice parses a device in the Resource format which matches
// the expected YAML format for Teleport resources, sets default values, and
// converts to *devicepb.Device.
func UnmarshalDevice(raw []byte) (*devicepb.Device, error) {
	var resource Resource
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resourceToProto(&resource), nil
}

// ProtoToResource converts a *devicepb.Device into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(device *devicepb.Device) *Resource {
	r := &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindDevice,
			Version: device.ApiVersion,
			Metadata: types.Metadata{
				Name: device.Id,
			},
		},
		Spec: device,
	}

	return r
}

func resourceToProto(r *Resource) *devicepb.Device {
	return r.Spec
}
