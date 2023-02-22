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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Resource is a wrapper around devicepb.Device that implements types.Resource.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the device specification
	Spec ResourceSpec `json:"spec"`
}

// ResourceSpec is the device resource specification.
type ResourceSpec struct {
	// OsType is the device's operating system type.
	OsType string `json:"os_type"`
	// AssetTag is the device's asset tag.
	AssetTag string `json:"asset_tag"`
	// CreateTime is the time the device was created.
	CreateTime time.Time `json:"create_time,omitempty"`
	// UpdateTime is the time the device was last updated.
	UpdateTime time.Time `json:"update_time,omitempty"`
	// EnrollToken is the device's enrollment token.
	EnrollToken *devicepb.DeviceEnrollToken `json:"enroll_token,omitempty"`
	// EnrollStatus is the device's enrollment status.
	EnrollStatus string `json:"enroll_status"`
	// Credential is the device's credential.
	Credential *devicepb.DeviceCredential `json:"credential,omitempty"`
	// CollectedData is the device's collected data.
	CollectedData []*devicepb.DeviceCollectedData `json:"collected_data,omitempty"`
}

// checkAndSetDefaults sanity checks Resource fields to catch simple errors, and
// sets default values for all fields with defaults.
func (r *Resource) checkAndSetDefaults() error {
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Kind == "" {
		r.Kind = types.KindDevice
		// Sanity check.
	} else if r.Kind != types.KindDevice {
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, types.KindDevice)
	}

	if _, ok := devicepb.OSType_value[r.Spec.OsType]; !ok {
		return trace.BadParameter("invalid OS type: %q", r.Spec.OsType)
	}

	if r.Spec.AssetTag == "" {
		return trace.BadParameter("missing asset tag")
	}

	if _, ok := devicepb.DeviceEnrollStatus_value[r.Spec.EnrollStatus]; !ok {
		return trace.BadParameter("invalid enrollment status: %q", r.Spec.EnrollStatus)
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
	if err := resource.checkAndSetDefaults(); err != nil {
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
				Name: device.ID,
			},
		},
		Spec: ResourceSpec{
			OsType:        devicepb.OSType_name[int32(device.OsType)],
			AssetTag:      device.AssetTag,
			EnrollToken:   device.EnrollToken,
			EnrollStatus:  devicepb.DeviceEnrollStatus_name[int32(device.EnrollStatus)],
			Credential:    device.Credential,
			CollectedData: device.CollectedData,
		},
	}

	if device.CreateTime != nil {
		r.Spec.CreateTime = device.CreateTime.AsTime()
	}

	if device.UpdateTime != nil {
		r.Spec.UpdateTime = device.UpdateTime.AsTime()
	}

	return r
}

func resourceToProto(r *Resource) *devicepb.Device {
	dev := &devicepb.Device{
		ApiVersion:    r.Version,
		Id:            r.Metadata.Name,
		OsType:        devicepb.OSType(devicepb.OSType_value[r.Spec.OsType]),
		AssetTag:      r.Spec.AssetTag,
		EnrollToken:   r.Spec.EnrollToken,
		EnrollStatus:  devicepb.DeviceEnrollStatus(devicepb.DeviceEnrollStatus_value[r.Spec.EnrollStatus]),
		Credential:    r.Spec.Credential,
		CollectedData: r.Spec.CollectedData,
	}

	if !r.Spec.CreateTime.IsZero() {
		dev.CreateTime = timestamppb.New(r.Spec.CreateTime)
	}

	if !r.Spec.UpdateTime.IsZero() {
		dev.UpdateTime = timestamppb.New(r.Spec.UpdateTime)
	}

	return dev
}
