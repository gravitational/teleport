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

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/fxamacker/cbor/v2"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
	"github.com/keys-pub/go-libfido2"

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
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
var fidoDeviceLocations = libfido2.DeviceLocations
var fidoNewDevice = func(path string) (FIDODevice, error) {
	return libfido2.NewDevice(path)
}

// IsFIDO2Available returns true if libfido2 is available in the current build.
func IsFIDO2Available() bool {
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
	case assertion == nil:
		return nil, "", trace.BadParameter("assertion required")
	case prompt == nil:
		return nil, "", trace.BadParameter("prompt required")
	case len(assertion.Response.Challenge) == 0:
		return nil, "", trace.BadParameter("assertion challenge required")
	case assertion.Response.RelyingPartyID == "":
		return nil, "", trace.BadParameter("assertion relying party ID required")
	}
	if opts == nil {
		opts = &LoginOpts{}
	}

	allowedCreds := assertion.Response.GetAllowedCredentialIDs()
	uv := assertion.Response.UserVerification == protocol.VerificationRequired

	// Presence of any allowed credential is interpreted as the user identity
	// being partially established, aka non-passwordless.
	passwordless := len(allowedCreds) == 0

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
	filter := func(dev FIDODevice, info *deviceInfo) (bool, error) {
		switch {
		case uv && !info.uvCapable():
			log.Debugf("FIDO2: Device %v: filtered due to lack of UV", info.path)
			return false, nil
		case passwordless && !info.rk:
			log.Debugf("FIDO2: Device %v: filtered due to lack of RK", info.path)
			return false, nil
		case len(allowedCreds) == 0: // Nothing else to check
			return true, nil
		}

		// Does the device have a suitable credential?
		const pin = ""
		actualRPID, err := discoverRPID(dev, pin, rpID, appID, allowedCreds)
		if err != nil {
			log.Debugf("FIDO2: Device %v: filtered due to lack of allowed credential", info.path)
			return false, nil
		}
		pathToRPID.Store(info.path, actualRPID)

		return true, nil
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

func discoverRPID(dev FIDODevice, pin, rpID, appID string, allowedCreds [][]byte) (string, error) {
	// The actual hash is not necessary here.
	const cdh = "00000000000000000000000000000000"

	opts := &libfido2.AssertionOpts{
		UP: libfido2.False,
	}
	for _, id := range []string{rpID, appID} {
		if id == "" {
			continue
		}
		if _, err := dev.Assertion(id, []byte(cdh), allowedCreds, pin, opts); err == nil {
			return id, nil
		}
	}
	return "", libfido2.ErrNoCredentials
}

func pickAssertion(
	assertions []*libfido2.Assertion, prompt LoginPrompt, user string, passwordless bool) (*libfido2.Assertion, error) {
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
	case cc == nil:
		return nil, trace.BadParameter("credential creation required")
	case prompt == nil:
		return nil, trace.BadParameter("prompt required")
	case len(cc.Response.Challenge) == 0:
		return nil, trace.BadParameter("credential creation challenge required")
	case cc.Response.RelyingParty.ID == "":
		return nil, trace.BadParameter("credential creation relying party ID required")
	}

	rrk := cc.Response.AuthenticatorSelection.RequireResidentKey != nil && *cc.Response.AuthenticatorSelection.RequireResidentKey
	if rrk {
		// Be more pedantic with resident keys, some of this info gets recorded with
		// the credential.
		switch {
		case len(cc.Response.RelyingParty.Name) == 0:
			return nil, trace.BadParameter("relying party name required for resident credential")
		case len(cc.Response.User.Name) == 0:
			return nil, trace.BadParameter("user name required for resident credential")
		case len(cc.Response.User.DisplayName) == 0:
			return nil, trace.BadParameter("user display name required for resident credential")
		case len(cc.Response.User.ID) == 0:
			return nil, trace.BadParameter("user ID required for resident credential")
		}
	}

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

	filter := func(dev FIDODevice, info *deviceInfo) (bool, error) {
		switch {
		case (plat && !info.plat) || (rrk && !info.rk) || (uv && !info.uvCapable()):
			log.Debugf("FIDO2: Device %v: filtered due to options", info.path)
			return false, nil
		case len(excludeList) == 0:
			return true, nil
		}

		// Does the device hold an excluded credential?
		const pin = "" // not required to filter
		switch _, err := dev.Assertion(rp.ID, ccdHash[:], excludeList, pin, &libfido2.AssertionOpts{
			UP: libfido2.False,
		}); {
		case errors.Is(err, libfido2.ErrNoCredentials):
			return true, nil
		case err == nil:
			log.Debugf("FIDO2: Device %v: filtered due to presence of excluded credential", info.path)
			return false, nil
		default: // unexpected error
			return false, trace.Wrap(err)
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

type deviceFilterFunc func(dev FIDODevice, info *deviceInfo) (ok bool, err error)
type deviceCallbackFunc func(dev FIDODevice, info *deviceInfo, pin string) error

// runPrompt defines the prompt operations necessary for runOnFIDO2Devices.
// (RegisterPrompt happens to match the minimal interface required.)
type runPrompt RegisterPrompt

// errNoSuitableDevices is used internally to loop over findSuitableDevices.
var errNoSuitableDevices = errors.New("no suitable devices found")

func runOnFIDO2Devices(
	ctx context.Context,
	prompt runPrompt,
	filter deviceFilterFunc,
	deviceCallback deviceCallbackFunc) error {
	// Do we have readily available devices?
	knownPaths := make(map[string]struct{}) // filled by findSuitableDevices*
	prompted := false
	devices, err := findSuitableDevices(filter, knownPaths)
	if errors.Is(err, errNoSuitableDevices) {
		// No readily available devices means we need to prompt, otherwise the
		// user gets no feedback whatsoever.
		prompt.PromptTouch()
		prompted = true

		devices, err = findSuitableDevicesOrTimeout(ctx, filter, knownPaths)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	if !prompted {
		prompt.PromptTouch() // about to select
	}
	dev, requiresPIN, err := selectDevice(ctx, "" /* pin */, devices, deviceCallback)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case !requiresPIN:
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
	prompt.PromptTouch()

	// Run the callback again with the informed PIN.
	// selectDevice is used since it correctly deals with cancellation.
	_, _, err = selectDevice(ctx, pin, []deviceWithInfo{dev}, deviceCallback)
	return trace.Wrap(err)
}

func findSuitableDevicesOrTimeout(
	ctx context.Context, filter deviceFilterFunc, knownPaths map[string]struct{}) ([]deviceWithInfo, error) {
	ticker := time.NewTicker(FIDO2PollInterval)
	defer ticker.Stop()

	for {
		switch devices, err := findSuitableDevices(filter, knownPaths); {
		case err == nil:
			return devices, nil
		case errors.Is(err, errNoSuitableDevices):
			// OK, carry on until we find a device or timeout.
		default:
			// Unexpected, abort.
			return nil, trace.Wrap(err)
		}

		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-ticker.C:
		}
	}
}

func findSuitableDevices(filter deviceFilterFunc, knownPaths map[string]struct{}) ([]deviceWithInfo, error) {
	locs, err := fidoDeviceLocations()
	if err != nil {
		return nil, trace.Wrap(err, "device locations")
	}

	var devs []deviceWithInfo
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
		const infoAttempts = 3
		for i := 0; i < infoAttempts; i++ {
			info, err = dev.Info()
			switch {
			case errors.Is(err, libfido2.ErrNotFIDO2):
				// Use an empty info and carry on.
				// A FIDO/U2F device has no capabilities beyond MFA
				// registrations/assertions.
				info = &libfido2.DeviceInfo{}
			case errors.Is(err, libfido2.ErrTX):
				// Happens occasionally, give the device a short grace period and retry.
				time.Sleep(1 * time.Millisecond)
				continue
			case err != nil: // unexpected error
				return nil, trace.Wrap(err, "device %v: info", path)
			}
			break // err == nil
		}
		if info == nil {
			return nil, trace.Wrap(libfido2.ErrTX, "device %v: max info attempts reached", path)
		}
		log.Debugf("FIDO2: Info for device %v: %#v", path, info)

		di := makeDevInfo(path, info)
		switch ok, err := filter(dev, di); {
		case err != nil:
			return nil, trace.Wrap(err, "device %v: filter", path)
		case !ok:
			continue // Skip device.
		}
		devs = append(devs, deviceWithInfo{FIDODevice: dev, info: di})
	}

	l := len(devs)
	if l == 0 {
		return nil, errNoSuitableDevices
	}
	log.Debugf("FIDO2: Found %v suitable devices", l)

	return devs, nil
}

func selectDevice(
	ctx context.Context,
	pin string, devices []deviceWithInfo, deviceCallback deviceCallbackFunc) (deviceWithInfo, bool, error) {
	callbackWrapper := func(dev FIDODevice, info *deviceInfo, pin string) (requiresPIN bool, err error) {
		// Attempt to select a device by running "deviceCallback" on it.
		// For most scenarios this works, saving a touch.
		err = deviceCallback(dev, info, pin)
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

		// TODO(codingllama): What we really want here is fido_dev_get_touch_begin.
		//  Another option is to put the authenticator into U2F mode.
		const rpID = "7f364cc0-958c-4177-b3ea-b2d8d7f15d4a" // arbitrary, unlikely to collide with a real RP
		const cdh = "00000000000000000000000000000000"      // "random", size 32
		_, err = dev.Assertion(rpID, []byte(cdh), nil /* credentials */, "", &libfido2.AssertionOpts{
			UP: libfido2.True,
		})
		if errors.Is(err, libfido2.ErrNoCredentials) {
			err = nil // OK, selected successfully
		}
		return
	}

	type selectResp struct {
		dev         deviceWithInfo
		requiresPIN bool
		err         error
	}

	respC := make(chan selectResp)
	numGoroutines := len(devices)
	for _, dev := range devices {
		dev := dev
		go func() {
			requiresPIN, err := callbackWrapper(dev, dev.info, pin)
			respC <- selectResp{
				dev:         dev,
				requiresPIN: requiresPIN,
				err:         err,
			}
		}()
	}

	// Stop on timeout or first interaction, whatever comes first wins and gets
	// returned.
	var resp selectResp
	select {
	case <-ctx.Done():
		resp.err = ctx.Err()
	case resp = <-respC:
		numGoroutines--
	}

	// Cancel ongoing operations and wait for goroutines to complete.
	for _, dev := range devices {
		if dev == resp.dev {
			continue
		}
		if err := dev.Cancel(); err != nil {
			log.WithError(err).Tracef("FIDO2: Device cancel")
		}
	}
	for i := 0; i < numGoroutines; i++ {
		cancelResp := <-respC
		if err := cancelResp.err; err != nil && !errors.Is(err, libfido2.ErrKeepaliveCancel) {
			log.WithError(err).Debugf("FIDO2: Device errored on cancel")
		}
	}

	return resp.dev, resp.requiresPIN, trace.Wrap(resp.err)
}

// deviceInfo contains an aggregate of a device's information and capabilities.
// Various fields match options under
// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorGetInfo.
type deviceInfo struct {
	path                           string
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

func makeDevInfo(path string, info *libfido2.DeviceInfo) *deviceInfo {
	di := &deviceInfo{path: path}
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
