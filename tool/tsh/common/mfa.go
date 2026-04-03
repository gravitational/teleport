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
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/slices"
)

type mfaCommands struct {
	ls  *mfaLSCommand
	add *mfaAddCommand
	rm  *mfaRemoveCommand
}

func newMFACommand(app *kingpin.Application) mfaCommands {
	mfa := app.Command("mfa", "Manage multi-factor authentication (MFA) devices.")
	return mfaCommands{
		ls:  newMFALSCommand(mfa),
		add: newMFAAddCommand(mfa),
		rm:  newMFARemoveCommand(mfa),
	}
}

type mfaLSCommand struct {
	*kingpin.CmdClause
	verbose bool
	format  string
}

func newMFALSCommand(parent *kingpin.CmdClause) *mfaLSCommand {
	c := &mfaLSCommand{
		CmdClause: parent.Command("ls", "Get a list of registered MFA devices."),
	}
	c.Flag("verbose", "Print more information about MFA devices.").Short('v').BoolVar(&c.verbose)
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)
	return c
}

func (c *mfaLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var devs []*types.MFADevice
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		rootAuthClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootAuthClient.Close()

		resp, err := rootAuthClient.GetMFADevices(cf.Context, &proto.GetMFADevicesRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		devs = resp.Devices
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	// Sort by name before printing.
	sort.Slice(devs, func(i, j int) bool { return devs[i].GetName() < devs[j].GetName() })

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		printMFADevices(devs, c.verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeMFADevices(devs, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}

	return nil
}

func serializeMFADevices(devs []*types.MFADevice, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(devs, "", "  ")
	} else {
		out, err = yaml.Marshal(devs)
	}
	return string(out), trace.Wrap(err)
}

func printMFADevices(devs []*types.MFADevice, verbose bool) {
	if verbose {
		t := asciitable.MakeTable([]string{"Name", "ID", "Type", "Added at", "Last used"})
		for _, dev := range devs {
			t.AddRow([]string{
				dev.Metadata.Name,
				dev.Id,
				dev.MFAType(),
				dev.AddedAt.Format(time.RFC1123),
				dev.LastUsed.Format(time.RFC1123),
			})
		}
		fmt.Println(t.AsBuffer().String())
	} else {
		t := asciitable.MakeTable([]string{"Name", "Type", "Added at", "Last used"})
		for _, dev := range devs {
			t.AddRow([]string{
				dev.GetName(),
				dev.MFAType(),
				dev.AddedAt.Format(time.RFC1123),
				dev.LastUsed.Format(time.RFC1123),
			})
		}
		fmt.Println(t.AsBuffer().String())
	}
}

type mfaAddCommand struct {
	*kingpin.CmdClause
	devName string
	devType string

	// allowPasswordless and allowPasswordlessSet hold the state of the
	// --(no-)allow-passwordless flag.
	//
	// allowPasswordless can only be set by users if wancli.IsFIDO2Available() is
	// true.
	// Note that Touch ID registrations are always passwordless-capable,
	// regardless of other settings.
	allowPasswordless, allowPasswordlessSet bool
}

func newMFAAddCommand(parent *kingpin.CmdClause) *mfaAddCommand {
	c := &mfaAddCommand{
		CmdClause: parent.Command("add", "Add a new MFA device."),
	}
	c.Flag("name", "Name of the new MFA device.").StringVar(&c.devName)
	c.Flag("type", fmt.Sprintf("Type of the new MFA device (%s).", apiutils.JoinStrings(libmfa.DefaultDeviceTypes, ", "))).
		EnumVar(&c.devType, slices.Map(libmfa.DefaultDeviceTypes, func(v mfa.MFADeviceType) string { return string(v) })...)
	if wancli.IsFIDO2Available() {
		c.Flag("allow-passwordless", "Allow passwordless logins.").
			IsSetByUser(&c.allowPasswordlessSet).
			BoolVar(&c.allowPasswordless)
	}
	return c
}

func (c *mfaAddCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx := cf.Context
	out := cf.Stdout()

	config := mfa.RegistrationCeremonyConfig{
		Confirmed:  true,
		DeviceName: c.devName,
		DeviceType: mfa.MFADeviceType(c.devType), // type correctness guaranteed by EnumVar
	}

	if c.allowPasswordlessSet {
		if c.allowPasswordless {
			config.DeviceUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
		} else {
			config.DeviceUsage = proto.DeviceUsage_DEVICE_USAGE_MFA
		}
	}

	added, err := tc.AddMFA(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	if added {
		fmt.Fprintf(out, "MFA device %q added.\n", config.DeviceName)
	}
	return nil
}

type mfaRemoveCommand struct {
	*kingpin.CmdClause
	name string
}

func newMFARemoveCommand(parent *kingpin.CmdClause) *mfaRemoveCommand {
	c := &mfaRemoveCommand{
		CmdClause: parent.Command("rm", "Remove a MFA device."),
	}
	c.Arg("name", "Name or ID of the MFA device to remove.").Required().StringVar(&c.name)
	return c
}

func (c *mfaRemoveCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := cf.Context
	if err := client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootAuthClient.Close()

		// Lookup device to delete.
		// This lets us exit early if the device doesn't exist and enables the
		// Touch ID cleanup at the end.
		devicesResp, err := rootAuthClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		var deviceToDelete *types.MFADevice
		for _, dev := range devicesResp.Devices {
			if dev.GetName() == c.name {
				deviceToDelete = dev
				break
			}
		}
		if deviceToDelete == nil {
			return trace.NotFound("device %q not found", c.name)
		}

		mfaResponse, err := tc.NewMFACeremony().Run(ctx, &proto.CreateAuthenticateChallengeRequest{
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Delete device.
		if err := rootAuthClient.DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
			DeviceName:          c.name,
			ExistingMFAResponse: mfaResponse,
		}); err != nil {
			return trace.Wrap(err)
		}

		// If deleted device was a webauthn device, then attempt to delete leftover
		// Touch ID credentials.
		if wanDevice := deviceToDelete.GetWebauthn(); wanDevice != nil {
			deleteTouchIDCredentialIfApplicable(string(wanDevice.CredentialId))
		}

		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q removed.\n", c.name)
	return nil
}

func deleteTouchIDCredentialIfApplicable(credentialID string) {
	switch err := touchid.AttemptDeleteNonInteractive(credentialID); {
	case errors.Is(err, &touchid.ErrAttemptFailed{}):
		// Nothing to do here, just proceed.
	case err != nil:
		logger.ErrorContext(context.Background(), "Failed to delete credential",
			"error", err,
			"credential", credentialID,
		)
	}
}
