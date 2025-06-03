package resource

import (
	"context"
	"fmt"
	"sort"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createDevice(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	res, err := services.UnmarshalDevice(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.IsForced() {
		_, err = client.DevicesClient().UpsertDevice(ctx, &devicepb.UpsertDeviceRequest{
			Device:           dev,
			CreateAsResource: true,
		})
		// err checked below
	} else {
		_, err = client.DevicesClient().CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device:           dev,
			CreateAsResource: true,
		})
		// err checked below
	}
	if err != nil {
		return trail.FromGRPC(err)
	}

	verb := "created"
	if rc.IsForced() {
		verb = "updated"
	}

	fmt.Printf("Device %v/%v %v\n",
		dev.AssetTag,
		devicetrust.FriendlyOSType(dev.OsType),
		verb,
	)
	return nil
}

func (rc *ResourceCommand) getDevice(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	remote := client.DevicesClient()
	if rc.ref.Name != "" {
		resp, err := remote.FindDevices(ctx, &devicepb.FindDevicesRequest{
			IdOrTag: rc.ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return collections.NewDeviceCollection(resp.Devices), nil
	}

	req := &devicepb.ListDevicesRequest{
		View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
	}
	var devs []*devicepb.Device
	for {
		resp, err := remote.ListDevices(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		devs = append(devs, resp.Devices...)

		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	sort.Slice(devs, func(i, j int) bool {
		d1 := devs[i]
		d2 := devs[j]

		if d1.AssetTag == d2.AssetTag {
			return d1.OsType < d2.OsType
		}

		return d1.AssetTag < d2.AssetTag
	})

	return collections.NewDeviceCollection(devs), nil
}
