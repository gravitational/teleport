//go:build libfido2
// +build libfido2

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

package webauthncli

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"
	"github.com/keys-pub/go-libfido2"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

const (
	// Max wait time for closing devices, before "abandoning" the device
	// goroutine.
	fido2DeviceMaxWait = 100 * time.Millisecond

	// Timeout for blocking operations.
	// Functions fail with FIDO_ERR_RX on timeout.
	fido2DeviceTimeout = 30 * time.Second

	// Operation retry interval.
	// Keep it less frequent than 5Hz / 0.2s.
	fido2RetryInterval = 500 * time.Millisecond

	// Timeout for touch.Status operations.
	// Keep it less frequent than 5Hz / 0.2s.
	fido2TouchMaxWait = 200 * time.Millisecond
)

// User-friendly device filter errors.
var (
	errHasExcludedCredential = errors.New("device already holds a registered credential")
	errNoPasswordless        = errors.New("device not registered for passwordless")
	errNoPlatform            = errors.New("device cannot fulfill platform attachment requirement")
	errNoRK                  = errors.New("device lacks resident key capabilities")
	errNoUV                  = errors.New("device lacks PIN or user verification capabilities necessary to support passwordless")
	errPasswordlessU2F       = errors.New("U2F devices cannot do passwordless")
)

// TouchRequest abstracts *libfido2.TouchRequest for testing.
type TouchRequest interface {
	Status(timeout time.Duration) (touched bool, err error)
	Stop() error
}

// FIDODevice abstracts *libfido2.Device for testing.
type FIDODevice interface {
	// Info mirrors libfido2.Device.Info.
	Info() (*libfido2.DeviceInfo, error)

	// IsFIDO2 mirrors libfido2.Device.IsFIDO2.
	IsFIDO2() (bool, error)

	// Cancel mirrors libfido2.Device.Cancel.
	Cancel() error

	// Close mirrors libfido2.Device.Close.
	Close() error

	// SetTimeout mirrors libfido2.Device.SetTimeout.
	SetTimeout(d time.Duration) error

	// MakeCredential mirrors libfido2.Device.MakeCredential.
	MakeCredential(
		clientDataHash []byte,
		rp libfido2.RelyingParty,
		user libfido2.User,
		typ libfido2.CredentialType,
		pin string,
		opts *libfido2.MakeCredentialOpts) (*libfido2.Attestation, error)

	// Assertion mirrors libfido2.Device.Assertion.
	Assertion(
		rpID string,
		clientDataHash []byte,
		credentialIDs [][]byte,
		pin string,
		opts *libfido2.AssertionOpts) ([]*libfido2.Assertion, error)

	// TouchBegin mirrors libfido2.Device.TouchBegin.
	TouchBegin() (TouchRequest, error)
}

type fido2DeviceAdapter struct {
	*libfido2.Device
}

func (a *fido2DeviceAdapter) TouchBegin() (TouchRequest, error) {
	return a.Device.TouchBegin()
}

// fidoDeviceLocations and fidoNewDevice are used to allow testing.
var (
	fidoDeviceLocations = libfido2.DeviceLocations
	fidoNewDevice       = func(path string) (FIDODevice, error) {
		dev, err := libfido2.NewDevice(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &fido2DeviceAdapter{dev}, nil
	}
)

// isLibfido2Enabled returns true if libfido2 is available in the current build.
func isLibfido2Enabled() bool {
	val, ok := os.LookupEnv("TELEPORT_FIDO2")
	// Default to enabled, otherwise obey the env variable.
	return !ok || val == "1"
}

// fido2Login implements FIDO2Login.
func fido2Login(
	ctx context.Context,
	origin string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	switch {
	case origin == "":
		return nil, "", trace.BadParameter("origin required")
	case prompt == nil:
		return nil, "", trace.BadParameter("prompt required")
	}
	if err := assertion.Validate(); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if opts == nil {
		opts = &LoginOpts{}
	}

	allowedCreds := assertion.Response.GetAllowedCredentialIDs()
	uv := assertion.Response.UserVerification == protocol.VerificationRequired

	// Presence of any allowed credential is interpreted as the user identity
	// being partially established, aka non-passwordless.
	passwordless := len(allowedCreds) == 0
	log.Debugf("FIDO2: assertion: passwordless=%v, uv=%v, %v allowed credentials", passwordless, uv, len(allowedCreds))

	// Prepare challenge data for the device.
	ccdJSON, err := json.Marshal(&CollectedClientData{
		Type:      string(protocol.AssertCeremony),
		Challenge: base64.RawURLEncoding.EncodeToString(assertion.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccdJSON)

	rpID := assertion.Response.RelyingPartyID
	var appID string
	if val, ok := assertion.Response.Extensions[wantypes.AppIDExtension]; ok {
		appID = fmt.Sprint(val)
	}

	// mu guards the variables below it.
	var mu sync.Mutex
	var assertionResp *libfido2.Assertion
	var usedAppID bool

	filter := func(dev FIDODevice, info *deviceInfo) error {
		switch {
		case !info.fido2 && (uv || passwordless):
			return errPasswordlessU2F
		case passwordless && (!info.uvCapable() || !info.rk):
			return errNoPasswordless
		case uv && !info.uvCapable():
			// Unlikely that we would ask for UV without passwordless, but let's check
			// just in case.
			// If left unchecked this causes libfido2.ErrUnsupportedOption.
			return errNoUV
		default:
			return nil
		}
	}

	user := opts.User
	deviceCallback := func(dev FIDODevice, info *deviceInfo, pin string) error {
		actualRPID := rpID
		if usesAppID(dev, info, ccdHash[:], allowedCreds, rpID, appID) {
			log.Debugf("FIDO2: Device %v registered for AppID (%q) instead of RPID", info.path, appID)
			actualRPID = appID
		}

		opts := &libfido2.AssertionOpts{
			UP: libfido2.True,
		}
		// Note that "uv" fails for PIN-capable devices with an empty PIN.
		// This is handled by runOnFIDO2Devices.
		if uv {
			opts.UV = libfido2.True
		}
		assertions, err := dev.Assertion(actualRPID, ccdHash[:], allowedCreds, pin, opts)
		if errors.Is(err, libfido2.ErrUnsupportedOption) && uv && pin != "" {
			// Try again if we are getting "unsupported option" and the PIN is set.
			// Happens inconsistently in some authenticator series (YubiKey 5).
			// We are relying on the fact that, because the PIN is set, the
			// authenticator will set the UV bit regardless of it being requested.
			log.Debugf("FIDO2: Device %v: retrying assertion without UV", info.path)
			opts.UV = libfido2.Default
			assertions, err = dev.Assertion(actualRPID, ccdHash[:], allowedCreds, pin, opts)
		}
		if errors.Is(err, libfido2.ErrNoCredentials) {
			// U2F devices error instantly with ErrNoCredentials.
			// If that is the case, we mark the error as non-interactive and continue
			// without this device. This is the only safe option, as it lets the
			// handleDevice goroutine exit gracefully. Do not attempt to wait for
			// touch - this causes another slew of problems with abandoned U2F
			// goroutines during registration.
			if !info.fido2 {
				log.Debugf("FIDO2: U2F device %v not registered, ignoring it", info.path)
				err = &nonInteractiveError{err: err}
			} else {
				err = ErrUsingNonRegisteredDevice // "Upgrade" error message.
			}
		}
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("FIDO2: Got %v assertions", len(assertions))

		// Find assertion for target user, or show the prompt.
		assertion, err := pickAssertion(assertions, prompt, user, passwordless)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Debugf(
			"FIDO2: Authenticated: credential ID (b64) = %v, user ID (hex) = %x, user name = %q",
			base64.RawURLEncoding.EncodeToString(assertion.CredentialID), assertion.User.ID, assertion.User.Name)

		// Use the first successful assertion.
		// In practice it is very unlikely we'd hit this twice.
		mu.Lock()
		if assertionResp == nil {
			assertionResp = assertion
			usedAppID = actualRPID != rpID
		}
		mu.Unlock()
		return nil
	}

	if err := runOnFIDO2Devices(ctx, prompt, filter, deviceCallback); err != nil {
		return nil, "", trace.Wrap(err)
	}

	var rawAuthData []byte
	if err := cbor.Unmarshal(assertionResp.AuthDataCBOR, &rawAuthData); err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Trust the assertion user if present, otherwise say nothing.
	actualUser := assertionResp.User.Name

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: &wanpb.CredentialAssertionResponse{
				Type:  string(protocol.PublicKeyCredentialType),
				RawId: assertionResp.CredentialID,
				Response: &wanpb.AuthenticatorAssertionResponse{
					ClientDataJson:    ccdJSON,
					AuthenticatorData: rawAuthData,
					Signature:         assertionResp.Sig,
					UserHandle:        assertionResp.User.ID,
				},
				Extensions: &wanpb.AuthenticationExtensionsClientOutputs{
					AppId: usedAppID,
				},
			},
		},
	}, actualUser, nil
}

func usesAppID(dev FIDODevice, info *deviceInfo, ccdHash []byte, allowedCreds [][]byte, rpID, appID string) bool {
	if appID == "" {
		return false
	}

	opts := &libfido2.AssertionOpts{
		UP: libfido2.False,
	}

	isRegistered := func(id string) bool {
		const pin = "" // Not necessary here.
		_, err := dev.Assertion(id, ccdHash, allowedCreds, pin, opts)
		return err == nil || (!info.fido2 && errors.Is(err, libfido2.ErrUserPresenceRequired))
	}

	return isRegistered(appID) && !isRegistered(rpID)
}

func pickAssertion(
	assertions []*libfido2.Assertion, prompt LoginPrompt, user string, passwordless bool,
) (*libfido2.Assertion, error) {
	switch l := len(assertions); {
	// Shouldn't happen, but let's be safe and handle it anyway.
	case l == 0:
		return nil, errors.New("authenticator returned empty assertions")

	// MFA or single account.
	// Note that authenticators don't return the user name, display name or icon
	// for a single account per RP.
	// See the authenticatorGetAssertion response, user member (0x04):
	// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorgetassertion-response-structure
	case !passwordless, l == 1:
		return assertions[0], nil

	// Explicit user required. First occurrence wins.
	case user != "":
		for _, assertion := range assertions {
			if assertion.User.Name == user {
				return assertion, nil
			}
		}
		return nil, fmt.Errorf("no credentials for user %q", user)
	}

	// Prepare credentials and show picker.
	creds := make([]*CredentialInfo, len(assertions))
	credToAssertion := make(map[*CredentialInfo]*libfido2.Assertion)
	for i, assertion := range assertions {
		cred := &CredentialInfo{
			ID: assertion.CredentialID,
			User: UserInfo{
				UserHandle: assertion.User.ID,
				Name:       assertion.User.Name,
			},
		}
		credToAssertion[cred] = assertion
		creds[i] = cred
	}
	chosen, err := prompt.PromptCredential(creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assertion, ok := credToAssertion[chosen]
	if !ok {
		return nil, fmt.Errorf("prompt returned invalid credential: %#v", chosen)
	}
	return assertion, nil
}

// fido2Register implements FIDO2Register.
func fido2Register(
	ctx context.Context,
	origin string, cc *wantypes.CredentialCreation, prompt RegisterPrompt,
) (*proto.MFARegisterResponse, error) {
	switch {
	case origin == "":
		return nil, trace.BadParameter("origin required")
	case prompt == nil:
		return nil, trace.BadParameter("prompt required")
	}
	if err := cc.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	rrk, err := cc.RequireResidentKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("FIDO2: registration: resident key=%v", rrk)

	// Can we create ES256 keys?
	// TODO(codingllama): Consider supporting other algorithms and respecting
	//  param order in the credential.
	ok := false
	for _, p := range cc.Response.Parameters {
		if p.Type == protocol.PublicKeyCredentialType && p.Algorithm == webauthncose.AlgES256 {
			ok = true
			break
		}
	}
	if !ok {
		return nil, trace.BadParameter("ES256 not allowed by credential parameters")
	}

	// Prepare challenge data for the device.
	ccdJSON, err := json.Marshal(&CollectedClientData{
		Type:      string(protocol.CreateCeremony),
		Challenge: base64.RawURLEncoding.EncodeToString(cc.Response.Challenge),
		Origin:    origin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccdHash := sha256.Sum256(ccdJSON)

	rp := libfido2.RelyingParty{
		ID:   cc.Response.RelyingParty.ID,
		Name: cc.Response.RelyingParty.Name,
	}
	user := libfido2.User{
		ID:          cc.Response.User.ID,
		Name:        cc.Response.User.Name,
		DisplayName: cc.Response.User.DisplayName,
	}
	plat := cc.Response.AuthenticatorSelection.AuthenticatorAttachment == protocol.Platform
	uv := cc.Response.AuthenticatorSelection.UserVerification == protocol.VerificationRequired

	excludeList := make([][]byte, len(cc.Response.CredentialExcludeList))
	for i := range cc.Response.CredentialExcludeList {
		excludeList[i] = cc.Response.CredentialExcludeList[i].CredentialID
	}

	// mu guards attestation from goroutines.
	var mu sync.Mutex
	var attestation *libfido2.Attestation

	filter := func(dev FIDODevice, info *deviceInfo) error {
		switch {
		case !info.fido2 && (rrk || uv):
			return errPasswordlessU2F
		case plat && !info.plat:
			return errNoPlatform
		case rrk && !info.rk:
			return errNoRK
		case uv && !info.uvCapable():
			return errNoUV
		case len(excludeList) == 0:
			return nil
		}

		// Does the device hold an excluded credential?
		const pin = "" // not required to filter
		switch _, err := dev.Assertion(rp.ID, ccdHash[:], excludeList, pin, &libfido2.AssertionOpts{
			UP: libfido2.False,
		}); {
		case errors.Is(err, libfido2.ErrNoCredentials):
			return nil
		case errors.Is(err, libfido2.ErrUserPresenceRequired):
			// Yubikey4 does this when the credential exists.
			return errHasExcludedCredential
		case err != nil:
			// Swallow unexpected errors: a double registration is better than
			// aborting the ceremony.
			log.Debugf(
				"FIDO2: Device %v: excluded credential assertion failed, letting device through: err=%q",
				info.path, err)
			return nil
		default:
			log.Debugf("FIDO2: Device %v: filtered due to presence of excluded credential", info.path)
			return errHasExcludedCredential
		}
	}

	deviceCallback := func(d FIDODevice, info *deviceInfo, pin string) error {
		// TODO(codingllama): We may need to setup a PIN if rrk=true.
		//  Do that as a response to specific MakeCredential failures.

		opts := &libfido2.MakeCredentialOpts{}
		if rrk {
			opts.RK = libfido2.True
		}
		// Only set the "uv" bit if the authenticator supports built-in
		// verification. PIN-enabled devices don't claim to support "uv", but they
		// are capable of UV assertions.
		// See
		// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#getinfo-uv.
		if uv && info.uv {
			opts.UV = libfido2.True
		}

		resp, err := d.MakeCredential(ccdHash[:], rp, user, libfido2.ES256, pin, opts)
		if err != nil {
			return trace.Wrap(err)
		}

		// Use the first successful attestation.
		// In practice it is very unlikely we'd hit this twice.
		mu.Lock()
		if attestation == nil {
			attestation = resp
		}
		mu.Unlock()
		return nil
	}

	if err := runOnFIDO2Devices(ctx, prompt, filter, deviceCallback); err != nil {
		return nil, trace.Wrap(err)
	}

	var rawAuthData []byte
	if err := cbor.Unmarshal(attestation.AuthData, &rawAuthData); err != nil {
		return nil, trace.Wrap(err)
	}

	format, attStatement, err := makeAttStatement(attestation)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attObj := &protocol.AttestationObject{
		RawAuthData:  rawAuthData,
		Format:       format,
		AttStatement: attStatement,
	}
	attestationCBOR, err := cbor.Marshal(attObj)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: &wanpb.CredentialCreationResponse{
				Type:  string(protocol.PublicKeyCredentialType),
				RawId: attestation.CredentialID,
				Response: &wanpb.AuthenticatorAttestationResponse{
					ClientDataJson:    ccdJSON,
					AttestationObject: attestationCBOR,
				},
			},
		},
	}, nil
}

func makeAttStatement(attestation *libfido2.Attestation) (string, map[string]interface{}, error) {
	const fidoU2F = "fido-u2f"
	const none = "none"
	const packed = "packed"

	// See https://www.w3.org/TR/webauthn-2/#sctn-defined-attestation-formats.
	// The formats handled below are what we expect from the keys libfido2
	// interacts with.
	format := attestation.Format
	switch format {
	case fidoU2F, packed: // OK, continue below
	case none:
		return format, nil, nil
	default:
		log.Debugf(`FIDO2: Unsupported attestation format %q, using "none"`, format)
		return none, nil, nil
	}

	sig := attestation.Sig
	if len(sig) == 0 {
		return "", nil, trace.BadParameter("attestation %q without signature", format)
	}
	cert := attestation.Cert
	if len(cert) == 0 {
		return "", nil, trace.BadParameter("attestation %q without certificate", format)
	}

	m := map[string]interface{}{
		"sig": sig,
		"x5c": []interface{}{cert},
	}
	if format == packed {
		m["alg"] = int64(attestation.CredentialType)
	}

	return format, m, nil
}

type (
	deviceFilterFunc     func(dev FIDODevice, info *deviceInfo) error
	deviceCallbackFunc   func(dev FIDODevice, info *deviceInfo, pin string) error
	pinAwareCallbackFunc func(dev FIDODevice, info *deviceInfo, pin string) (requiresPIN bool, err error)
)

// runPrompt defines the prompt operations necessary for runOnFIDO2Devices.
// (RegisterPrompt happens to match the minimal interface required.)
type runPrompt RegisterPrompt

func runOnFIDO2Devices(
	ctx context.Context,
	prompt runPrompt,
	filter deviceFilterFunc,
	deviceCallback deviceCallbackFunc,
) error {
	locs, err := fidoDeviceLocations()
	if err != nil {
		return trace.Wrap(err, "device locations")
	}
	if len(locs) == 0 {
		return trace.Wrap(errors.New("no security keys found"))
	}

	devices, devicesC, err := startDevices(locs, filter, deviceCallback, prompt)
	if err != nil {
		return trace.Wrap(err)
	}

	var receiveCount int
	defer func() {
		// Cancel all in-flight requests, if any.
		devices.cancelAll(nil /* except */)

		// Give the devices some time to tidy up, but don't wait forever.
		maxWait := time.NewTimer(fido2DeviceMaxWait)
		defer maxWait.Stop()

		for receiveCount < devices.len() {
			select {
			case <-devicesC:
				receiveCount++
			case <-maxWait.C:
				log.Debugf("FIDO2: Abandoning device goroutines after %s", fido2DeviceMaxWait)
				return
			}
		}
		log.Debug("FIDO2: Device goroutines exited cleanly")
	}()

	// First "interactive" response wins.
	for receiveCount < devices.len() {
		select {
		case err := <-devicesC:
			receiveCount++

			// Keep going on cancels or non-interactive errors.
			if errors.Is(err, libfido2.ErrKeepaliveCancel) || errors.Is(err, &nonInteractiveError{}) {
				log.Debugf("FIDO2: Got cancel or non-interactive device error: %v", err)
				continue
			}

			return trace.Wrap(err)

		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
	return trace.Wrap(errors.New("all MFA devices failed"))
}

func startDevices(
	locs []*libfido2.DeviceLocation,
	filter deviceFilterFunc,
	deviceCallback deviceCallbackFunc,
	prompt runPrompt,
) (devices *openedDevices, devicesC <-chan error, err error) {
	fidoDevs := make([]FIDODevice, 0, len(locs))
	openDevs := make([]*openedDevice, 0, len(locs))

	// closeAll should only be used until the devices are handed over.
	// Do not defer-call it.
	closeAll := func() {
		for i, dev := range fidoDevs {
			path := openDevs[i].path
			err := dev.Close()
			log.Debugf("FIDO2: Close device %v, err=%v", path, err)
		}
	}

	// Open all devices in one go.
	// This ensures cancels propagate to the complete list.
	for _, loc := range locs {
		path := loc.Path

		dev, err := fidoNewDevice(path)
		if err != nil {
			// Be resilient to open errors.
			// This can happen to devices that failed to cancel (and thus are still
			// asserting) when we run sequential operations. For example: registration
			// immediately followed by assertion (in a single process).
			// This is largely safe to ignore, as opening is fairly consistent in
			// other situations and failures are likely from a non-chosen device in
			// multi-device scenarios.
			log.Debugf("FIDO2: Device %v failed to open, skipping: %v", path, err)
			continue
		}

		fidoDevs = append(fidoDevs, dev)
		openDevs = append(openDevs, &openedDevice{
			path: path,
			dev:  dev,
		})
	}
	if len(fidoDevs) == 0 {
		return nil, nil, errors.New("failed to open security keys")
	}

	// Prompt touch, it's about to begin.
	ackTouch, err := prompt.PromptTouch()
	if err != nil {
		closeAll()
		return nil, nil, trace.Wrap(err)
	}
	//nolint:ineffassign // closeAll not meant to be used from here onwards.
	closeAll = nil

	errC := make(chan error, len(fidoDevs))
	devices = &openedDevices{
		devices: openDevs,
	}

	// Fire device handling goroutines.
	// From this point onwards devices are owned by their respective goroutines,
	// only cancels are supposed to happen outside of them.
	for i, dev := range fidoDevs {
		path := openDevs[i].path
		dev := dev
		go func() {
			errC <- handleDevice(path, dev, filter, deviceCallback, devices.cancelAll, ackTouch, prompt)
		}()
	}

	return devices, errC, nil
}

type openedDevice struct {
	path string

	// dev is the opened device.
	// Only cancels may be issued outside of the handleDevice goroutine.
	dev interface{ Cancel() error }

	// Keep tabs on canceled devices to avoid multiple cancels.
	canceled bool
}

type openedDevices struct {
	// mu guards device changes and cancelAll().
	// Note that the size of the devices slice doesn't change after it's assigned,
	// only the `canceled` device field changes.
	mu      sync.Mutex
	devices []*openedDevice
}

func (l *openedDevices) len() int {
	// Safe to read without locking, the size of the slice doesn't change after
	// assigned.
	return len(l.devices)
}

// cancelAll cancels all devices but `except`.
func (l *openedDevices) cancelAll(except FIDODevice) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, d := range l.devices {
		if d.dev == except || d.canceled {
			continue
		}

		d.canceled = true

		// Note that U2F devices fail Cancel with "invalid argument".
		err := d.dev.Cancel()
		log.Debugf("FIDO2: Cancel device %v, err=%v", d.path, err)
	}
}

// handleDevice handles all device interactions, apart from external cancels.
func handleDevice(
	path string,
	dev FIDODevice,
	filter deviceFilterFunc, deviceCallback deviceCallbackFunc,
	cancelAll func(except FIDODevice),
	firstTouchAck func() error,
	pinPrompt runPrompt,
) error {
	// handleDevice owns the device, thus it has the privilege to shut it down.
	defer func() {
		err := dev.Close()
		log.Debugf("FIDO2: Close device %v, err=%v", path, err)
	}()

	if err := dev.SetTimeout(fido2DeviceTimeout); err != nil {
		return trace.Wrap(&nonInteractiveError{err: err})
	}

	// Gather device information.
	var info *libfido2.DeviceInfo
	isFIDO2, err := dev.IsFIDO2()
	if err != nil {
		return trace.Wrap(&nonInteractiveError{err: err})
	}
	if isFIDO2 {
		info, err = devInfo(path, dev)
		if err != nil {
			return trace.Wrap(&nonInteractiveError{err: err})
		}
		log.Debugf("FIDO2: Device %v: info %#v", path, info)
	} else {
		log.Debugf("FIDO2: Device %v: not a FIDO2 device", path)
	}
	di := makeDevInfo(path, info, isFIDO2)

	// Apply initial filters, waiting for confirmation if the filter fails before
	// relaying the error.
	if err := filter(dev, di); err != nil {
		log.Debugf("FIDO2: Device %v filtered, err=%v", path, err)

		// If the device is chosen then treat the error as interactive.
		if touched, _ := waitForTouch(dev); touched {
			cancelAll(dev)
		} else {
			err = &nonInteractiveError{err: err}
		}
		return trace.Wrap(err)
	}

	// Run the callback.
	cb := withPINHandler(withRetries(deviceCallback))
	requiresPIN, err := cb(dev, di, "" /* pin */)
	log.Debugf("FIDO2: Device %v: callback returned, requiresPIN=%v, err=%v", path, requiresPIN, err)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := firstTouchAck(); err != nil {
		return trace.Wrap(err)
	}

	// Cancel other devices only on success. This avoids multiple cancel attempts
	// as non-chosen devices return FIDO_ERR_KEEPALIVE_CANCEL.
	cancelAll(dev)

	if !requiresPIN {
		return nil
	}

	// Ask for PIN, prompt for next touch.
	pin, err := pinPrompt.PromptPIN()
	switch {
	case err != nil:
		return trace.Wrap(err)
	case pin == "":
		return libfido2.ErrPinRequired
	}
	ackTouch, err := pinPrompt.PromptTouch()
	if err != nil {
		return trace.Wrap(err)
	}

	cb = withoutPINHandler(withRetries(deviceCallback))
	if _, err := cb(dev, di, pin); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(ackTouch())
}

func devInfo(path string, dev FIDODevice) (*libfido2.DeviceInfo, error) {
	const infoAttempts = 3
	var lastErr error
	for i := 0; i < infoAttempts; i++ {
		info, err := dev.Info()
		if err == nil {
			return info, nil
		}

		lastErr = err
		log.Debugf("FIDO2: Device %v: Info failed, retrying after %s: %v", path, fido2RetryInterval, err)
		time.Sleep(fido2RetryInterval)
	}

	return nil, trace.Wrap(lastErr)
}

// withRetries wraps callback with retries and error handling for commonly seen
// errors.
func withRetries(callback deviceCallbackFunc) deviceCallbackFunc {
	return func(dev FIDODevice, info *deviceInfo, pin string) error {
		const maxRetries = 3
		var err error
		for i := 0; i < maxRetries; i++ {
			err = callback(dev, info, pin)
			if err == nil {
				return nil
			}

			// Handle errors mapped by go-libfido2.
			// ErrOperationDenied happens when fingerprint reading fails (UV=false).
			if errors.Is(err, libfido2.ErrOperationDenied) {
				fmt.Println("Gesture validation failed, make sure you use a registered fingerprint")
				log.Debug("FIDO2: Retrying libfido2 error 'operation denied'")
				continue
			}

			// Handle generic libfido2.Error instances.
			var fidoErr libfido2.Error
			if !errors.As(err, &fidoErr) {
				return err
			}

			// See https://github.com/Yubico/libfido2/blob/main/src/fido/err.h#L32.
			switch fidoErr.Code {
			case 60: // FIDO_ERR_UV_BLOCKED, 0x3c
				const msg = "" +
					"The user verification function in your security key is blocked. " +
					"This is likely due to too many failed authentication attempts. " +
					"Consult your manufacturer documentation for how to unblock your security key. " +
					"Alternatively, you may unblock your device by using it in the Web UI."
				return trace.Wrap(err, msg)
			case 63: // FIDO_ERR_UV_INVALID, 0x3f
				log.Debug("FIDO2: Retrying libfido2 error 63")
				continue
			default: // Unexpected code.
				return err
			}
		}

		return fmt.Errorf("max retry attempts reached: %w", err)
	}
}

func withPINHandler(cb deviceCallbackFunc) pinAwareCallbackFunc {
	return func(dev FIDODevice, info *deviceInfo, pin string) (requiresPIN bool, err error) {
		// Attempt to select a device by running "deviceCallback" on it.
		// For most scenarios this works, saving a touch.
		err = cb(dev, info, pin)
		switch {
		case errors.Is(err, libfido2.ErrPinRequired):
			// Continued below.
		case errors.Is(err, libfido2.ErrUnsupportedOption) && pin == "" && !info.uv && info.clientPinSet:
			// The failing option is likely to be "UV", so we handle this the same as
			// ErrPinRequired: see if the user selects this device, ask for the PIN and
			// try again.
			// Continued below.
		default:
			return
		}

		// ErrPinRequired means we can't use "deviceCallback" as the selection
		// mechanism. Let's run a different operation to ask for a touch.
		requiresPIN = true

		if touched, _ := waitForTouch(dev); touched {
			err = nil // OK, selected successfully
		} else {
			err = &nonInteractiveError{err: err}
		}
		return
	}
}

func withoutPINHandler(cb deviceCallbackFunc) pinAwareCallbackFunc {
	return func(dev FIDODevice, info *deviceInfo, pin string) (bool, error) {
		return false, cb(dev, info, pin)
	}
}

// nonInteractiveError tags device errors that happen before user interaction.
// These are are usually ignored in the context of selecting devices.
type nonInteractiveError struct {
	err error
}

func (e *nonInteractiveError) Error() string {
	return e.err.Error()
}

func (e *nonInteractiveError) Is(err error) bool {
	_, ok := err.(*nonInteractiveError)
	return ok
}

func waitForTouch(dev FIDODevice) (touched bool, err error) {
	touch, err := dev.TouchBegin()
	if err != nil {
		// Error logged here as it's mostly ignored by callers.
		log.Debugf("FIDO2: Device touch begin error: %v", err)
		return false, trace.Wrap(err)
	}
	defer touch.Stop()

	// Block until we get a touch or a cancel.
	for {
		touched, err := touch.Status(fido2TouchMaxWait)
		if err != nil {
			// Error logged here as it's mostly ignored by callers.
			log.Debugf("FIDO2: Device touch status error: %v", err)
			return false, trace.Wrap(err)
		}
		if touched {
			return true, nil
		}
	}
}

// deviceInfo contains an aggregate of a device's information and capabilities.
// Various fields match options under
// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorGetInfo.
type deviceInfo struct {
	path                           string
	fido2                          bool
	plat                           bool
	rk                             bool
	clientPinCapable, clientPinSet bool
	uv                             bool
	bioEnroll                      bool
}

// uvCapable returns true for both "uv" and pin-configured devices.
func (di *deviceInfo) uvCapable() bool {
	return di.uv || di.clientPinSet
}

func makeDevInfo(path string, info *libfido2.DeviceInfo, fido2 bool) *deviceInfo {
	di := &deviceInfo{
		path:  path,
		fido2: fido2,
	}

	// U2F devices don't respond to dev.Info().
	if !fido2 {
		return di
	}

	for _, opt := range info.Options {
		// See
		// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorGetInfo.
		switch opt.Name {
		case "plat":
			di.plat = opt.Value == libfido2.True
		case "rk":
			di.rk = opt.Value == libfido2.True
		case "clientPin":
			di.clientPinCapable = true
			di.clientPinSet = opt.Value == libfido2.True
		case "uv":
			di.uv = opt.Value == libfido2.True
		case "bioEnroll":
			di.bioEnroll = opt.Value == libfido2.True
		}
	}
	return di
}
