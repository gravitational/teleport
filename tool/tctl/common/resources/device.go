/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/services"
)

type deviceCollection struct {
	devices []*devicepb.Device
}

func (c *deviceCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.devices))
	for i, dev := range c.devices {
		resources[i] = types.DeviceToResource(dev)
	}
	return resources
}

func (c *deviceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"ID", "OS Type", "Asset Tag", "Enrollment Status", "Creation Time", "Last Updated"})
	for _, device := range c.devices {
		t.AddRow([]string{
			device.GetId(),
			devicetrust.FriendlyOSType(device.GetOsType()),
			device.GetAssetTag(),
			devicetrust.FriendlyDeviceEnrollStatus(device.GetEnrollStatus()),
			device.GetCreateTime().AsTime().Format(time.RFC3339),
			device.GetUpdateTime().AsTime().Format(time.RFC3339),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func deviceHandler() Handler {
	return Handler{
		getHandler:    getDevice,
		createHandler: createDevice,
		deleteHandler: deleteDevice,
		description:   "Represents a device enrolled in Device Trust.",
	}
}

func getDevice(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	remote := client.DevicesClient()
	if ref.Name != "" {
		resp, err := remote.FindDevices(ctx, devicepb.FindDevicesRequest_builder{
			IdOrTag: ref.Name,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &deviceCollection{resp.GetDevices()}, nil
	}

	req := devicepb.ListDevicesRequest_builder{
		View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
	}.Build()
	var devs []*devicepb.Device
	for {
		resp, err := remote.ListDevices(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		devs = append(devs, resp.GetDevices()...)

		if resp.GetNextPageToken() == "" {
			break
		}
		req.SetPageToken(resp.GetNextPageToken())
	}

	sort.Slice(devs, func(i, j int) bool {
		d1 := devs[i]
		d2 := devs[j]

		if d1.GetAssetTag() == d2.GetAssetTag() {
			return d1.GetOsType() < d2.GetOsType()
		}

		return d1.GetAssetTag() < d2.GetAssetTag()
	})

	return &deviceCollection{devices: devs}, nil
}

func createDevice(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	res, err := services.UnmarshalDevice(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		_, err = client.DevicesClient().UpsertDevice(ctx, devicepb.UpsertDeviceRequest_builder{
			Device:           dev,
			CreateAsResource: true,
		}.Build())
		// err checked below
	} else {
		_, err = client.DevicesClient().CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device:           dev,
			CreateAsResource: true,
		}.Build())
		// err checked below
	}
	if err != nil {
		return trail.FromGRPC(err)
	}

	verb := "created"
	if opts.Force {
		verb = "updated"
	}

	fmt.Printf("Device %v/%v %v\n",
		dev.GetAssetTag(),
		devicetrust.FriendlyOSType(dev.GetOsType()),
		verb,
	)
	return nil
}

func deleteDevice(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	remote := client.DevicesClient()
	device, err := findDeviceByIDOrTag(ctx, remote, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := remote.DeleteDevice(ctx, devicepb.DeleteDeviceRequest_builder{
		DeviceId: device[0].GetId(),
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Device %q removed\n", ref.Name)
	return nil
}

func findDeviceByIDOrTag(ctx context.Context, remote devicepb.DeviceTrustServiceClient, idOrTag string) ([]*devicepb.Device, error) {
	resp, err := remote.FindDevices(ctx, devicepb.FindDevicesRequest_builder{
		IdOrTag: idOrTag,
	}.Build())
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case len(resp.GetDevices()) == 0:
		return nil, trace.NotFound("device %q not found", idOrTag)
	case len(resp.GetDevices()) == 1:
		return resp.GetDevices(), nil
	}

	// Do we have an ID match?
	for _, dev := range resp.GetDevices() {
		if dev.GetId() == idOrTag {
			return []*devicepb.Device{dev}, nil
		}
	}

	return nil, trace.BadParameter("found multiple devices for asset tag %q, please retry using the device ID instead", idOrTag)
}
