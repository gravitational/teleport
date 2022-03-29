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

	// Credentials mirrors libfido2.Device.Credentials.
	Credentials(rpID string, pin string) ([]*libfido2.Credential, error)

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
		opts *libfido2.AssertionOpts) (*libfido2.Assertion, error)
}

// fidoDeviceLocations and fidoNewDevice are used to allow testing.
var fidoDeviceLocations = libfido2.DeviceLocations
var fidoNewDevice = func(path string) (FIDODevice, error) {
	return libfido2.NewDevice(path)
}

// IsFIDO2Available returns true if libfido2 is available in the current build.
func IsFIDO2Available() bool {
	return true
}

// fido2Login implements FIDO2Login.
func fido2Login(
	ctx context.Context,
	origin, user string, assertion *wanlib.CredentialAssertion, prompt LoginPrompt,
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

	allowedCreds := assertion.Response.GetAllowedCredentialIDs()
	uv := assertion.Response.UserVerification == protocol.VerificationRequired
	passwordless := len(allowedCreds) == 0 && uv

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
	var credentialID []byte
	var userID []byte
	var username string
	var usedAppID bool

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
		const pin = "" // not required to filter
		if _, err := dev.Assertion(rpID, ccdHash[:], allowedCreds, pin, &libfido2.AssertionOpts{
			UP: libfido2.False,
		}); err == nil {
			return true, nil
		}

		// Try again with the App ID, if present.
		if appID == "" {
			return false, nil
		}
		if _, err := dev.Assertion(appID, ccdHash[:], allowedCreds, pin, &libfido2.AssertionOpts{
			UP: libfido2.False,
		}); err != nil {
			log.Debugf("FIDO2: Device %v: filtered due to lack of allowed credential", info.path)
			return false, nil
		}
		return true, nil
	}

	deviceCallback := func(dev FIDODevice, info *deviceInfo, pin string) error {
		var actualRPID string
		var cID []byte
		var uID []byte
		var uName string
		if passwordless {
			cred, err := getPasswordlessCredentials(dev, pin, rpID, user)
			if err != nil {
				return trace.Wrap(err)
			}
			actualRPID = rpID
			cID = cred.ID
			uID = cred.User.ID
			uName = cred.User.Name
		} else {
			// TODO(codingllama): Ideally we'd rely on fido_assert_id_ptr/_len.
			var err error
			actualRPID, cID, err = getMFACredentials(dev, pin, rpID, appID, allowedCreds)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if passwordless {
			// Ask for another touch before the assertion, we used the first touch
			// in the Credentials() call.
			prompt.PromptTouch()
		}

		opts := &libfido2.AssertionOpts{
			UP: libfido2.True,
		}
		if uv {
			opts.UV = libfido2.True
		}
		resp, err := dev.Assertion(actualRPID, ccdHash[:], [][]byte{cID}, pin, opts)
		if err != nil {
			return trace.Wrap(err)
		}

		// Use the first successful assertion.
		// In practice it is very unlikely we'd hit this twice.
		mu.Lock()
		if assertionResp == nil {
			assertionResp = resp
			credentialID = cID
			userID = uID
			username = uName
			usedAppID = actualRPID != rpID
		}
		mu.Unlock()
		return nil
	}

	if err := runOnFIDO2Devices(ctx, prompt, passwordless, filter, deviceCallback); err != nil {
		return nil, "", trace.Wrap(err)
	}

	var rawAuthData []byte
	if err := cbor.Unmarshal(assertionResp.AuthDataCBOR, &rawAuthData); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: &wanpb.CredentialAssertionResponse{
				Type:  string(protocol.PublicKeyCredentialType),
				RawId: credentialID,
				Response: &wanpb.AuthenticatorAssertionResponse{
					ClientDataJson:    ccdJSON,
					AuthenticatorData: rawAuthData,
					Signature:         assertionResp.Sig,
					UserHandle:        userID,
				},
				Extensions: &wanpb.AuthenticationExtensionsClientOutputs{
					AppId: usedAppID,
				},
			},
		},
	}, username, nil
}

func getPasswordlessCredentials(dev FIDODevice, pin, rpID, user string) (*libfido2.Credential, error) {
	creds, err := dev.Credentials(rpID, pin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): After this line we should cancel other devices,
	//  the user picked the current one.

	if user != "" {
		log.Debugf("FIDO2: Searching credentials for user %q", user)
	}

	switch {
	case len(creds) == 0:
		return nil, libfido2.ErrNoCredentials
	case len(creds) == 1 && user == "": // no need to disambiguate
		cred := creds[0]
		log.Debugf("FIDO2: Found resident credential for user %q", cred.User.Name)
		return cred, nil
	case len(creds) > 1 && user == "": // can't disambiguate
		return nil, trace.BadParameter("too many credentials found, explicit user required")
	}

	duplicateWarning := false
	var res *libfido2.Credential
	for _, cred := range creds {
		if cred.User.Name == user {
			// Print information about matched credential, useful for debugging.
			// ykman prints user IDs in hex, hence the unusual encoding choice below.
			cID := base64.RawURLEncoding.EncodeToString(cred.ID)
			uID := cred.User.ID
			log.Debugf("FIDO2: Found resident credential for user %v, credential ID (b64) = %v, user ID (hex) = %x", user, cID, uID)
			if res == nil {
				res = cred
				continue // Don't break, we want to warn about duplicates.
			}
			if !duplicateWarning {
				duplicateWarning = true
				log.Warnf("Found multiple credentials for %q, using first match", user)
			}
		}
	}
	if res == nil {
		return nil, trace.BadParameter("no credentials for user %q", user)
	}
	return res, nil
}

func getMFACredentials(dev FIDODevice, pin, rpID, appID string, allowedCreds [][]byte) (string, []byte, error) {
	// The actual hash is not necessary here.
	const cdh = "00000000000000000000000000000000"

	opts := &libfido2.AssertionOpts{
		UP: libfido2.False,
	}
	actualRPID := rpID
	var cID []byte
	for _, cred := range allowedCreds {
		_, err := dev.Assertion(rpID, []byte(cdh), [][]byte{cred}, pin, opts)
		if err == nil {
			cID = cred
			break
		}

		// Try again with the U2F appID, if present.
		if appID != "" {
			_, err = dev.Assertion(appID, []byte(cdh), [][]byte{cred}, pin, opts)
			if err == nil {
				actualRPID = appID
				cID = cred
				break
			}
		}
	}
	if len(cID) == 0 {
		return "", nil, libfido2.ErrNoCredentials
	}

	return actualRPID, cID, nil
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

	const passwordless = false
	if err := runOnFIDO2Devices(ctx, prompt, passwordless, filter, deviceCallback); err != nil {
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
type runPrompt LoginPrompt

func runOnFIDO2Devices(
	ctx context.Context,
	prompt runPrompt, passwordless bool,
	filter deviceFilterFunc,
	deviceCallback deviceCallbackFunc) error {
	devices, err := findSuitableDevicesOrTimeout(ctx, filter)
	if err != nil {
		return trace.Wrap(err)
	}

	var dev deviceWithInfo
	if shouldDoEagerPINPrompt(passwordless, devices) {
		dev = devices[0] // single device guaranteed in this case
	} else {
		prompt.PromptTouch() // about to select

		d, requiresPIN, err := selectDevice(ctx, "" /* pin */, devices, deviceCallback)
		switch {
		case err != nil:
			return trace.Wrap(err)
		case !requiresPIN:
			return nil
		}
		dev = d
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

	// Prompt again for a touch if MFA, but don't prompt for passwordless.
	// The passwordless callback calls the prompt at a more appropriate time.
	if !passwordless {
		prompt.PromptTouch()
	}

	// Run the callback again with the informed PIN.
	// selectDevice is used since it correctly deals with cancellation.
	_, _, err = selectDevice(ctx, pin, []deviceWithInfo{dev}, deviceCallback)
	return trace.Wrap(err)
}

func shouldDoEagerPINPrompt(passwordless bool, devices []deviceWithInfo) bool {
	// Don't eagerly prompt for PIN if MFA, it usually doesn't require PINs.
	// Also don't eagerly prompt if >1 device, the touch chooses the device and we
	// can't know which device will be chosen.
	if !passwordless || len(devices) > 1 {
		return false
	}

	// Only eagerly prompt for PINs if not bio, biometric devices unlock with
	// touch instead (explicit PIN not required).
	info := devices[0].info
	return info.clientPinSet && !info.bioEnroll
}

func findSuitableDevicesOrTimeout(ctx context.Context, filter deviceFilterFunc) ([]deviceWithInfo, error) {
	ticker := time.NewTicker(FIDO2PollInterval)
	defer ticker.Stop()

	knownPaths := make(map[string]struct{})
	for {
		devices, err := findSuitableDevices(filter, knownPaths)
		if err == nil {
			return devices, nil
		}
		log.WithError(err).Debug("FIDO2: Selecting devices")

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
		return nil, errors.New("no suitable devices found")
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
		if err = deviceCallback(dev, info, pin); !errors.Is(err, libfido2.ErrPinRequired) {
			return
		}

		// ErrPinRequired means we can't use "deviceCallback" as the selection
		// mechanism. Let's run a different operation to ask for a touch.
		requiresPIN = true

		// TODO(codingllama): What we really want here is fido_dev_get_touch_begin.
		//  Another option is to put the authenticator into U2F mode.
		const rpID = "7f364cc0-958c-4177-b3ea-b2d8d7f15d4a" // arbitrary, unlikely to collide with a real RP
		const cdh = "00000000000000000000000000000000"      // "random", size 32
		_, err = dev.Assertion(rpID, []byte(cdh), nil /* credentials */, pin, &libfido2.AssertionOpts{
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

// deviceInfo contains an aggregate of a device's informations and capabilities.
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
