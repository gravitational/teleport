/*
Copyright 2015-2023 Gravitational, Inc.

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

package testlib

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func (s *TerraformSuiteEnterprise) TestTrustedDevices() {
	deviceTrust := modules.GetProtoEntitlement(s.teleportFeatures, entitlements.DeviceTrust)
	require.True(s.T(),
		deviceTrust.Enabled,
		"Test requires Device Trust",
	)

	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	device1 := "teleport_trusted_device.TESTDEVICE1"
	device2 := "teleport_trusted_device.TESTDEVICE2"

	allDevices := []string{device1, device2}

	checkDeviceDestroyed := func(state *terraform.State) error {
		for _, deviceName := range allDevices {
			_, err := s.client.GetDeviceResource(ctx, deviceName)
			switch {
			case err == nil:
				return fmt.Errorf("Device %s was not deleted", deviceName)
			case trace.IsNotFound(err):
				continue
			default:
				return err
			}
		}
		return nil
	}

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDeviceDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("device_trust_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(device1, "spec.asset_tag", "TESTDEVICE1"),
					resource.TestCheckResourceAttr(device1, "spec.os_type", "macos"),
					resource.TestCheckResourceAttr(device1, "spec.enroll_status", "enrolled"),
				),
			},
			{
				Config:   s.getFixture("device_trust_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("device_trust_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(device1, "spec.enroll_status", "not_enrolled"),
					resource.TestCheckResourceAttr(device2, "spec.asset_tag", "TESTDEVICE2"),
					resource.TestCheckResourceAttr(device2, "spec.os_type", "linux"),
					resource.TestCheckResourceAttr(device2, "spec.enroll_status", "not_enrolled"),
				),
			},
			{
				Config:   s.getFixture("device_trust_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportTrustedDevices() {
	deviceTrust := modules.GetProtoEntitlement(s.teleportFeatures, entitlements.DeviceTrust)
	require.True(s.T(),
		deviceTrust.Enabled,
		"Test requires Device Trust",
	)

	ctx := context.Background()

	r := "teleport_trusted_device"
	id := "test_device"
	deviceID := "1a6d1c46-cccf-4f58-8f67-85e6272ebef1"
	name := r + "." + id

	device := &types.DeviceV1{
		ResourceHeader: types.ResourceHeader{
			Kind: "device",
			Metadata: types.Metadata{
				Name: deviceID,
			},
		},
		Spec: &types.DeviceSpec{
			AssetTag:     "DEVICE1",
			OsType:       "macos",
			EnrollStatus: "not_enrolled",
		},
	}

	_, err := s.client.CreateDeviceResource(ctx, device)
	s.Require().NoError(err)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: deviceID,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					s.Require().Equal(deviceID, state[0].Attributes["metadata.name"])
					s.Require().Equal("device", state[0].Attributes["kind"])
					s.Require().Equal("DEVICE1", state[0].Attributes["spec.asset_tag"])
					s.Require().Equal("macos", state[0].Attributes["spec.os_type"])
					s.Require().Equal("not_enrolled", state[0].Attributes["spec.enroll_status"])
					return nil
				},
			},
		},
	})
}
