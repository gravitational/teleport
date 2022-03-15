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

package webauthncli_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/fxamacker/cbor/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/keys-pub/go-libfido2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

var makeCredentialAuthDataRaw, makeCredentialAuthDataCBOR, makeCredentialSig []byte
var assertionAuthDataRaw, assertionAuthDataCBOR, assertionSig []byte

func init() {
	// Initialize arrays with random data, but use realistic sizes.
	// YMMV.
	makeCredentialAuthDataRaw = make([]byte, 37)
	makeCredentialSig = make([]byte, 70)
	assertionAuthDataRaw = make([]byte, 37)
	assertionSig = make([]byte, 70)
	for _, b := range [][]byte{
		makeCredentialAuthDataRaw,
		makeCredentialSig,
		assertionAuthDataRaw,
		assertionSig,
	} {
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
	}

	// Returned authData is CBOR-encoded, so let's do that.
	pairs := []*[]byte{
		&makeCredentialAuthDataRaw, &makeCredentialAuthDataCBOR,
		&assertionAuthDataRaw, &assertionAuthDataCBOR,
	}
	for i := 0; i < len(pairs); i += 2 {
		dataRaw := pairs[i]
		dataCBOR := pairs[i+1]

		res, err := cbor.Marshal(*dataRaw)
		if err != nil {
			panic(err)
		}
		*dataCBOR = res
	}
}

type noopPrompt struct{}

func (p noopPrompt) PromptPIN() (string, error) {
	return "", nil
}

func (p noopPrompt) PromptAdditionalTouch() error {
	return nil
}

func TestFIDO2Login(t *testing.T) {
	resetFIDO2AfterTests(t)
	wancli.FIDO2PollInterval = 1 * time.Millisecond // run fast on tests

	const rpID = "example.com"
	const appID = "https://example.com"
	const origin = "https://example.com"

	authOpts := []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "clientPin", Value: "false"}, // supported but unset
	}
	pinOpts := []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "clientPin", Value: "true"}, // supported and configured
	}
	bioOpts := []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "uv", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "alwaysUv", Value: "true"},
		{Name: "bioEnroll", Value: "true"}, // supported and configured
		{Name: "clientPin", Value: "true"}, // supported and configured
	}

	// User IDs and names for resident credentials / passwordless.
	const llamaName = "llama"
	const alpacaName = "alpaca"
	var llamaID = make([]byte, 16)
	var alpacaID = make([]byte, 16)
	for _, b := range [][]byte{llamaID, alpacaID} {
		_, err := rand.Read(b)
		require.NoError(t, err, "Read failed")
	}

	// auth1 is a FIDO2 authenticator without a PIN configured.
	auth1 := mustNewFIDO2Device("/path1", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})
	// pin1 is a FIDO2 authenticator with a PIN.
	pin1 := mustNewFIDO2Device("/pin1", "supersecretpinllama", &libfido2.DeviceInfo{
		Options: pinOpts,
	})
	// pin2 is a FIDO2 authenticator with a PIN.
	pin2 := mustNewFIDO2Device("/pin2", "supersecretpin2", &libfido2.DeviceInfo{
		Options: pinOpts,
	})
	// pin3 is a FIDO2 authenticator with a PIN and resident credentials.
	pin3 := mustNewFIDO2Device("/pin3", "supersecretpin3", &libfido2.DeviceInfo{
		Options: pinOpts,
	}, &libfido2.Credential{
		User: libfido2.User{
			ID:   alpacaID,
			Name: alpacaName,
		},
	})
	// bio1 is a biometric authenticator.
	bio1 := mustNewFIDO2Device("/bio1", "supersecretBIOpin", &libfido2.DeviceInfo{
		Options: bioOpts,
	})
	// bio2 is a biometric authenticator with configured resident credentials.
	bio2 := mustNewFIDO2Device("/bio2", "supersecretBIO2pin", &libfido2.DeviceInfo{
		Options: bioOpts,
	}, &libfido2.Credential{
		User: libfido2.User{
			ID:   llamaID,
			Name: llamaName,
		},
	}, &libfido2.Credential{
		User: libfido2.User{
			ID:   alpacaID,
			Name: alpacaName,
		},
	})
	// legacy1 is an authenticator registered using the U2F App ID.
	legacy1 := mustNewFIDO2Device("/legacy1", "" /* pin */, &libfido2.DeviceInfo{Options: authOpts})
	legacy1.wantRPID = appID

	challenge, err := protocol.CreateChallenge()
	require.NoError(t, err, "CreateChallenge failed")

	baseAssertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:          challenge,
			RelyingPartyID:     rpID,
			AllowedCredentials: []protocol.CredentialDescriptor{},
			UserVerification:   protocol.VerificationDiscouraged,
			Extensions:         map[string]interface{}{},
		},
	}

	tests := []struct {
		name            string
		timeout         time.Duration
		fido2           *fakeFIDO2
		setUP           func()
		user            string
		createAssertion func() *wanlib.CredentialAssertion
		prompt          wancli.LoginPrompt
		// assertResponse and wantErr are mutually exclusive.
		assertResponse func(t *testing.T, resp *wanpb.CredentialAssertionResponse)
		wantErr        string
	}{
		{
			name:  "single device",
			fido2: newFakeFIDO2(auth1),
			setUP: func() {
				go func() {
					// Simulate delayed user press.
					time.Sleep(100 * time.Millisecond)
					auth1.setUP()
				}()
			},
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
				}
				return &cp
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, auth1.credentialID(), resp.RawId, "RawId mismatch")
			},
		},
		{
			name:  "pin protected device",
			fido2: newFakeFIDO2(pin1),
			setUP: pin1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: pin1.credentialID()},
				}
				return &cp
			},
		},
		{
			name:  "biometric device",
			fido2: newFakeFIDO2(bio1),
			setUP: bio1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: bio1.credentialID()},
				}
				return &cp
			},
		},
		{
			name:  "legacy device (AppID)",
			fido2: newFakeFIDO2(legacy1),
			setUP: legacy1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = protocol.AuthenticationExtensions{
					wanlib.AppIDExtension: appID,
				}
				return &cp
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.True(t, resp.Extensions.AppId, "AppID mismatch")
			},
		},
		{
			name: "multiple valid devices",
			fido2: newFakeFIDO2(
				auth1,
				pin1,
				bio1,
				legacy1,
			),
			setUP: bio1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: pin1.credentialID()},
					{CredentialID: bio1.credentialID()},
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = protocol.AuthenticationExtensions{
					wanlib.AppIDExtension: appID,
				}
				return &cp
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio1.credentialID(), resp.RawId, "RawId mismatch (want bio1)")
			},
		},
		{
			name: "multiple devices filtered",
			fido2: newFakeFIDO2(
				auth1, // allowed
				pin1,  // not allowed
				bio1,
				legacy1, // doesn't match RPID or AppID
			),
			setUP: auth1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: bio1.credentialID()},
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = protocol.AuthenticationExtensions{
					wanlib.AppIDExtension: "https://badexample.com",
				}
				return &cp
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, auth1.credentialID(), resp.RawId, "RawId mismatch (want auth1)")
			},
		},
		{
			name: "multiple pin devices",
			fido2: newFakeFIDO2(
				auth1,
				pin1, pin2,
				bio1,
			),
			setUP: pin2.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: pin1.credentialID()},
					{CredentialID: pin2.credentialID()},
					{CredentialID: bio1.credentialID()},
				}
				return &cp
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, pin2.credentialID(), resp.RawId, "RawId mismatch (want pin2)")
			},
		},
		{
			name:    "NOK no devices plugged times out",
			timeout: 10 * time.Millisecond,
			fido2:   newFakeFIDO2(),
			setUP:   func() {},
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
				}
				return &cp
			},
			wantErr: context.DeadlineExceeded.Error(),
		},
		{
			name:    "NOK no devices touched times out",
			timeout: 10 * time.Millisecond,
			fido2:   newFakeFIDO2(auth1, pin1, bio1, legacy1),
			setUP:   func() {}, // no interaction
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []protocol.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: pin1.credentialID()},
					{CredentialID: bio1.credentialID()},
				}
				return &cp
			},
			wantErr: context.DeadlineExceeded.Error(),
		},
		{
			name:  "passwordless pin",
			fido2: newFakeFIDO2(pin3),
			setUP: pin3.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: pin3,
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, pin3.credentials[0].ID, resp.RawId, "RawId mismatch (want %q resident credential)", alpacaName)
				assert.Equal(t, alpacaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", alpacaName)
			},
		},
		{
			name:  "passwordless biometric (llama)",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			user:  llamaName,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2,
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio2.credentials[0].ID, resp.RawId, "RawId mismatch (want %q resident credential)", llamaName)
				assert.Equal(t, llamaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", llamaName)
			},
		},
		{
			name:  "passwordless biometric (alpaca)",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			user:  alpacaName,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2,
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio2.credentials[1].ID, resp.RawId, "RawId mismatch (want %q resident credential)", alpacaName)
				assert.Equal(t, alpacaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", alpacaName)
			},
		},
		{
			name:  "NOK passwordless no credentials",
			fido2: newFakeFIDO2(bio1),
			setUP: bio1.setUP,
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt:  bio1,
			wantErr: libfido2.ErrNoCredentials.Error(),
		},
		{
			name:  "NOK passwordless ambiguous user",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			user:  "", // >1 resident credential, can't pick unambiguous username.
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt:  bio2,
			wantErr: "explicit user required",
		},
		{
			name:  "NOK passwordless unknown user",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			user:  "camel", // unknown
			createAssertion: func() *wanlib.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt:  bio2,
			wantErr: "no credentials for user",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fido2.setCallbacks()
			test.setUP()

			timeout := test.timeout
			if timeout == 0 {
				timeout = 1 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			prompt := test.prompt
			if prompt == nil {
				prompt = noopPrompt{}
			}

			mfaResp, _, err := wancli.FIDO2Login(ctx, origin, test.user, test.createAssertion(), prompt)
			switch {
			case test.wantErr != "" && err == nil:
				t.Fatalf("FIDO2Login returned err = nil, wantErr %q", test.wantErr)
			case test.wantErr != "":
				require.Contains(t, err.Error(), test.wantErr, "FIDO2Login returned err = %q, wantErr %q", err, test.wantErr)
				return
			default:
				require.NoError(t, err, "FIDO2Login failed")
				require.NotNil(t, mfaResp, "mfaResp nil")
			}

			// Do a few baseline checks, tests can assert further.
			got := mfaResp.GetWebauthn()
			require.NotNil(t, got, "assertion response nil")
			require.NotNil(t, got.Response, "authenticator response nil")
			assert.NotNil(t, got.Response.ClientDataJson, "ClientDataJSON nil")
			want := &wanpb.CredentialAssertionResponse{
				Type:  string(protocol.PublicKeyCredentialType),
				RawId: got.RawId,
				Response: &wanpb.AuthenticatorAssertionResponse{
					ClientDataJson:    got.Response.ClientDataJson,
					AuthenticatorData: assertionAuthDataRaw,
					Signature:         assertionSig,
					UserHandle:        got.Response.UserHandle,
				},
				Extensions: got.Extensions,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("FIDO2Login()/CredentialAssertionResponse mismatch (-want +got):\n%v", diff)
			}

			if test.assertResponse != nil {
				test.assertResponse(t, got)
			}
		})
	}
}

func TestFIDO2Login_errors(t *testing.T) {
	resetFIDO2AfterTests(t)

	// Make sure we won't call the real libfido2.
	f2 := newFakeFIDO2()
	f2.setCallbacks()

	const origin = "https://example.com"
	const user = ""
	okAssertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []protocol.CredentialDescriptor{
				{Type: protocol.PublicKeyCredentialType, CredentialID: []byte{1, 2, 3, 4, 5}},
			},
		},
	}
	var prompt noopPrompt

	nilChallengeAssertion := *okAssertion
	nilChallengeAssertion.Response.Challenge = nil

	emptyRPIDAssertion := *okAssertion
	emptyRPIDAssertion.Response.RelyingPartyID = ""

	tests := []struct {
		name      string
		origin    string
		assertion *wanlib.CredentialAssertion
		prompt    wancli.LoginPrompt
		wantErr   string
	}{
		{
			name:      "ok - timeout", // check that good params are good
			origin:    origin,
			assertion: okAssertion,
			prompt:    prompt,
			wantErr:   context.DeadlineExceeded.Error(),
		},
		{
			name:      "nil origin",
			assertion: okAssertion,
			prompt:    prompt,
			wantErr:   "origin",
		},
		{
			name:    "nil assertion",
			origin:  origin,
			prompt:  prompt,
			wantErr: "assertion required",
		},
		{
			name:      "assertion without challenge",
			origin:    origin,
			assertion: &nilChallengeAssertion,
			prompt:    prompt,
			wantErr:   "challenge",
		},
		{
			name:      "assertion without RPID",
			origin:    origin,
			assertion: &emptyRPIDAssertion,
			prompt:    prompt,
			wantErr:   "relying party ID",
		},
		{
			name:      "nil prompt",
			origin:    origin,
			assertion: okAssertion,
			wantErr:   "prompt",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, _, err := wancli.FIDO2Login(ctx, test.origin, user, test.assertion, test.prompt)
			require.Error(t, err, "FIDO2Login returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "FIDO2Login returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func resetFIDO2AfterTests(t *testing.T) {
	pollInterval := wancli.FIDO2PollInterval
	devLocations := wancli.FIDODeviceLocations
	newDevice := wancli.FIDONewDevice
	t.Cleanup(func() {
		wancli.FIDO2PollInterval = pollInterval
		wancli.FIDODeviceLocations = devLocations
		wancli.FIDONewDevice = newDevice
	})
}

type fakeFIDO2 struct {
	locs    []*libfido2.DeviceLocation
	devices map[string]*fakeFIDO2Device
}

func newFakeFIDO2(devs ...*fakeFIDO2Device) *fakeFIDO2 {
	f := &fakeFIDO2{
		devices: make(map[string]*fakeFIDO2Device),
	}
	for _, dev := range devs {
		if _, ok := f.devices[dev.path]; ok {
			panic(fmt.Sprintf("Duplicate device path registered: %q", dev.path))
		}
		f.locs = append(f.locs, &libfido2.DeviceLocation{
			Path: dev.path,
		})
		f.devices[dev.path] = dev
	}
	return f
}

func (f *fakeFIDO2) setCallbacks() {
	*wancli.FIDODeviceLocations = f.newMeteredDeviceLocations()
	*wancli.FIDONewDevice = f.NewDevice
}

func (f *fakeFIDO2) newMeteredDeviceLocations() func() ([]*libfido2.DeviceLocation, error) {
	i := 0
	return func() ([]*libfido2.DeviceLocation, error) {
		// Delay showing devices for a while to exercise polling.
		i++
		const minLoops = 2
		if i < minLoops {
			return nil, nil
		}
		return f.locs, nil
	}
}

func (f *fakeFIDO2) NewDevice(path string) (wancli.FIDODevice, error) {
	if dev, ok := f.devices[path]; ok {
		return dev, nil
	}
	// go-libfido2 doesn't actually error here, but we do for simplicity.
	return nil, errors.New("not found")
}

type fakeFIDO2Device struct {
	path        string
	info        *libfido2.DeviceInfo
	pin         string
	credentials []*libfido2.Credential

	// wantRPID may be set directly to enable RPID checks on Assertion.
	wantRPID string
	// format may be set directly to change the attestation format.
	format string

	key    *mocku2f.Key
	pubKey []byte

	// mu and cond guard up and cancel.
	mu         sync.Mutex
	cond       *sync.Cond
	up, cancel bool
}

func mustNewFIDO2Device(path, pin string, info *libfido2.DeviceInfo, creds ...*libfido2.Credential) *fakeFIDO2Device {
	dev, err := newFIDO2Device(path, pin, info, creds...)
	if err != nil {
		panic(err)
	}
	return dev
}

func newFIDO2Device(path, pin string, info *libfido2.DeviceInfo, creds ...*libfido2.Credential) (*fakeFIDO2Device, error) {
	key, err := mocku2f.Create()
	if err != nil {
		return nil, err
	}

	pubKeyCBOR, err := wanlib.U2FKeyToCBOR(&key.PrivateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	for _, cred := range creds {
		cred.ID = make([]byte, 16) // somewhat arbitrary
		if _, err := rand.Read(cred.ID); err != nil {
			return nil, err
		}
		cred.Type = libfido2.ES256
	}

	d := &fakeFIDO2Device{
		path:        path,
		pin:         pin,
		credentials: creds,
		format:      "packed",
		info:        info,
		key:         key,
		pubKey:      pubKeyCBOR,
	}
	d.cond = sync.NewCond(&d.mu)
	return d, nil
}

func (f *fakeFIDO2Device) PromptPIN() (string, error) {
	return f.pin, nil
}

func (f *fakeFIDO2Device) PromptAdditionalTouch() error {
	f.setUP()
	return nil
}

func (f *fakeFIDO2Device) credentialID() []byte {
	return f.key.KeyHandle
}

func (f *fakeFIDO2Device) cert() []byte {
	return f.key.Cert
}

func (f *fakeFIDO2Device) Info() (*libfido2.DeviceInfo, error) {
	return f.info, nil
}

func (f *fakeFIDO2Device) setUP() {
	f.mu.Lock()
	f.up = true
	f.mu.Unlock()
	f.cond.Broadcast()
}

func (f *fakeFIDO2Device) Cancel() error {
	f.mu.Lock()
	f.cancel = true
	f.mu.Unlock()
	f.cond.Broadcast()
	return nil
}

func (f *fakeFIDO2Device) Credentials(rpID string, pin string) ([]*libfido2.Credential, error) {
	if f.pin != "" {
		if err := f.validatePIN(pin); err != nil {
			return nil, err
		}
	}
	return f.credentials, nil
}

func (f *fakeFIDO2Device) Assertion(
	rpID string,
	clientDataHash []byte,
	credentialIDs [][]byte,
	pin string,
	opts *libfido2.AssertionOpts,
) (*libfido2.Assertion, error) {
	switch {
	case rpID == "":
		return nil, errors.New("rp.ID required")
	case f.wantRPID != "" && f.wantRPID != rpID:
		return nil, libfido2.ErrNoCredentials
	case len(clientDataHash) == 0:
		return nil, errors.New("clientDataHash required")
	case opts.UV == libfido2.False: // can only be empty or true
		return nil, libfido2.ErrUnsupportedOption
	}

	// Validate PIN only if present and UP is required.
	// This is in line with how current YubiKeys behave.
	privilegedAccess := f.isBio()
	if pin != "" && opts.UP == libfido2.True {
		if err := f.validatePIN(pin); err != nil {
			return nil, err
		}
		privilegedAccess = true
	}

	// Is our credential allowed?
	foundCredential := false
	for _, cred := range credentialIDs {
		if bytes.Equal(cred, f.key.KeyHandle) {
			foundCredential = true
			break
		}

		// Check resident credentials if we are properly authorized.
		if !privilegedAccess {
			continue
		}
		for _, resident := range f.credentials {
			if bytes.Equal(cred, resident.ID) {
				foundCredential = true
				break
			}
		}
		if foundCredential {
			break
		}
	}
	explicitCreds := len(credentialIDs) > 0
	if explicitCreds && !foundCredential {
		return nil, libfido2.ErrNoCredentials
	}

	if err := f.maybeLockUntilInteraction(opts.UP == libfido2.True); err != nil {
		return nil, err
	}

	// Pick a credential for the user?
	switch {
	case !explicitCreds && privilegedAccess && len(f.credentials) > 0:
		// OK, at this point an authenticator picks a credential for the user.
	case !explicitCreds:
		return nil, libfido2.ErrNoCredentials
	}

	return &libfido2.Assertion{
		AuthDataCBOR: assertionAuthDataCBOR,
		Sig:          assertionSig,
	}, nil
}

func (f *fakeFIDO2Device) validatePIN(pin string) error {
	switch {
	case f.isBio() && pin == "": // OK, biometric check supersedes PIN.
	case f.pin != "" && pin == "":
		return libfido2.ErrPinRequired
	case f.pin != "" && f.pin != pin:
		return libfido2.ErrPinInvalid
	}
	return nil
}

func (f *fakeFIDO2Device) hasUV() bool {
	for _, opt := range f.info.Options {
		if opt.Name == "uv" {
			return opt.Value == libfido2.True
		}
	}
	return false
}

func (f *fakeFIDO2Device) isBio() bool {
	for _, opt := range f.info.Options {
		if opt.Name == "bioEnroll" {
			return opt.Value == libfido2.True
		}
	}
	return false
}

func (f *fakeFIDO2Device) maybeLockUntilInteraction(up bool) error {
	if !up {
		return nil // without UserPresence it doesn't lock.
	}

	// Lock until we get a touch or a cancel.
	f.mu.Lock()
	for !f.up && !f.cancel {
		f.cond.Wait()
	}

	// Record/reset state.
	isCancel := f.cancel
	f.up = false
	f.cancel = false

	if isCancel {
		f.mu.Unlock()
		return libfido2.ErrKeepaliveCancel
	}
	f.mu.Unlock()

	return nil
}
