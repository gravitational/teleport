package devices

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/service"
)

type osType = string

const (
	linuxType   osType = "linux"
	macosType   osType = "macos"
	windowsType osType = "windows"
)

var osTypes = []string{linuxType, macosType, windowsType}

var osTypeToEnum = map[osType]devicepb.OSType{
	linuxType:   devicepb.OSType_OS_TYPE_LINUX,
	macosType:   devicepb.OSType_OS_TYPE_MACOS,
	windowsType: devicepb.OSType_OS_TYPE_WINDOWS,
}

// Command implements the `tctl devices` command.
type Command struct {
	add    addCommand
	ls     lsCommand
	rm     rmCommand
	enroll enrollCommand
	lock   lockCommand
}

func (c *Command) Initialize(app *kingpin.Application, cfg *service.Config) {
	devicesCmd := app.Command("devices", "Register and manage trusted devices").Hidden()

	addCmd := devicesCmd.Command("add", "Register managed devices").Hidden()
	addCmd.Flag("os", "Operating system").
		Required().
		EnumVar(&c.add.os, osTypes...)
	addCmd.Flag("asset-tag", "Inventory identifier for the device (e.g., Mac serial number)").
		Required().
		StringVar(&c.add.assetTag)
	addCmd.Flag("enroll", "If set, creates a device enrollment token").
		BoolVar(&c.add.enroll)

	_ = devicesCmd.Command("ls", "Lists managed devices").Hidden()

	rmCmd := devicesCmd.Command("rm", "Removes a managed device").Hidden()
	rmCmd.Flag("device-id", "Device identifier").StringVar(&c.rm.deviceID)
	rmCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.rm.assetTag)

	enrollCmd := devicesCmd.Command("enroll", "Creates a new device enrollment token").Hidden()
	enrollCmd.Flag("device-id", "Device identifier").StringVar(&c.enroll.deviceID)
	enrollCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.enroll.assetTag)

	lockCmd := devicesCmd.Command("lock", "Locks a device").Hidden()
	lockCmd.Flag("device-id", "Device identifier").StringVar(&c.lock.deviceID)
	lockCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.lock.assetTag)
}

// runner is used as a simple interface for subcommands.
type runner interface {
	Run(context.Context, auth.ClientI) error
}

func (c *Command) TryRun(ctx context.Context, selectedCommand string, authClient auth.ClientI) (match bool, err error) {
	if innerCmd, ok := map[string]runner{
		"devices add":    &c.add,
		"devices ls":     &c.ls,
		"devices rm":     &c.rm,
		"devices enroll": &c.enroll,
		"devices lock":   &c.lock,
	}[selectedCommand]; ok {
		return true, innerCmd.Run(ctx, authClient)
	}
	return false, nil
}

type addCommand struct {
	os       string
	assetTag string
	enroll   bool
}

func (c *addCommand) Run(ctx context.Context, authClient auth.ClientI) error {
	osType, ok := osTypeToEnum[c.os]
	if !ok {
		return trace.BadParameter("invalid --os: %v", c.os)
	}

	created, err := authClient.DevicesClient().CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   osType,
			AssetTag: c.assetTag,
		},
		CreateEnrollToken: c.enroll,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"Device %v/%v added to the inventory\n",
		created.AssetTag,
		devicetrust.FriendlyOSType(created.OsType))
	if token := created.EnrollToken.GetToken(); token != "" {
		printEnrollMessage(created.AssetTag, token)
	}

	return nil
}

func printEnrollMessage(name, token string) {
	fmt.Printf(""+
		"Run the command below on device %q to enroll it:\n"+
		"tsh device enroll --token=%v\n",
		name, token,
	)
}

type lsCommand struct{}

func (c *lsCommand) Run(ctx context.Context, authClient auth.ClientI) error {
	devices := authClient.DevicesClient()

	// List all devices.
	req := &devicepb.ListDevicesRequest{
		View: devicepb.DeviceView_DEVICE_VIEW_LIST,
	}
	var devs []*devicepb.Device
	for {
		resp, err := devices.ListDevices(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		devs = append(devs, resp.Devices...)

		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}
	if len(devs) == 0 {
		fmt.Println("No devices found")
		return nil
	}

	// Sort by {AssetTag, OsType}.
	sort.Slice(devs, func(i, j int) bool {
		d1 := devs[i]
		d2 := devs[j]

		if d1.AssetTag == d2.AssetTag {
			return d1.OsType < d2.OsType
		}

		return d1.AssetTag < d2.AssetTag
	})

	// Print devices.
	table := asciitable.MakeTable([]string{"Asset Tag", "OS", "Enroll Status", "Device ID"})
	for _, dev := range devs {
		table.AddRow([]string{
			dev.AssetTag,
			devicetrust.FriendlyOSType(dev.OsType),
			devicetrust.FriendlyDeviceEnrollStatus(dev.EnrollStatus),
			dev.Id,
		})
	}
	fmt.Println(table.AsBuffer().String())

	return nil
}

type rmCommand struct {
	deviceID, assetTag string
}

func (c *rmCommand) Run(ctx context.Context, authClient auth.ClientI) error {
	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	}

	devices := authClient.DevicesClient()

	// Find the specified device, if necessary.
	deviceID, name, err := findDeviceID(ctx, devices, c.deviceID, c.assetTag)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := devices.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
		DeviceId: deviceID,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Device %q removed\n", name)
	return nil
}

type enrollCommand struct {
	deviceID, assetTag string
}

func (c *enrollCommand) Run(ctx context.Context, authClient auth.ClientI) error {
	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	}

	devices := authClient.DevicesClient()

	// Find the specified device, if necessary.
	deviceID, name, err := findDeviceID(ctx, devices, c.deviceID, c.assetTag)
	if err != nil {
		return trace.Wrap(err)
	}

	token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
		DeviceId: deviceID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	printEnrollMessage(name, token.Token)
	return nil
}

type lockCommand struct {
	deviceID, assetTag string
}

func (c *lockCommand) Run(context.Context, auth.ClientI) error {
	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	}

	return errors.New("not implemented")
}

func findDeviceID(ctx context.Context, devices devicepb.DeviceTrustServiceClient, deviceID, assetTag string) (id, name string, err error) {
	if deviceID != "" {
		// No need to query.
		return deviceID, deviceID, nil
	}

	resp, err := devices.FindDevices(ctx, &devicepb.FindDevicesRequest{
		IdOrTag: assetTag,
	})
	switch l := len(resp.Devices); {
	case err != nil:
		return "", "", trace.Wrap(err)
	case l == 0:
		return "", "", trace.NotFound("device %q not found", assetTag)
	case l > 1:
		return "", "", trace.BadParameter(
			"found multiple devices for asset tag %q, please retry using the device ID instead", assetTag)
	default:
		return resp.Devices[0].Id, assetTag, nil
	}
}
