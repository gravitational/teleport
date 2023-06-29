/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalDevice unmarshals a DeviceV1 resource and runs CheckAndSetDefaults.
func UnmarshalDevice(raw []byte) (*types.DeviceV1, error) {
	dev := &types.DeviceV1{}
	if err := utils.FastUnmarshal(raw, dev); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, trace.Wrap(dev.CheckAndSetDefaults())
}

// MarshalDevice marshals a DeviceV1 resource.
func MarshalDevice(dev *types.DeviceV1) ([]byte, error) {
	devBytes, err := utils.FastMarshal(dev)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return devBytes, nil
}
