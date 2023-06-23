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

package common

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"golang.org/x/exp/slices"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpDeviceType     = "TOTP"
	webauthnDeviceType = "WEBAUTHN"
	touchIDDeviceType  = "TOUCHID"
)

var (
	totpDeviceTypes = []string{totpDeviceType}
	webDeviceTypes  = initWebDevs()

	// defaultDeviceTypes lists the supported device types for `tsh mfa add`.
	defaultDeviceTypes = append(totpDeviceTypes, webDeviceTypes...)
)

func initWebDevs() []string {
	if touchid.IsAvailable() {
		return []string{webauthnDeviceType, touchIDDeviceType}
	}
	return []string{webauthnDeviceType}
}

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
	c.Flag("verbose", "Print more information about MFA devices").Short('v').BoolVar(&c.verbose)
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
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(cf.Context)
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

	// allowPasswordless is initially true if --allow-passwordless is set, false
	// if not explicitly requested.
	// It can only be set by users if wancli.IsFIDO2Available() is true.
	// Note that Touch ID registrations are always passwordless-capable,
	// regardless of other settings.
	allowPasswordless bool
}

func newMFAAddCommand(parent *kingpin.CmdClause) *mfaAddCommand {
	c := &mfaAddCommand{
		CmdClause: parent.Command("add", "Add a new MFA device."),
	}
	c.Flag("name", "Name of the new MFA device").StringVar(&c.devName)
	c.Flag("type", fmt.Sprintf("Type of the new MFA device (%s)", strings.Join(defaultDeviceTypes, ", "))).
		EnumVar(&c.devType, defaultDeviceTypes...)
	if wancli.IsFIDO2Available() {
		c.Flag("allow-passwordless", "Allow passwordless logins").BoolVar(&c.allowPasswordless)
	}
	return c
}

func (c *mfaAddCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx := cf.Context

	// Attempt to diagnose clamshell failures.
	if !slices.Contains(defaultDeviceTypes, touchIDDeviceType) {
		diag, err := touchid.Diag()
		if err == nil && diag.IsClamshellFailure() {
			log.Warn("Touch ID support disabled, is your MacBook lid closed?")
		}
	}

	if c.devType == "" {
		// If we are prompting the user for the device type, then take a glimpse at
		// server-side settings and adjust the options accordingly.
		// This is undesirable to do during flag setup, but we can do it here.
		pingResp, err := tc.Ping(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		c.devType, err = prompt.PickOne(
			ctx, os.Stdout, prompt.Stdin(),
			"Choose device type", deviceTypesFromSecondFactor(pingResp.Auth.SecondFactor))
		if err != nil {
			return trace.Wrap(err)
		}
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
		return trace.BadParameter("device name cannot be empty")
	}

	switch c.devType {
	case webauthnDeviceType:
		// Ask the user?
		// c.allowPasswordless=false at this point only means that the flag wasn't
		// explicitly set.
		if !c.allowPasswordless && wancli.IsFIDO2Available() {
			answer, err := prompt.PickOne(ctx, os.Stdout, prompt.Stdin(), "Allow passwordless logins", []string{"YES", "NO"})
			if err != nil {
				return trace.Wrap(err)
			}
			c.allowPasswordless = answer == "YES"
		}
	case touchIDDeviceType:
		// Touch ID is always a resident key/passwordless
		c.allowPasswordless = true
	}
	log.Debugf("tsh using passwordless registration? %v", c.allowPasswordless)

	dev, err := c.addDeviceRPC(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("MFA device %q added.\n", dev.Metadata.Name)
	return nil
}

func deviceTypesFromSecondFactor(sf constants.SecondFactorType) []string {
	switch sf {
	case constants.SecondFactorOTP:
		return totpDeviceTypes
	case constants.SecondFactorWebauthn:
		return webDeviceTypes
	default:
		return defaultDeviceTypes
	}
}

func (c *mfaAddCommand) addDeviceRPC(ctx context.Context, tc *client.TeleportClient) (*types.MFADevice, error) {
	devTypePB := map[string]proto.DeviceType{
		totpDeviceType:     proto.DeviceType_DEVICE_TYPE_TOTP,
		webauthnDeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		touchIDDeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[c.devType]
	// Sanity check.
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected device type: %q", c.devType)
	}

	var dev *types.MFADevice
	if err := client.RetryWithRelogin(ctx, tc, func() error {
		pc, err := tc.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(ctx)
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
		usage := proto.DeviceUsage_DEVICE_USAGE_MFA
		if c.allowPasswordless {
			usage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
		}
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_Init{
			Init: &proto.AddMFADeviceRequestInit{
				DeviceName:  c.devName,
				DeviceType:  devTypePB,
				DeviceUsage: usage,
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
		authResp, err := tc.PromptMFAChallenge(ctx, "" /* proxyAddr */, authChallenge, func(opts *client.PromptMFAChallengeOpts) {
			opts.PromptDevicePrefix = "*registered* "
		})
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
		regResp, regCallback, err := promptRegisterChallenge(ctx, tc.WebProxyAddr, c.devType, regChallenge)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{
			NewMFARegisterResponse: regResp,
		}}); err != nil {
			regCallback.Rollback()
			return trace.Wrap(err)
		}

		// Receive registered device ack.
		resp, err = stream.Recv()
		if err != nil {
			// Don't rollback here, the registration may have been successful.
			return trace.Wrap(err)
		}
		ack := resp.GetAck()
		if ack == nil {
			// Don't rollback here, the registration may have been successful.
			return trace.BadParameter("server bug: server sent %T when client expected AddMFADeviceResponse_Ack", resp.Response)
		}
		dev = ack.Device

		return regCallback.Confirm()
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, nil
}

type registerCallback interface {
	Rollback() error
	Confirm() error
}

type noopRegisterCallback struct{}

func (n noopRegisterCallback) Rollback() error {
	return nil
}

func (n noopRegisterCallback) Confirm() error {
	return nil
}

func promptRegisterChallenge(ctx context.Context, proxyAddr, devType string, c *proto.MFARegisterChallenge) (*proto.MFARegisterResponse, registerCallback, error) {
	switch c.Request.(type) {
	case *proto.MFARegisterChallenge_TOTP:
		resp, err := promptTOTPRegisterChallenge(ctx, c.GetTOTP())
		return resp, noopRegisterCallback{}, err

	case *proto.MFARegisterChallenge_Webauthn:
		origin := proxyAddr
		if !strings.HasPrefix(proxyAddr, "https://") {
			origin = "https://" + origin
		}
		cc := wanlib.CredentialCreationFromProto(c.GetWebauthn())

		if devType == touchIDDeviceType {
			return promptTouchIDRegisterChallenge(origin, cc)
		}

		resp, err := promptWebauthnRegisterChallenge(ctx, origin, cc)
		return resp, noopRegisterCallback{}, err

	default:
		return nil, nil, trace.BadParameter("server bug: unexpected registration challenge type: %T", c.Request)
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
		totpCode, err = prompt.Password(
			ctx, os.Stdout, prompt.Stdin(), "Once created, enter an OTP code generated by the app")
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

func promptWebauthnRegisterChallenge(ctx context.Context, origin string, cc *wanlib.CredentialCreation) (*proto.MFARegisterResponse, error) {
	log.Debugf("WebAuthn: prompting MFA devices with origin %q", origin)

	prompt := wancli.NewDefaultPrompt(ctx, os.Stdout)
	prompt.PINMessage = "Enter your *new* security key PIN"
	prompt.FirstTouchMessage = "Tap your *new* security key"
	prompt.SecondTouchMessage = "Tap your *new* security key again to complete registration"

	resp, err := wancli.Register(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

func promptTouchIDRegisterChallenge(origin string, cc *wanlib.CredentialCreation) (*proto.MFARegisterResponse, registerCallback, error) {
	log.Debugf("Touch ID: prompting registration with origin %q", origin)

	reg, err := touchid.Register(origin, cc)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wanlib.CredentialCreationResponseToProto(reg.CCR),
		},
	}, reg, nil
}

type mfaRemoveCommand struct {
	*kingpin.CmdClause
	name string
}

func newMFARemoveCommand(parent *kingpin.CmdClause) *mfaRemoveCommand {
	c := &mfaRemoveCommand{
		CmdClause: parent.Command("rm", "Remove a MFA device."),
	}
	c.Arg("name", "Name or ID of the MFA device to remove").Required().StringVar(&c.name)
	return c
}

func (c *mfaRemoveCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(cf.Context)
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
		authResp, err := tc.PromptMFAChallenge(cf.Context, "" /* proxyAddr */, authChallenge, nil /* applyOpts */)
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
		// If deleted device was webauthn device, try to delete touch-id credentials.
		if wanDevice := ack.GetDevice().GetWebauthn(); wanDevice != nil {
			deleteTouchIDCredentialIfApplicable(string(wanDevice.CredentialId))
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
	// imageViewerArgs is used to send additional arguments to exec command.
	var imageViewerArgs []string
	switch runtime.GOOS {
	case "linux":
		imageViewer = "xdg-open"
	case "darwin":
		imageViewer = "open"
	case "windows":
		// On windows start and many other commands are not executable files,
		// rather internal commands of Command prompt. In order to use internal
		// command it need to executed as: `cmd.exe /c start filename`
		imageViewer = "cmd.exe"
		imageViewerArgs = []string{"/c", "start"}
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

	cmd := exec.Command(imageViewer, append(imageViewerArgs, imageFile.Name())...)
	if err := cmd.Start(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	log.Debugf("Opened QR code via %q", imageViewer)
	return func() {
		if err := utils.RemoveSecure(imageFile.Name()); err != nil {
			log.WithError(err).Debugf("Failed to clean up temporary QR code file %q", imageFile.Name())
		}
		if err := cmd.Process.Kill(); err != nil {
			log.WithError(err).Debug("Failed to stop the QR code image viewer")
		}
	}, nil
}

func deleteTouchIDCredentialIfApplicable(credentialID string) {
	switch err := touchid.AttemptDeleteNonInteractive(credentialID); {
	case errors.Is(err, &touchid.ErrAttemptFailed{}):
		// Nothing to do here, just proceed.
	case err != nil:
		log.WithError(err).Errorf("Failed to delete credential: %s\n", credentialID)
	}
}
