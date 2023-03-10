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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/utils"
)

// Resource is a wrapper around devicepb.Device that implements types.Resource.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource.
	types.ResourceHeader
	// Spec is the device specification.
	Spec ResourceSpec `json:"spec"`
}

// ResourceSpec is the device resource specification.
// This spec is intended to closely mirror [devicepb.Device] but swaps some data around
// to get a UX that matches with our other resource types.
type ResourceSpec struct {
	// OSType is represented as a string here for user-friendly manipulation.
	OSType      string                      `json:"os_type"`
	AssetTag    string                      `json:"asset_tag"`
	CreateTime  time.Time                   `json:"create_time,omitempty"`
	UpdateTime  time.Time                   `json:"update_time,omitempty"`
	EnrollToken *devicepb.DeviceEnrollToken `json:"enroll_token,omitempty"`
	// EnrollStatus is represented as a string here for user-friendly manipulation.
	EnrollStatus  string                     `json:"enroll_status"`
	Credential    *devicepb.DeviceCredential `json:"credential,omitempty"`
	CollectedData []CollectedData            `json:"collected_data,omitempty"`
}

// CollectedData mirrors [devicepb.DeviceCollectedData] but with a different
// timestamp type to achieve consistent serialization output.
type CollectedData struct {
	CollectTime time.Time `json:"collect_time"`
	RecordTime  time.Time `json:"record_time"`
	// OSType is represented as a string here for user-friendly manipulation.
	OSType       string `json:"os_type"`
	SerialNumber string `json:"serial_number,omitempty"`
}

// checkAndSetDefaults sanity checks Resource fields to catch simple errors, and
// sets default values for all fields with defaults.
func (r *Resource) checkAndSetDefaults() error {
	// Assign defaults:
	// - Kind = device
	// - Metadata.Name = UUID
	// - Spec.EnrollStatus = unspecified
	if r.Kind == "" {
		r.Kind = types.KindDevice
	}
	if r.Metadata.Name == "" {
		r.Metadata.Name = uuid.NewString()
	}
	if r.Spec.EnrollStatus == "" {
		r.Spec.EnrollStatus =
			devicetrust.ResourceEnrollStatusToString(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED)
	}

	// Validate Metadata.
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Validate "simple" fields.
	switch {
	case r.Kind != types.KindDevice: // Sanity check.
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, types.KindDevice)
	case r.Spec.OSType == "":
		return trace.BadParameter("missing OS type")
	case r.Spec.AssetTag == "":
		return trace.BadParameter("missing asset tag")
	}

	// Validate enum conversions.
	if _, err := devicetrust.ResourceOSTypeFromString(r.Spec.OSType); err != nil {
		return trace.Wrap(err)
	}
	if _, err := devicetrust.ResourceEnrollStatusFromString(r.Spec.EnrollStatus); err != nil {
		return trace.Wrap(err)
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
	return resourceToProto(&resource)
}

// ProtoToResource converts a *devicepb.Device into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ProtoToResource(device *devicepb.Device) *Resource {
	collectedData := make([]CollectedData, 0, len(device.CollectedData))
	for _, d := range device.CollectedData {
		collectedData = append(collectedData, CollectedData{
			CollectTime:  d.CollectTime.AsTime(),
			RecordTime:   d.RecordTime.AsTime(),
			OSType:       devicetrust.ResourceOSTypeToString(d.OsType),
			SerialNumber: d.SerialNumber,
		})
	}

	r := &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindDevice,
			Version: device.ApiVersion,
			Metadata: types.Metadata{
				Name: device.Id,
			},
		},
		Spec: ResourceSpec{
			OSType:        devicetrust.ResourceOSTypeToString(device.OsType),
			AssetTag:      device.AssetTag,
			EnrollToken:   device.EnrollToken,
			EnrollStatus:  devicetrust.ResourceEnrollStatusToString(device.EnrollStatus),
			Credential:    device.Credential,
			CollectedData: collectedData,
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

func resourceToProto(r *Resource) (*devicepb.Device, error) {
	osType, err := devicetrust.ResourceOSTypeFromString(r.Spec.OSType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enrollStatus, err := devicetrust.ResourceEnrollStatusFromString(r.Spec.EnrollStatus)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	collectedData := make([]*devicepb.DeviceCollectedData, 0, len(r.Spec.CollectedData))
	for _, d := range r.Spec.CollectedData {
		dataOSType, err := devicetrust.ResourceOSTypeFromString(d.OSType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		collectedData = append(collectedData, &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.New(d.CollectTime),
			RecordTime:   timestamppb.New(d.RecordTime),
			OsType:       dataOSType,
			SerialNumber: d.SerialNumber,
		})
	}

	dev := &devicepb.Device{
		ApiVersion:    r.Version,
		Id:            r.Metadata.Name,
		OsType:        osType,
		AssetTag:      r.Spec.AssetTag,
		EnrollToken:   r.Spec.EnrollToken,
		EnrollStatus:  enrollStatus,
		Credential:    r.Spec.Credential,
		CollectedData: collectedData,
	}

	if !r.Spec.CreateTime.IsZero() {
		dev.CreateTime = timestamppb.New(r.Spec.CreateTime)
	}

	if !r.Spec.UpdateTime.IsZero() {
		dev.UpdateTime = timestamppb.New(r.Spec.UpdateTime)
	}

	return dev, nil
}
