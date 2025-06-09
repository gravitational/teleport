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

var device = resource{
	getHandler:    getDevice,
	createHandler: createDevice,
	deleteHandler: deleteDevice,
}

func createDevice(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	res, err := services.UnmarshalDevice(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.force {
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
	if opts.force {
		verb = "updated"
	}

	fmt.Printf("Device %v/%v %v\n",
		dev.AssetTag,
		devicetrust.FriendlyOSType(dev.OsType),
		verb,
	)
	return nil
}

func getDevice(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	remote := client.DevicesClient()
	if ref.Name != "" {
		resp, err := remote.FindDevices(ctx, &devicepb.FindDevicesRequest{
			IdOrTag: ref.Name,
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

func deleteDevice(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	remote := client.DevicesClient()
	device, err := findDeviceByIDOrTag(ctx, remote, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := remote.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
		DeviceId: device[0].Id,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Device %q removed\n", ref.Name)
	return nil
}

func findDeviceByIDOrTag(ctx context.Context, remote devicepb.DeviceTrustServiceClient, idOrTag string) ([]*devicepb.Device, error) {
	resp, err := remote.FindDevices(ctx, &devicepb.FindDevicesRequest{
		IdOrTag: idOrTag,
	})
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case len(resp.Devices) == 0:
		return nil, trace.NotFound("device %q not found", idOrTag)
	case len(resp.Devices) == 1:
		return resp.Devices, nil
	}

	// Do we have an ID match?
	for _, dev := range resp.Devices {
		if dev.Id == idOrTag {
			return []*devicepb.Device{dev}, nil
		}
	}

	return nil, trace.BadParameter("found multiple devices for asset tag %q, please retry using the device ID instead", idOrTag)
}
