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
	"encoding/base32"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	wanwin "github.com/gravitational/teleport/lib/auth/webauthnwin"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

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

		usage := proto.DeviceUsage_DEVICE_USAGE_MFA
		if c.allowPasswordless {
			usage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
		}

		// Issue the authn challenge.
		// Required for the registration challenge.
		authChallenge, err := aci.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Tweak Windows platform messages so it's clear we whether we are prompting
		// for the *registered* or *new* device.
		// We do it here, preemptively, because it's the simpler solution (instead
		// of finding out whether it is a Windows prompt or not).
		const registeredMsg = "Using platform authentication for *registered* device, follow the OS dialogs"
		const newMsg = "Using platform authentication for *new* device, follow the OS dialogs"
		defer wanwin.ResetPromptPlatformMessage()
		wanwin.PromptPlatformMessage = registeredMsg

		// Prompt for authentication.
		// Does nothing if no challenges were issued (aka user has no devices).
		authnResp, err := tc.NewMFAPrompt(mfa.WithPromptDeviceType(mfa.DeviceDescriptorRegistered)).Run(ctx, authChallenge)
		if err != nil {
			return trace.Wrap(err)
		}

		// Issue the registration challenge.
		registerChallenge, err := aci.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			ExistingMFAResponse: authnResp,
			DeviceType:          devTypePB,
			DeviceUsage:         usage,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Prompt for registration.
		wanwin.PromptPlatformMessage = newMsg
		registerResp, registerCallback, err := promptRegisterChallenge(ctx, tc.WebProxyAddr, c.devType, registerChallenge)
		if err != nil {
			return trace.Wrap(err)
		}

		// Complete registration and confirm new key.
		addResp, err := aci.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
			NewDeviceName:  c.devName,
			NewMFAResponse: registerResp,
			DeviceUsage:    usage,
		})
		if err != nil {
			registerCallback.Rollback() // Attempt to delete new key.
			return trace.Wrap(err)
		}
		if err := registerCallback.Confirm(); err != nil {
			return trace.Wrap(err)
		}

		dev = addResp.Device
		return nil
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
		cc := wantypes.CredentialCreationFromProto(c.GetWebauthn())

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
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{
				Code: totpCode,
				ID:   c.ID,
			},
		},
	}, nil
}

func promptWebauthnRegisterChallenge(ctx context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	log.Debugf("WebAuthn: prompting MFA devices with origin %q", origin)

	prompt := wancli.NewDefaultPrompt(ctx, os.Stdout)
	prompt.PINMessage = "Enter your *new* security key PIN"
	prompt.FirstTouchMessage = "Tap your *new* security key"
	prompt.SecondTouchMessage = "Tap your *new* security key again to complete registration"

	resp, err := wancli.Register(ctx, origin, cc, prompt)
	return resp, trace.Wrap(err)
}

func promptTouchIDRegisterChallenge(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, registerCallback, error) {
	log.Debugf("Touch ID: prompting registration with origin %q", origin)

	reg, err := touchid.Register(origin, cc)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(reg.CCR),
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

	ctx := cf.Context
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

		// Lookup device to delete.
		// This lets us exit early if the device doesn't exist and enables the
		// Touch ID cleanup at the end.
		devicesResp, err := aci.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
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

		// Issue and solve authn challenge.
		authnChal, err := aci.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		authnSolved, err := tc.PromptMFA(ctx, authnChal)
		if err != nil {
			return trace.Wrap(err)
		}

		// Delete device.
		if err := aci.DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
			DeviceName:          c.name,
			ExistingMFAResponse: authnSolved,
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
