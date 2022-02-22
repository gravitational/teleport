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
	"encoding/base32"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

const (
	totpDeviceType     = "TOTP"
	u2fDeviceType      = "U2F"
	webauthnDeviceType = "WEBAUTHN"
)

// defaultDeviceTypes lists the supported device types for `tsh mfa add`.
var defaultDeviceTypes = []string{totpDeviceType, u2fDeviceType, webauthnDeviceType}

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
		CmdClause: parent.Command("ls", "Get a list of registered MFA devices"),
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
		aci, err := pc.ConnectToRootCluster(cf.Context, false)
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

	// Sort by name before printing.
	sort.Slice(devs, func(i, j int) bool { return devs[i].GetName() < devs[j].GetName() })

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
		CmdClause: parent.Command("add", "Add a new MFA device"),
	}
	c.Flag("name", "Name of the new MFA device").StringVar(&c.devName)
	c.Flag("type", fmt.Sprintf("Type of the new MFA device (%s)", strings.Join(defaultDeviceTypes, ", "))).
		StringVar(&c.devType)
	return c
}

func (c *mfaAddCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx := cf.Context

	deviceTypes := defaultDeviceTypes
	if c.devType == "" {
		// If we are prompting the user for the device type, then take a glimpse at
		// server-side settings and adjust the options accordingly.
		// This is undesirable to do during flag setup, but we can do it here.
		pingResp, err := tc.Ping(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		deviceTypes = deviceTypesFromPreferredMFA(pingResp.Auth.PreferredLocalMFA)

		c.devType, err = prompt.PickOne(ctx, os.Stdout, prompt.Stdin(), "Choose device type", deviceTypes)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	m := map[string]proto.DeviceType{
		totpDeviceType:     proto.DeviceType_DEVICE_TYPE_TOTP,
		u2fDeviceType:      proto.DeviceType_DEVICE_TYPE_U2F,
		webauthnDeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}
	devType := m[c.devType]
	if devType == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return trace.BadParameter("unknown device type %q, must be one of %v", c.devType, strings.Join(deviceTypes, ", "))
	}

	if c.devName == "" {
		var err error
		c.devName, err = prompt.Input(ctx, os.Stdout, prompt.Stdin(), "Enter device name")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	c.devName = strings.TrimSpace(c.devName)
	if c.devName == "" {
		return trace.BadParameter("device name can not be empty")
	}

	dev, err := c.addDeviceRPC(ctx, tc, c.devName, devType)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q added.\n", dev.Metadata.Name)
	return nil
}

func deviceTypesFromPreferredMFA(preferredMFA constants.SecondFactorType) []string {
	log.Debugf("Got server-side preferred local MFA: %v", preferredMFA)

	m := map[constants.SecondFactorType]string{
		constants.SecondFactorOTP:      totpDeviceType,
		constants.SecondFactorU2F:      u2fDeviceType,
		constants.SecondFactorWebauthn: webauthnDeviceType,
	}

	// Use preferredMFA as a way to choose between Webauthn and U2F, so both don't
	// appear together in the interactive UI.
	// We won't attempt to deal with all nuances of second factor configuration
	// here, just make a sensible choice and let the backend deal with the rest.
	switch preferredType, ok := m[preferredMFA]; {
	case !ok: // Empty or unknown suggestion, fallback to defaults.
		return defaultDeviceTypes
	case preferredType == totpDeviceType: // OTP only
		return []string{preferredType}
	default: // OTP + Webauthn or U2F
		return []string{totpDeviceType, preferredType}
	}
}

func (c *mfaAddCommand) addDeviceRPC(ctx context.Context, tc *client.TeleportClient, devName string, devType proto.DeviceType) (*types.MFADevice, error) {
	var dev *types.MFADevice
	if err := client.RetryWithRelogin(ctx, tc, func() error {
		pc, err := tc.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(ctx, false)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		// TODO(awly): mfa: move this logic somewhere under /lib/auth/, closer
		// to the server logic. The CLI layer should ideally be thin.
		stream, err := aci.AddMFADevice(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		// Init.
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_Init{
			Init: &proto.AddMFADeviceRequestInit{
				DeviceName: devName,
				DeviceType: devType,
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
		authResp, err := client.PromptMFAChallenge(ctx, tc.Config.WebProxyAddr, authChallenge, "*registered* ", false)
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
		regResp, err := promptRegisterChallenge(ctx, tc.Config.WebProxyAddr, regChallenge)
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
		return promptTOTPRegisterChallenge(ctx, c.GetTOTP())
	case *proto.MFARegisterChallenge_U2F:
		return promptU2FRegisterChallenge(ctx, proxyAddr, c.GetU2F())
	case *proto.MFARegisterChallenge_Webauthn:
		return promptWebauthnRegisterChallenge(ctx, proxyAddr, c.GetWebauthn())
	default:
		return nil, trace.BadParameter("server bug: unexpected registration challenge type: %T", c.Request)
	}
}

func promptTOTPRegisterChallenge(ctx context.Context, c *proto.TOTPRegisterChallenge) (*proto.MFARegisterResponse, error) {
	secretBin, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(c.Secret)
	if err != nil {
		return nil, trace.BadParameter("server sent an invalid TOTP secret key %q: %v", c.Secret, err)
	}
	var algorithm otp.Algorithm
	switch strings.ToUpper(c.Algorithm) {
	case "SHA1":
		algorithm = otp.AlgorithmSHA1
	case "SHA256":
		algorithm = otp.AlgorithmSHA256
	case "SHA512":
		algorithm = otp.AlgorithmSHA512
	case "MD5":
		algorithm = otp.AlgorithmMD5
	default:
		return nil, trace.BadParameter("server sent an unknown TOTP algorithm %q", c.Algorithm)
	}
	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      c.Issuer,
		AccountName: c.Account,
		Period:      uint(c.PeriodSeconds),
		Secret:      secretBin,
		Digits:      otp.Digits(c.Digits),
		Algorithm:   algorithm,
	})
	if err != nil {
		return nil, trace.BadParameter("server sent invalid TOTP parameters: %v", err)
	}

	// Try to show a QR code in the system image viewer.
	// This is not supported on all platforms.
	var showingQRCode bool
	closeQR, err := showOTPQRCode(otpKey)
	if err != nil {
		log.WithError(err).Debug("Failed to show QR code")
	} else {
		showingQRCode = true
		defer closeQR()
	}

	fmt.Println()
	if showingQRCode {
		fmt.Println("Open your TOTP app and scan the QR code. Alternatively, you can manually enter these fields:")
	} else {
		fmt.Println("Open your TOTP app and create a new manual entry with these fields:")
	}
	fmt.Printf(`  URL: %s
  Account name: %s
  Secret key: %s
  Issuer: %s
  Algorithm: %s
  Number of digits: %d
  Period: %ds
`, otpKey.URL(), c.Account, c.Secret, c.Issuer, c.Algorithm, c.Digits, c.PeriodSeconds)
	fmt.Println()

	var totpCode string
	// Help the user with typos, don't submit the code until it has the right
	// length.
	for {
		totpCode, err = prompt.Input(ctx, os.Stdout, prompt.Stdin(), "Once created, enter an OTP code generated by the app")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(totpCode) == int(c.Digits) {
			break
		}
		fmt.Printf("TOTP code must be exactly %d digits long, try again\n", c.Digits)
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

func promptWebauthnRegisterChallenge(ctx context.Context, proxyAddr string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	origin := proxyAddr
	if !strings.HasPrefix(proxyAddr, "https://") {
		origin = "https://" + origin
	}
	log.Debugf("WebAuthn: prompting U2F devices with origin %q", origin)

	fmt.Println("Tap your *new* security key")

	resp, err := wancli.Register(ctx, origin, wanlib.CredentialCreationFromProto(cc))
	return resp, trace.Wrap(err)
}

type mfaRemoveCommand struct {
	*kingpin.CmdClause
	name string
}

func newMFARemoveCommand(parent *kingpin.CmdClause) *mfaRemoveCommand {
	c := &mfaRemoveCommand{
		CmdClause: parent.Command("rm", "Remove a MFA device"),
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
		aci, err := pc.ConnectToRootCluster(cf.Context, false)
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
		authResp, err := client.PromptMFAChallenge(cf.Context, tc.Config.WebProxyAddr, authChallenge, "", false)
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

func showOTPQRCode(k *otp.Key) (cleanup func(), retErr error) {
	var imageViewer string
	switch runtime.GOOS {
	case "linux":
		imageViewer = "xdg-open"
	case "darwin":
		imageViewer = "open"
	default:
		return func() {}, trace.NotImplemented("showing QR codes is not implemented on %s", runtime.GOOS)
	}

	otpImage, err := k.Image(456, 456)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imageFile, err := os.CreateTemp("", "teleport-otp-qr-code-*.png")
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer func() {
		if retErr != nil {
			imageFile.Close()
			os.Remove(imageFile.Name())
		}
	}()

	if err := png.Encode(imageFile, otpImage); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := imageFile.Close(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	log.Debugf("Wrote OTP QR code to %s", imageFile.Name())

	cmd := exec.Command(imageViewer, imageFile.Name())
	if err := cmd.Start(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	log.Debugf("Opened QR code via %q", imageViewer)
	return func() {
		if err := os.Remove(imageFile.Name()); err != nil {
			log.WithError(err).Debugf("Failed to clean up temporary QR code file %q", imageFile.Name())
		}
		if err := cmd.Process.Kill(); err != nil {
			log.WithError(err).Debug("Failed to stop the QR code image viewer")
		}
	}, nil
}
