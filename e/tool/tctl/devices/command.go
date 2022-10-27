package devices

import (
	"context"
	"errors"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/auth"
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
	if _, ok := osTypeToEnum[c.os]; !ok {
		return trace.BadParameter("invalid --os: %v", c.os)
	}

	return errors.New("not implemented")
}

type lsCommand struct{}

func (c *lsCommand) Run(ctx context.Context, authClient auth.ClientI) error {
	return errors.New("not implemented")
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

	return errors.New("not implemented")
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

	return errors.New("not implemented")
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
