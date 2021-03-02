/*
Copyright 2021 Gravitational, Inc.

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

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils/prompt"
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
}

func newMFALSCommand(parent *kingpin.CmdClause) *mfaLSCommand {
	c := &mfaLSCommand{
		CmdClause: parent.Command("ls", "Get a list of registered MFA devices").Hidden(),
	}
	c.Flag("verbose", "Print more information about MFA devices").Short('v').BoolVar(&c.verbose)
	return c
}

func (c *mfaLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	var devs []*types.MFADevice
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToCurrentCluster(cf.Context, false)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		resp, err := aci.GetMFADevices(cf.Context, &proto.GetMFADevicesRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		devs = resp.Devices
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	printMFADevices(devs, c.verbose)
	return nil
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
}

func newMFAAddCommand(parent *kingpin.CmdClause) *mfaAddCommand {
	c := &mfaAddCommand{
		CmdClause: parent.Command("add", "Add a new MFA device").Hidden(),
	}
	c.Flag("name", "Name of the new MFA device").StringVar(&c.devName)
	c.Flag("type", "Type of the new MFA device (TOTP or U2F)").StringVar(&c.devType)
	return c
}

func (c *mfaAddCommand) run(cf *CLIConf) error {
	if c.devType == "" {
		var err error
		c.devType, err = prompt.PickOne(os.Stdout, os.Stdin, "Choose device type", []string{"TOTP", "U2F"})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	var typ proto.AddMFADeviceRequestInit_DeviceType
	switch strings.ToUpper(c.devType) {
	case "TOTP":
		typ = proto.AddMFADeviceRequestInit_TOTP
	case "U2F":
		typ = proto.AddMFADeviceRequestInit_U2F
	default:
		return trace.BadParameter("unknown device type %q, must be either TOTP or U2F", c.devType)
	}

	if c.devName == "" {
		var err error
		c.devName, err = prompt.Input(os.Stdout, os.Stdin, "Enter device name")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	c.devName = strings.TrimSpace(c.devName)
	if c.devName == "" {
		return trace.BadParameter("device name can not be empty")
	}

	dev, err := c.addDeviceRPC(cf, c.devName, typ)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q added.\n", dev.Metadata.Name)
	return nil
}

func (c *mfaAddCommand) addDeviceRPC(cf *CLIConf, devName string, devType proto.AddMFADeviceRequestInit_DeviceType) (*types.MFADevice, error) {
	tc, err := makeClient(cf, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var dev *types.MFADevice
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToCurrentCluster(cf.Context, false)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		// TODO(awly): mfa: move this logic somewhere under /lib/auth/, closer
		// to the server logic. The CLI layer should ideally be thin.
		stream, err := aci.AddMFADevice(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		// Init.
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_Init{
			Init: &proto.AddMFADeviceRequestInit{
				DeviceName: devName,
				Type:       devType,
			},
		}}); err != nil {
			return trace.Wrap(err)
		}

		// Auth challenge using existing device.
		resp, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		authChallenge := resp.GetExistingMFAChallenge()
		if authChallenge == nil {
			return trace.BadParameter("server bug: server sent %T when client expected AddMFADeviceResponse_ExistingMFAChallenge", resp.Response)
		}
		authResp, err := client.PromptMFAChallenge(cf.Context, tc.Config.WebProxyAddr, authChallenge, "*registered* ")
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_ExistingMFAResponse{
			ExistingMFAResponse: authResp,
		}}); err != nil {
			return trace.Wrap(err)
		}

		// Registration challenge for new device.
		resp, err = stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		regChallenge := resp.GetNewMFARegisterChallenge()
		if regChallenge == nil {
			return trace.BadParameter("server bug: server sent %T when client expected AddMFADeviceResponse_NewMFARegisterChallenge", resp.Response)
		}
		regResp, err := promptRegisterChallenge(cf.Context, tc.Config.WebProxyAddr, regChallenge)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{
			NewMFARegisterResponse: regResp,
		}}); err != nil {
			return trace.Wrap(err)
		}

		// Receive registered device ack.
		resp, err = stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		ack := resp.GetAck()
		if ack == nil {
			return trace.BadParameter("server bug: server sent %T when client expected AddMFADeviceResponse_Ack", resp.Response)
		}
		dev = ack.Device
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, nil
}

func promptRegisterChallenge(ctx context.Context, proxyAddr string, c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, error) {
	switch c.Request.(type) {
	case *proto.MFARegisterChallenge_TOTP:
		return promptTOTPRegisterChallenge(c.GetTOTP())
	case *proto.MFARegisterChallenge_U2F:
		return promptU2FRegisterChallenge(ctx, proxyAddr, c.GetU2F())
	default:
		return nil, trace.BadParameter("server bug: server sent %T when client expected either MFARegisterChallenge_TOTP or MFARegisterChallenge_U2F", c.Request)
	}
}

func promptTOTPRegisterChallenge(c *proto.TOTPRegisterChallenge) (*proto.MFARegisterResponse, error) {
	// TODO(awly): mfa: use OS-specific image viewer to show a QR code.
	// TODO(awly): mfa: print OTP URL
	fmt.Println("Open your TOTP app and create a new manual entry with these fields:")
	fmt.Printf("Name: %s\n", c.Account)
	fmt.Printf("Issuer: %s\n", c.Issuer)
	fmt.Printf("Algorithm: %s\n", c.Algorithm)
	fmt.Printf("Number of digits: %d\n", c.Digits)
	fmt.Printf("Period: %ds\n", c.PeriodSeconds)
	fmt.Printf("Secret: %s\n", c.Secret)
	fmt.Println()

	totpCode, err := prompt.Input(os.Stdout, os.Stdin, "Once created, enter an OTP code generated by the app")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
		TOTP: &proto.TOTPRegisterResponse{Code: totpCode},
	}}, nil
}

func promptU2FRegisterChallenge(ctx context.Context, proxyAddr string, c *proto.U2FRegisterChallenge) (*proto.MFARegisterResponse, error) {
	fmt.Println("Tap your *new* security key")

	facet := proxyAddr
	if !strings.HasPrefix(proxyAddr, "https://") {
		facet = "https://" + facet
	}
	resp, err := u2f.RegisterSignChallenge(ctx, u2f.RegisterChallenge{
		Challenge: c.Challenge,
		AppID:     c.AppID,
	}, facet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
		RegistrationData: resp.RegistrationData,
		ClientData:       resp.ClientData,
	}}}, nil
}

type mfaRemoveCommand struct {
	*kingpin.CmdClause
	name string
}

func newMFARemoveCommand(parent *kingpin.CmdClause) *mfaRemoveCommand {
	c := &mfaRemoveCommand{
		CmdClause: parent.Command("rm", "Remove a MFA device").Hidden(),
	}
	c.Arg("name", "Name or ID of the MFA device to remove").Required().StringVar(&c.name)
	return c
}

func (c *mfaRemoveCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToCurrentCluster(cf.Context, false)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		stream, err := aci.DeleteMFADevice(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		// Init.
		if err := stream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_Init{
			Init: &proto.DeleteMFADeviceRequestInit{
				DeviceName: c.name,
			},
		}}); err != nil {
			return trace.Wrap(err)
		}

		// Auth challenge.
		resp, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		authChallenge := resp.GetMFAChallenge()
		if authChallenge == nil {
			return trace.BadParameter("server bug: server sent %T when client expected DeleteMFADeviceResponse_MFAChallenge", resp.Response)
		}
		authResp, err := client.PromptMFAChallenge(cf.Context, tc.Config.WebProxyAddr, authChallenge, "")
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_MFAResponse{
			MFAResponse: authResp,
		}}); err != nil {
			return trace.Wrap(err)
		}

		// Receive deletion ack.
		resp, err = stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
		ack := resp.GetAck()
		if ack == nil {
			return trace.BadParameter("server bug: server sent %T when client expected DeleteMFADeviceResponse_Ack", resp.Response)
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q removed.\n", c.name)
	return nil
}
