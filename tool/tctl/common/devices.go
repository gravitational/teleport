/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/devicetrust"
	dtnative "github.com/gravitational/teleport/lib/devicetrust/native"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// DevicesCommand implements the `tctl devices` command.
// Device trust is an enterprise-only feature, so this
// command will fail when run against an OSS auth server.
type DevicesCommand struct {
	add    deviceAddCommand
	ls     deviceListCommand
	rm     deviceRemoveCommand
	enroll deviceEnrollCommand
	lock   deviceLockCommand
}

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

func (c *DevicesCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	devicesCmd := app.Command("devices", "Register and manage trusted devices").Hidden()

	addCmd := devicesCmd.Command("add", "Register managed devices.")
	addCmd.Flag("os", "Operating system").
		EnumVar(&c.add.os, osTypes...)
	addCmd.Flag("asset-tag", "Inventory identifier for the device (e.g., Mac serial number)").
		StringVar(&c.add.assetTag)
	addCmd.Flag("current-device", "Registers the current device. Overrides --os and --asset-tag.").
		BoolVar(&c.add.currentDevice)
	addCmd.Flag("enroll", "If set, creates a device enrollment token").
		BoolVar(&c.add.enroll)
	addCmd.Flag("enroll-ttl", "Time duration for the enrollment token").
		DurationVar(&c.add.enrollTTL)

	_ = devicesCmd.Command("ls", "Lists managed devices.")

	rmCmd := devicesCmd.Command("rm", "Removes a managed device.")
	rmCmd.Flag("device-id", "Device identifier").StringVar(&c.rm.deviceID)
	rmCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.rm.assetTag)
	rmCmd.Flag("current-device", "Removes the current device. Overrides --device-id and --asset-tag.").
		BoolVar(&c.rm.currentDevice)

	enrollCmd := devicesCmd.Command("enroll", "Creates a new device enrollment token.")
	enrollCmd.Flag("device-id", "Device identifier").StringVar(&c.enroll.deviceID)
	enrollCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.enroll.assetTag)
	enrollCmd.Flag("current-device", "Enrolls the current device. Overrides --device-id and --asset-tag.").
		BoolVar(&c.enroll.currentDevice)
	enrollCmd.Flag("ttl", "Time duration for the enrollment token").DurationVar(&c.enroll.ttl)

	lockCmd := devicesCmd.Command("lock", "Locks a device.")
	lockCmd.Flag("device-id", "Device identifier").StringVar(&c.lock.deviceID)
	lockCmd.Flag("asset-tag", "Inventory identifier for the device").StringVar(&c.lock.assetTag)
	lockCmd.Flag("current-device", "Locks the current device. Overrides --device-id and --asset-tag.").
		BoolVar(&c.lock.currentDevice)
	lockCmd.Flag("message", "Message to display to locked-out users").StringVar(&c.lock.message)
	lockCmd.Flag("expires", "Time point (RFC3339) when the lock expires").StringVar(&c.lock.expires)
	lockCmd.Flag("ttl", "Time duration after which the lock expires").DurationVar(&c.lock.ttl)
}

// runner is used as a simple interface for subcommands.
type runner interface {
	Run(context.Context, *authclient.Client) error
}

func (c *DevicesCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	innerCmd, ok := map[string]runner{
		"devices add":    &c.add,
		"devices ls":     &c.ls,
		"devices rm":     &c.rm,
		"devices enroll": &c.enroll,
		"devices lock":   &c.lock,
	}[cmd]
	if !ok {
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	switch err := trail.FromGRPC(innerCmd.Run(ctx, client)); {
	case trace.IsNotImplemented(err):
		return true, trace.AccessDenied("Device Trust requires a Teleport Enterprise Auth Server running v12 or later.")
	default:
		return true, trace.Wrap(err)
	}
}

type deviceAddCommand struct {
	canOperateOnCurrentDevice

	os        string // string from command line, distinct from inherited osType!
	enroll    bool
	enrollTTL time.Duration
}

func (c *deviceAddCommand) Run(ctx context.Context, authClient *authclient.Client) error {
	if _, err := c.setCurrentDevice(); err != nil {
		return trace.Wrap(err)
	}

	// Mimic our required flag errors.
	if !c.currentDevice {
		switch {
		case c.os == "" && c.assetTag == "":
			return trace.BadParameter("required flags [--os --asset-tag] not provided")
		case c.os == "":
			return trace.BadParameter("required flag --os not provided")
		case c.assetTag == "":
			return trace.BadParameter("required flag --asset-tag not provided")
		}
	}

	if c.os != "" {
		var ok bool
		c.osType, ok = osTypeToEnum[c.os]
		if !ok {
			return trace.BadParameter("invalid --os: %v", c.os)
		}
	}

	var enrollExpireTime *timestamppb.Timestamp
	if c.enrollTTL > 0 {
		enrollExpireTime = timestamppb.New(time.Now().Add(c.enrollTTL))
	}
	created, err := authClient.DevicesClient().CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   c.osType,
			AssetTag: c.assetTag,
		},
		CreateEnrollToken:     c.enroll,
		EnrollTokenExpireTime: enrollExpireTime,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"Device %v/%v added to the inventory\n",
		created.AssetTag,
		devicetrust.FriendlyOSType(created.OsType))
	printEnrollMessage(created.AssetTag, created.EnrollToken)

	return nil
}

func printEnrollMessage(name string, token *devicepb.DeviceEnrollToken) {
	if token.GetToken() == "" {
		return
	}
	expireTime := token.ExpireTime.AsTime()

	fmt.Printf(`The enrollment token: %v
This token will expire in %v.

Run the command below on device %q to enroll it:
tsh device enroll --token=%v
`,
		token.Token, time.Until(expireTime).Round(time.Second), name, token.Token,
	)
}

type deviceListCommand struct{}

func (c *deviceListCommand) Run(ctx context.Context, authClient *authclient.Client) error {
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

type deviceRemoveCommand struct {
	canOperateOnCurrentDevice

	deviceID string
}

func (c *deviceRemoveCommand) Run(ctx context.Context, authClient *authclient.Client) error {
	switch ok, err := c.setCurrentDevice(); {
	case err != nil:
		return trace.Wrap(err)
	case ok:
		c.deviceID = ""
	}

	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	}

	devices := authClient.DevicesClient()

	// Find the specified device, if necessary.
	deviceID, name, err := findDeviceID(ctx, devices, c.deviceID, c.assetTag, c.osType)
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

type deviceEnrollCommand struct {
	canOperateOnCurrentDevice

	deviceID string
	ttl      time.Duration
}

func (c *deviceEnrollCommand) Run(ctx context.Context, authClient *authclient.Client) error {
	switch ok, err := c.setCurrentDevice(); {
	case err != nil:
		return trace.Wrap(err)
	case ok:
		c.deviceID = ""
	}

	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	}

	devices := authClient.DevicesClient()

	// Find the specified device, if necessary.
	deviceID, name, err := findDeviceID(ctx, devices, c.deviceID, c.assetTag, c.osType)
	if err != nil {
		return trace.Wrap(err)
	}

	var expireTime *timestamppb.Timestamp
	if c.ttl > 0 {
		expireTime = timestamppb.New(time.Now().Add(c.ttl))
	}
	token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
		DeviceId:   deviceID,
		ExpireTime: expireTime,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	printEnrollMessage(name, token)
	return nil
}

type deviceLockCommand struct {
	canOperateOnCurrentDevice

	deviceID string
	message  string
	expires  string
	ttl      time.Duration
}

func (c *deviceLockCommand) Run(ctx context.Context, authClient *authclient.Client) error {
	switch ok, err := c.setCurrentDevice(); {
	case err != nil:
		return trace.Wrap(err)
	case ok:
		c.deviceID = ""
		// Print here, otherwise device information isn't apparent.
		// In other command modes the user just wrote the ID or asset tag in the
		// command line.
		fmt.Printf("Locking device %q.\n", c.assetTag)
	}

	switch {
	case c.deviceID == "" && c.assetTag == "":
		return trace.BadParameter("either --device-id or --asset-tag must be set")
	case c.deviceID != "" && c.assetTag != "":
		return trace.BadParameter("only one of --device-id or --asset-tag must be set")
	case c.expires != "" && c.ttl != 0:
		return trace.BadParameter("use only one of --expires and --ttl")
	}

	var expires *time.Time
	switch {
	case c.expires != "":
		t, err := time.Parse(time.RFC3339, c.expires)
		if err != nil {
			return trace.Wrap(err)
		}
		expires = &t
	case c.ttl != 0:
		t := time.Now().UTC().Add(c.ttl)
		expires = &t
	}

	deviceID, _, err := findDeviceID(ctx, authClient.DevicesClient(), c.deviceID, c.assetTag, c.osType)
	if err != nil {
		return trace.Wrap(err)
	}

	lock, err := types.NewLock(uuid.NewString(), types.LockSpecV2{
		Target: types.LockTarget{
			Device: deviceID,
		},
		Message: c.message,
		Expires: expires,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authClient.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Created a lock with name %q.\n", lock.GetName())
	return nil
}

// findDeviceID finds the device ID when supplied with either a deviceID or
// assetTag. If supplied with the former, no backend queries are made. It exists
// to simplify the logic of commands that take either --device-id or --asset-tag
// as an argument.
//
// The optional osType parameter is used to distinguish devices registered in
// multiple platforms.
//
// Returns the device ID and a name that can be used for CLI messages, the
// latter matching whatever was originally supplied - the device ID or the asset
// tag.
func findDeviceID(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient,
	deviceID, assetTag string, osType devicepb.OSType,
) (id, name string, err error) {
	if deviceID != "" {
		// No need to query.
		return deviceID, deviceID, nil
	}

	resp, err := devices.FindDevices(ctx, &devicepb.FindDevicesRequest{
		IdOrTag: assetTag,
	})
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	for _, found := range resp.Devices {
		// Skip ID matches and unexpected osTypes.
		if found.AssetTag != assetTag || (osType != devicepb.OSType_OS_TYPE_UNSPECIFIED && found.OsType != osType) {
			continue
		}

		// Sanity check.
		if deviceID != "" {
			return "", "", trace.BadParameter(
				"found multiple devices for asset tag %q, please retry using the device ID instead", assetTag)
		}

		deviceID = found.Id
	}
	if deviceID == "" {
		return "", "", trace.NotFound("device %q not found", assetTag)
	}

	return deviceID, assetTag, nil
}

// canOperateOnCurrentDevice marks commands capable of operating against the
// current device.
type canOperateOnCurrentDevice struct {
	osType   devicepb.OSType
	assetTag string

	// currentDevice means osType and assetTag are set according to the current
	// device.
	currentDevice bool
}

func (c *canOperateOnCurrentDevice) setCurrentDevice() (bool, error) {
	if !c.currentDevice {
		return false, nil
	}

	cdd, err := dtnative.CollectDeviceData(dtnative.CollectedDataMaybeEscalate)
	if err != nil {
		return false, trace.Wrap(err)
	}

	c.osType = cdd.OsType
	c.assetTag = cdd.SerialNumber
	slog.DebugContext(
		context.Background(),
		"Running device command against current device",
		"asset_tag", c.assetTag,
		"os_type", devicetrust.FriendlyOSType(c.osType),
	)
	return true, nil
}
