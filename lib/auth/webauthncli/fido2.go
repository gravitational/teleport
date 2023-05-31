//go:build libfido2
// +build libfido2

// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

// User-friendly device filter errors.
var (
	errHasExcludedCredential   = errors.New("device already holds a registered credential")
	errNoPasswordless          = errors.New("device not registered for passwordless")
	errNoPlatform              = errors.New("device cannot fulfill platform attachment requirement")
	errNoRK                    = errors.New("device lacks resident key capabilities")
	errNoRegisteredCredentials = errors.New("device lacks registered credentials")
	errNoUV                    = errors.New("device lacks PIN or user verification capabilities")
	errPasswordlessU2F         = errors.New("U2F devices cannot do passwordless")
)

// FIDODevice abstracts *libfido2.Device for testing.
type FIDODevice interface {
	// Info mirrors libfido2.Device.Info.
	Info() (*libfido2.DeviceInfo, error)

	// Cancel mirrors libfido2.Device.Cancel.
	Cancel() error

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
}

// fidoDeviceLocations and fidoNewDevice are used to allow testing.
var (
	fidoDeviceLocations = libfido2.DeviceLocations
	fidoNewDevice       = func(path string) (FIDODevice, error) {
		return libfido2.NewDevice(path)
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
	origin string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
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
	if val, ok := assertion.Response.Extensions[wanlib.AppIDExtension]; ok {
		appID = fmt.Sprint(val)
	}

	// mu guards the variables below it.
	var mu sync.Mutex
	var assertionResp *libfido2.Assertion
	var usedAppID bool

	pathToRPID := &sync.Map{} // map[string]string
	filter := func(dev FIDODevice, info *deviceInfo) error {
		switch {
		case info.u2f && (uv || passwordless):
			return errPasswordlessU2F
		case passwordless && (!info.uvCapable() || !info.rk):
			return errNoPasswordless
		case uv && !info.uvCapable():
			// Unlikely that we would ask for UV without passwordless, but let's check
			// just in case.
			// If left unchecked this causes libfido2.ErrUnsupportedOption.
			return errNoUV
		case passwordless: // Nothing else to check
			return nil
		}

		// TODO(codingllama): Kill discoverRPID? It makes behavioral assumptions
		//  that caused problems before.
		// Does the device have a suitable credential?
		const pin = ""
		actualRPID, err := discoverRPID(dev, info, pin, rpID, appID, allowedCreds)
		if err != nil {
			return errNoRegisteredCredentials
		}
		pathToRPID.Store(info.path, actualRPID)

		return nil
	}

	user := opts.User
	deviceCallback := func(dev FIDODevice, info *deviceInfo, pin string) error {
		actualRPID := rpID
		if val, ok := pathToRPID.Load(info.path); ok {
			actualRPID = val.(string)
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

func discoverRPID(dev FIDODevice, info *deviceInfo, pin, rpID, appID string, allowedCreds [][]byte) (string, error) {
	// The actual hash is not necessary here.
	const cdh = "00000000000000000000000000000000"

	opts := &libfido2.AssertionOpts{
		UP: libfido2.False,
	}
	for _, id := range []string{rpID, appID} {
		if id == "" {
			continue
		}
		switch _, err := dev.Assertion(id, []byte(cdh), allowedCreds, pin, opts); {
		// Yubikey4 returns ErrUserPresenceRequired if the credential exists,
		// despite the UP=false opts above.
		case err == nil, errors.Is(err, libfido2.ErrUserPresenceRequired):
			return id, nil
		case errors.Is(err, libfido2.ErrNoCredentials):
			// Device not registered for RPID=id, keep trying.
		default:
			log.WithError(err).Debugf("FIDO2: Device %v: attempt RPID = %v", info.path, id)
		}
	}
	return "", libfido2.ErrNoCredentials
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
	origin string, cc *wanlib.CredentialCreation, prompt RegisterPrompt,
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
		Icon:        cc.Response.User.Icon,
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
		case info.u2f && (rrk || uv):
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

type deviceWithInfo struct {
	FIDODevice
	info *deviceInfo
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
	// About to select, prompt user.
	ackTouch, err := prompt.PromptTouch()
	if err != nil {
		return trace.Wrap(err)
	}

	// List/select devices.
	cb := withPINHandler(withRetries(deviceCallback))
	dev, requiresPIN, err := findAndSelectDevice(ctx, filter, cb)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := ackTouch(); err != nil {
		return trace.Wrap(err)
	}

	if !requiresPIN {
		return nil
	}

	// Selected device requires PIN, let's use the prompt and run the callback
	// again.
	pin, err := prompt.PromptPIN()
	switch {
	case err != nil:
		return trace.Wrap(err)
	case pin == "":
		return libfido2.ErrPinRequired
	}

	// Prompt a second touch after reading the PIN.
	ackTouch, err = prompt.PromptTouch()
	if err != nil {
		return trace.Wrap(err)
	}

	// Run the callback again with the informed PIN.
	// selectDevice is used since it correctly deals with cancellation.
	cb = withoutPINHandler(withRetries(deviceCallback))
	_, err = selectDevice(ctx, pin, dev, cb)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(ackTouch())
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

		err = waitForTouch(dev)
		if errors.Is(err, libfido2.ErrNoCredentials) {
			err = nil // OK, selected successfully
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

func withInteractiveError(filter deviceFilterFunc, cb pinAwareCallbackFunc) pinAwareCallbackFunc {
	return func(dev FIDODevice, info *deviceInfo, pin string) (bool, error) {
		filterErr := filter(dev, info)
		if filterErr == nil {
			return cb(dev, info, pin)
		}

		// U2F devices tend to cause problems with the waitForTouch strategy below,
		// so we filter them silently, as we used to do with all devices in previous
		// versions.
		if info.u2f {
			log.Warnf("FIDO2: Device %v: U2F device filtered due to lack of capabilities", info.path)
			return false, &nonInteractiveError{filterErr}
		}

		// Device got filtered out, let's see if the user chooses it and provide a
		// nice error message.
		switch waitErr := waitForTouch(dev); {
		case errors.Is(waitErr, libfido2.ErrKeepaliveCancel):
			// Device not chosen.
			return false, &nonInteractiveError{filterErr}
		case errors.Is(waitErr, libfido2.ErrNoCredentials):
			// Device chosen.
			// Escalate error to ErrUsingNonRegisteredDevice, if appropriate, so we
			// send a better message to the user.
			if errors.Is(filterErr, errNoRegisteredCredentials) {
				filterErr = ErrUsingNonRegisteredDevice
			}
		default:
			log.Warnf("FIDO2: Device %v: unexpected wait error: %q", info.path, waitErr)
		}

		return false, trace.Wrap(filterErr)
	}
}

func waitForTouch(dev FIDODevice) error {
	// TODO(codingllama): What we really want here is fido_dev_get_touch_begin.
	const rpID = "7f364cc0-958c-4177-b3ea-b2d8d7f15d4a" // arbitrary, unlikely to collide with a real RP
	const cdh = "00000000000000000000000000000000"      // "random", size 32
	_, err := dev.Assertion(rpID, []byte(cdh), nil /* credentials */, "", &libfido2.AssertionOpts{
		UP: libfido2.True,
	})
	return err
}

func findAndSelectDevice(ctx context.Context, filter deviceFilterFunc, deviceCallback pinAwareCallbackFunc) (dev *deviceWithInfo, requiresPIN bool, err error) {
	innerCtx, innerCancel := context.WithCancel(ctx)
	// innerCancel handled below.

	type devicesResp struct {
		devs []*deviceWithInfo
		err  error
	}
	// devicesC transport newly-found devices.
	// Closed on exit by the device poll goroutine below.
	devicesC := make(chan devicesResp)

	// Poll for new devices until the user selects one (via touch).
	// Runs until innerCtx is closed.
	go func() {
		defer close(devicesC)

		// knownPaths is retained between findDevices calls so only "new" devices
		// are returned.
		knownPaths := make(map[string]struct{})

		ticker := time.NewTicker(FIDO2PollInterval)
		defer ticker.Stop()

		for {
			devs, err := findDevices(knownPaths)
			devicesC <- devicesResp{
				devs: devs,
				err:  err,
			}

			select {
			case <-innerCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	type selectResp struct {
		dev         *deviceWithInfo
		requiresPIN bool
		err         error
	}
	selectC := make(chan selectResp)
	selectGoroutines := 0

	defer func() {
		innerCancel() // Cancel all goroutines

		// Collect select goroutines.
		for selectGoroutines > 0 {
			<-selectC
			selectGoroutines--
		}

		// Empty devicesC, if blocked.
		for {
			if _, open := <-devicesC; !open {
				break
			}
		}
	}()

	cb := withInteractiveError(filter, deviceCallback)
	for {
		select {
		// New devices found.
		case resp := <-devicesC:
			if resp.err != nil {
				return nil, false, trace.Wrap(resp.err)
			}
			for _, dev := range resp.devs {
				dev := dev
				selectGoroutines++
				go func() {
					requiresPIN, err := selectDevice(innerCtx, "" /* pin */, dev, cb)
					selectC <- selectResp{
						dev:         dev,
						requiresPIN: requiresPIN,
						err:         err,
					}
				}()
			}

		// User selected device.
		case resp := <-selectC:
			selectGoroutines--
			if errors.Is(resp.err, &nonInteractiveError{}) {
				continue
			}
			return resp.dev, resp.requiresPIN, trace.Wrap(resp.err)

		// Timed out.
		case <-ctx.Done():
			return nil, false, trace.Wrap(ctx.Err())
		}
	}
}

func findDevices(knownPaths map[string]struct{}) ([]*deviceWithInfo, error) {
	locs, err := fidoDeviceLocations()
	if err != nil {
		return nil, trace.Wrap(err, "device locations")
	}

	var devs []*deviceWithInfo
	for _, loc := range locs {
		path := loc.Path
		if _, ok := knownPaths[path]; ok {
			continue
		}
		knownPaths[path] = struct{}{}

		dev, err := fidoNewDevice(path)
		if err != nil {
			return nil, trace.Wrap(err, "device %v: open", path)
		}

		var info *libfido2.DeviceInfo
		var u2f bool
		const infoAttempts = 3
		for i := 0; i < infoAttempts; i++ {
			info, err = dev.Info()
			switch {
			case errors.Is(err, libfido2.ErrNotFIDO2):
				u2f = true
			case errors.Is(err, libfido2.ErrTX):
				// Happens occasionally, let's retry.
				fallthrough
			case err != nil:
				// Unexpected error, retry anyway.
				// Note that U2F devices fail in a variety of different ways.
				time.Sleep(100 * time.Millisecond)
				continue
			}
			break // err == nil
		}
		if !u2f && info == nil {
			log.Warnf("FIDO2: Device %v: max info attempts reached, treating as U2F", path)
			u2f = true
		}
		log.Debugf("FIDO2: Info for device %v: %#v", path, info)

		devs = append(devs, &deviceWithInfo{
			FIDODevice: dev,
			info:       makeDevInfo(path, info, u2f),
		})
	}

	if l := len(devs); l > 0 {
		log.Debugf("FIDO2: Found %v new devices", l)
	}

	return devs, nil
}

func selectDevice(
	ctx context.Context,
	pin string, dev *deviceWithInfo, cb pinAwareCallbackFunc,
) (requiresPIN bool, err error) {
	// Spin a goroutine to run the callback so we can deal with context
	// cancellation.
	done := make(chan struct{})
	go func() {
		requiresPIN, err = cb(dev, dev.info, pin)
		close(done)
	}()

	select {
	case <-done:
		log.Debugf("FIDO2: device %v: selected with err=%v", dev.info.path, err)
	case <-ctx.Done():
		log.Debugf("FIDO2: device %v: requesting cancel", dev.info.path)
		if err := dev.Cancel(); err != nil {
			log.Debugf("FIDO2: device %v: cancel errored: %v", dev.info.path, err)
		}

		// Give the device a grace period to cancel/cleanup, but do not wait
		// forever.
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			log.Warnf("FIDO2: " +
				"Timed out waiting for device cancels. " +
				"It's possible some devices are left blinking.")
		}

		return false, trace.Wrap(ctx.Err())
	}

	// Returns variables captured by goroutine.
	return
}

// deviceInfo contains an aggregate of a device's information and capabilities.
// Various fields match options under
// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorGetInfo.
type deviceInfo struct {
	path                           string
	u2f                            bool
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

func makeDevInfo(path string, info *libfido2.DeviceInfo, u2f bool) *deviceInfo {
	di := &deviceInfo{
		path: path,
		u2f:  u2f,
	}

	// U2F devices don't respond to dev.Info().
	if u2f {
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
