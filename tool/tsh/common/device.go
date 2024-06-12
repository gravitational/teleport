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
	"encoding/json"
	"fmt"
	"io"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
	dtnative "github.com/gravitational/teleport/lib/devicetrust/native"
	"github.com/gravitational/teleport/lib/linux"
)

type deviceCommand struct {
	enroll *deviceEnrollCommand

	// collect, assetTag and keyget are debug commands.
	collect  *deviceCollectCommand
	assetTag *deviceAssetTagCommand
	keyget   *deviceKeygetCommand

	// activateCredential is a hidden command invoked on an elevated child
	// process
	activateCredential *deviceActivateCredentialCommand

	// dmiRead is a hidden command invoked on an elevated child to read the
	// device's DMI information.
	dmiRead *deviceDMIReadCommand
}

func newDeviceCommand(app *kingpin.Application) *deviceCommand {
	root := &deviceCommand{
		enroll:             &deviceEnrollCommand{},
		collect:            &deviceCollectCommand{},
		assetTag:           &deviceAssetTagCommand{},
		keyget:             &deviceKeygetCommand{},
		activateCredential: &deviceActivateCredentialCommand{},
		dmiRead:            &deviceDMIReadCommand{},
	}

	// "tsh device" command.
	parentCmd := app.Command(
		"device", "Manage this device. Requires Teleport Enterprise.")

	// "tsh device enroll" command.
	root.enroll.CmdClause = parentCmd.Command(
		"enroll", "Enroll this device as a trusted device. Requires Teleport Enterprise.")
	root.enroll.Flag(
		"current-device",
		"Attempts to register and enroll the current device. Requires device admin privileges.").
		BoolVar(&root.enroll.currentDevice)
	root.enroll.Flag("token", "Device enrollment token").StringVar(&root.enroll.token)

	// "tsh device" hidden debug commands.
	root.collect.CmdClause = parentCmd.Command("collect", "Simulate enroll/authn device data collection").Hidden()
	root.assetTag.CmdClause = parentCmd.Command("asset-tag", "Print the detected device asset tag").Hidden()
	root.keyget.CmdClause = parentCmd.Command("keyget", "Get information about the device key").Hidden()

	// Windows TPM hidden support commands.
	root.activateCredential.CmdClause = parentCmd.Command("tpm-activate-credential", "").Hidden()
	root.activateCredential.Flag("encrypted-credential", "").
		Required().
		StringVar(&root.activateCredential.encryptedCredential)
	root.activateCredential.Flag("encrypted-credential-secret", "").
		Required().
		StringVar(&root.activateCredential.encryptedCredentialSecret)

	// Linux TPM hidden support commands.
	root.dmiRead.CmdClause = parentCmd.Command("dmi-read", "Read device DMI information").Hidden()

	return root
}

type deviceEnrollCommand struct {
	*kingpin.CmdClause

	currentDevice bool
	token         string
}

func (c *deviceEnrollCommand) run(cf *CLIConf) error {
	if c.token == "" && !c.currentDevice {
		// Mimic our required flag error.
		// We don't want to suggest --current-device casually.
		return trace.BadParameter("required flag --token not provided")
	}

	teleportClient, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := cf.Context
	return trace.Wrap(client.RetryWithRelogin(ctx, teleportClient, func() error {
		proxyClient, err := teleportClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err := proxyClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()
		devices := authClient.DevicesClient()
		enrollCeremony := enroll.NewCeremony()

		// Admin fast-tracked enrollment.
		if c.currentDevice {
			dev, outcome, err := enrollCeremony.RunAdmin(ctx, devices, cf.Debug)
			printEnrollOutcome(cf.Stdout(), outcome, dev) // Report partial successes.
			return trace.Wrap(err)
		}

		// End-user enrollment.
		dev, err := enrollCeremony.Run(ctx, devices, cf.Debug, c.token)
		if err == nil {
			printEnrollOutcome(cf.Stdout(), enroll.DeviceEnrolled, dev)
		}
		return trace.Wrap(err)
	}))
}

func printEnrollOutcome(out io.Writer, outcome enroll.RunAdminOutcome, dev *devicepb.Device) {
	var action string
	switch outcome {
	case enroll.DeviceRegisteredAndEnrolled:
		action = "registered and enrolled"
	case enroll.DeviceRegistered:
		action = "registered"
	case enroll.DeviceEnrolled:
		action = "enrolled"
	default:
		return // All actions failed, don't print anything.
	}

	// This shouldn't happen, but let's play it safe and avoid a silly panic.
	if dev == nil {
		fmt.Fprintf(out, "Device %v\n", action)
		return
	}

	fmt.Fprintf(
		out,
		"Device %q/%v %v\n",
		dev.AssetTag, devicetrust.FriendlyOSType(dev.OsType), action)
}

type deviceCollectCommand struct {
	*kingpin.CmdClause
}

func (c *deviceCollectCommand) run(cf *CLIConf) error {
	cdd, err := dtnative.CollectDeviceData(dtnative.CollectedDataAlwaysEscalate)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := &protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: true,
	}
	val, err := opts.Marshal(cdd)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "DeviceCollectedData %s\n", val)
	return nil
}

type deviceAssetTagCommand struct {
	*kingpin.CmdClause
}

func (c *deviceAssetTagCommand) run(cf *CLIConf) error {
	cdd, err := dtnative.CollectDeviceData(dtnative.CollectedDataAlwaysEscalate)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(cf.Stdout(), cdd.SerialNumber)
	return nil
}

type deviceKeygetCommand struct {
	*kingpin.CmdClause
}

func (c *deviceKeygetCommand) run(cf *CLIConf) error {
	cred, err := dtnative.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err)
	}

	opts := &protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: true,
	}
	val, err := opts.Marshal(cred)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "DeviceCredential %s\n", val)
	return nil
}

type deviceActivateCredentialCommand struct {
	*kingpin.CmdClause
	encryptedCredential       string
	encryptedCredentialSecret string
}

func (c *deviceActivateCredentialCommand) run(cf *CLIConf) error {
	//nolint:staticcheck // HandleTPMActivateCredential works depending on the platform.
	err := dtnative.HandleTPMActivateCredential(
		c.encryptedCredential, c.encryptedCredentialSecret,
	)
	//nolint:staticcheck // `err` can indeed be nil.
	if cf.Debug && err != nil {
		// On error, wait for user input before executing. This is because this
		// opens in a second window. If we return the error immediately, then
		// this window closes before the user can inspect it.
		log.WithError(err).Error("An error occurred during credential activation. Press enter to close this window.")
		_, _ = fmt.Scanln()
	}
	return trace.Wrap(err)
}

type deviceDMIReadCommand struct {
	*kingpin.CmdClause
}

func (c *deviceDMIReadCommand) run(cf *CLIConf) error {
	dmiInfo, err := linux.DMIInfoFromSysfs()
	if err != nil {
		log.WithError(err).Warn("Device Trust: Failed to read DMI information")
		// err swallowed on purpose.
	}
	if dmiInfo != nil {
		_ = json.NewEncoder(cf.Stdout()).Encode(dmiInfo)
	}
	return nil
}
