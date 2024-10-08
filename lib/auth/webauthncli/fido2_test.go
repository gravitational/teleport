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

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/google/go-cmp/cmp"
	"github.com/keys-pub/go-libfido2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

var (
	makeCredentialAuthDataRaw, makeCredentialAuthDataCBOR, makeCredentialSig []byte
	assertionAuthDataRaw, assertionAuthDataCBOR, assertionSig                []byte
)

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

// Capture common authenticator options.
var (
	authOpts = []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "clientPin", Value: "false"}, // supported but unset
	}
	pinOpts = []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "clientPin", Value: "true"}, // supported and configured
	}
	bioOpts = []libfido2.Option{
		{Name: "rk", Value: "true"},
		{Name: "up", Value: "true"},
		{Name: "uv", Value: "true"},
		{Name: "plat", Value: "false"},
		{Name: "alwaysUv", Value: "true"},
		{Name: "bioEnroll", Value: "true"}, // supported and configured
		{Name: "clientPin", Value: "true"}, // supported and configured
	}
)

// simplePicker is a credential picker that always picks the first credential.
type simplePicker struct{}

func (p simplePicker) PromptCredential(creds []*wancli.CredentialInfo) (*wancli.CredentialInfo, error) {
	return creds[0], nil
}

type noopPrompt struct {
	simplePicker
}

func (p noopPrompt) PromptPIN() (string, error) {
	return "", nil
}

func (p noopPrompt) PromptTouch() (wancli.TouchAcknowledger, error) {
	return func() error { return nil }, nil
}

// pinCancelPrompt exercises cancellation after device selection.
type pinCancelPrompt struct {
	simplePicker

	pin    string
	cancel context.CancelFunc
}

func (p *pinCancelPrompt) PromptPIN() (string, error) {
	p.cancel()
	return p.pin, nil
}

func (p *pinCancelPrompt) PromptTouch() (wancli.TouchAcknowledger, error) {
	// 2nd touch never happens
	return func() error { return nil }, nil
}

func TestFIDO2Login(t *testing.T) {
	resetFIDO2AfterTests(t)
	wancli.FIDO2PollInterval = 1 * time.Millisecond // run fast on tests

	const rpID = "example.com"
	const appID = "https://example.com"
	const origin = "https://example.com"

	// User IDs and names for resident credentials / passwordless.
	const llamaName = "llama"
	const alpacaName = "alpaca"
	llamaID := make([]byte, 16)
	alpacaID := make([]byte, 16)
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

	challenge, err := wantypes.CreateChallenge()
	require.NoError(t, err, "CreateChallenge failed")

	baseAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:          challenge,
			RelyingPartyID:     rpID,
			AllowedCredentials: []wantypes.CredentialDescriptor{},
			UserVerification:   protocol.VerificationDiscouraged,
			Extensions:         map[string]interface{}{},
		},
	}

	tests := []struct {
		name            string
		timeout         time.Duration
		fido2           *fakeFIDO2
		setUP           func()
		createAssertion func() *wantypes.CredentialAssertion
		prompt          wancli.LoginPrompt
		opts            *wancli.LoginOpts
		// assertResponse and wantErr are mutually exclusive.
		assertResponse func(t *testing.T, resp *wanpb.CredentialAssertionResponse)
		wantErr        string
		wantUser       string
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
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
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
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: pin1.credentialID()},
				}
				return &cp
			},
		},
		{
			name:  "biometric device",
			fido2: newFakeFIDO2(bio1),
			setUP: bio1.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: bio1.credentialID()},
				}
				return &cp
			},
		},
		{
			name:  "legacy device (AppID)",
			fido2: newFakeFIDO2(legacy1),
			setUP: legacy1.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = wantypes.AuthenticationExtensions{
					wantypes.AppIDExtension: appID,
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
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: pin1.credentialID()},
					{CredentialID: bio1.credentialID()},
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = wantypes.AuthenticationExtensions{
					wantypes.AppIDExtension: appID,
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
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: bio1.credentialID()},
					{CredentialID: legacy1.credentialID()},
				}
				cp.Response.Extensions = wantypes.AuthenticationExtensions{
					wantypes.AppIDExtension: "https://badexample.com",
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
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
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
			name:  "NOK no devices plugged errors",
			fido2: newFakeFIDO2(),
			setUP: func() {},
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
				}
				return &cp
			},
			wantErr: "no security keys found",
		},
		{
			name:    "NOK no devices touched times out",
			timeout: 10 * time.Millisecond,
			fido2:   newFakeFIDO2(auth1, pin1, bio1, legacy1),
			setUP:   func() {}, // no interaction
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
					{CredentialID: pin1.credentialID()},
					{CredentialID: bio1.credentialID()},
				}
				return &cp
			},
			wantErr: context.DeadlineExceeded.Error(),
		},
		{
			name:    "NOK single candidate times out",
			timeout: 10 * time.Millisecond,
			fido2:   newFakeFIDO2(auth1, pin1),
			setUP:   func() {}, // no interaction
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
					{CredentialID: auth1.credentialID()},
				}
				return &cp
			},
			wantErr: context.DeadlineExceeded.Error(),
		},
		{
			name:   "NOK cancel after PIN",
			fido2:  newFakeFIDO2(pin3, bio2),        // pin3 and bio2 have resident credentials
			setUP:  pin3.setUP,                      // user chooses pin3, but cancels before further touches
			prompt: &pinCancelPrompt{pin: pin3.pin}, // cancel set on test body
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil // passwordless forces PIN
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			wantErr: context.Canceled.Error(),
		},
		{
			name:  "passwordless pin",
			fido2: newFakeFIDO2(pin3),
			setUP: pin3.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
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
			wantUser: "", // single account response
		},
		{
			name:  "passwordless biometric (llama)",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2,
			opts: &wancli.LoginOpts{
				User: llamaName,
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio2.credentials[0].ID, resp.RawId, "RawId mismatch (want %q resident credential)", llamaName)
				assert.Equal(t, llamaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", llamaName)
			},
			wantUser: llamaName,
		},
		{
			name:  "passwordless biometric (alpaca)",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2,
			opts: &wancli.LoginOpts{
				User: alpacaName,
			},
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio2.credentials[1].ID, resp.RawId, "RawId mismatch (want %q resident credential)", alpacaName)
				assert.Equal(t, alpacaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", alpacaName)
			},
			wantUser: alpacaName,
		},
		{
			name:  "passwordless single-choice credential picker",
			fido2: newFakeFIDO2(pin3),
			setUP: pin3.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
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
			wantUser: "", // single account response
		},
		{
			name:  "passwordless multi-choice credential picker",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2, // picks first credential from list.
			assertResponse: func(t *testing.T, resp *wanpb.CredentialAssertionResponse) {
				assert.Equal(t, bio2.credentials[0].ID, resp.RawId, "RawId mismatch (want %q resident credential)", llamaName)
				assert.Equal(t, llamaID, resp.Response.UserHandle, "UserHandle mismatch (want %q)", llamaName)
			},
			wantUser: llamaName,
		},
		{
			name:  "NOK passwordless no credentials",
			fido2: newFakeFIDO2(bio1),
			setUP: bio1.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt:  bio1,
			wantErr: wancli.ErrUsingNonRegisteredDevice.Error(),
		},
		{
			name:  "NOK passwordless unknown user",
			fido2: newFakeFIDO2(bio2),
			setUP: bio2.setUP,
			createAssertion: func() *wantypes.CredentialAssertion {
				cp := *baseAssertion
				cp.Response.AllowedCredentials = nil
				cp.Response.UserVerification = protocol.VerificationRequired
				return &cp
			},
			prompt: bio2,
			opts: &wancli.LoginOpts{
				User: "camel", // unknown
			},
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
			if pp, ok := prompt.(*pinCancelPrompt); ok {
				pp.cancel = cancel
			}

			// Run FIDO2Login asynchronously, so we can fail the test if it hangs.
			// mfaResp and err checked below.
			var mfaResp *proto.MFAAuthenticateResponse
			var actualUser string
			var err error
			done := make(chan struct{})
			go func() {
				mfaResp, actualUser, err = wancli.FIDO2Login(ctx, origin, test.createAssertion(), prompt, test.opts)
				close(done)
			}()
			select {
			case <-done: // OK, proceed.
			case <-time.After(timeout + 1*time.Second):
				t.Fatal("Timed out waiting for FIDO2Login")
			}

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

			assert.Equal(t, test.wantUser, actualUser, "actual user mismatch")
		})
	}
}

func TestFIDO2Login_retryUVFailures(t *testing.T) {
	resetFIDO2AfterTests(t)

	const user = "llama"
	pin1 := mustNewFIDO2Device("/pin1", "supersecretpinllama", &libfido2.DeviceInfo{
		Options: pinOpts,
	}, &libfido2.Credential{
		ID: []byte{1, 1, 1, 1, 1},
		User: libfido2.User{
			ID:   []byte{1, 1, 1, 1, 2},
			Name: user,
		},
	})
	pin1.failUV = true // fail UV regardless of PIN

	f2 := newFakeFIDO2(pin1)
	f2.setCallbacks()

	const rpID = "example.com"
	const origin = "https://example.com"
	ctx := context.Background()
	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID:   rpID,
			UserVerification: protocol.VerificationRequired,
		},
	}

	pin1.setUP()
	_, _, err := wancli.FIDO2Login(ctx, origin, assertion, pin1 /* prompt */, nil /* opts */)
	require.NoError(t, err, "FIDO2Login failed UV retry")
}

func TestFIDO2Login_singleResidentCredential(t *testing.T) {
	resetFIDO2AfterTests(t)

	const user1Name = "llama"
	const user2Name = "alpaca"
	user1ID := []byte{1, 1, 1, 1, 1}
	user2ID := []byte{1, 1, 1, 1, 2}

	oneCredential := mustNewFIDO2Device("/bio1", "supersecretBIO1pin", &libfido2.DeviceInfo{
		Options: bioOpts,
	}, &libfido2.Credential{
		ID: []byte{1, 1, 1, 1, 1},
		User: libfido2.User{
			ID:   user1ID,
			Name: user1Name,
		},
	})
	manyCredentials := mustNewFIDO2Device("/bio2", "supersecretBIO2pin", &libfido2.DeviceInfo{
		Options: bioOpts,
	},
		&libfido2.Credential{
			ID: user1ID,
			User: libfido2.User{
				ID:   user1ID,
				Name: user1Name,
			},
		},
		&libfido2.Credential{
			ID: user2ID,
			User: libfido2.User{
				ID:   user2ID,
				Name: user2Name,
			},
		})

	f2 := newFakeFIDO2(oneCredential, manyCredentials)
	f2.setCallbacks()

	const rpID = "example.com"
	const origin = "https://example.com"
	ctx := context.Background()
	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID:   rpID,
			UserVerification: protocol.VerificationRequired,
		},
	}

	tests := []struct {
		name       string
		up         func()
		prompt     wancli.LoginPrompt
		opts       *wancli.LoginOpts
		wantUserID []byte
		// Actual user is empty for all single account cases.
		// Authenticators don't return the data.
		wantUser string
	}{
		{
			name:       "single credential with empty user",
			up:         oneCredential.setUP,
			prompt:     oneCredential,
			wantUserID: user1ID,
		},
		{
			name:   "single credential with correct user",
			up:     oneCredential.setUP,
			prompt: oneCredential,
			opts: &wancli.LoginOpts{
				User: user1Name, // happens to match
			},
			wantUserID: user1ID,
		},
		{
			name:   "single credential with ignored user",
			up:     oneCredential.setUP,
			prompt: oneCredential,
			opts: &wancli.LoginOpts{
				User: user2Name, // ignored, we just can't know
			},
			wantUserID: user1ID,
		},
		{
			name:   "multi credentials",
			up:     manyCredentials.setUP,
			prompt: manyCredentials,
			opts: &wancli.LoginOpts{
				User: user2Name, // respected, authenticator returns the data
			},
			wantUserID: user2ID,
			wantUser:   user2Name,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.up()

			resp, gotUser, err := wancli.FIDO2Login(ctx, origin, assertion, test.prompt, test.opts)
			require.NoError(t, err, "FIDO2Login failed")
			gotUserID := resp.GetWebauthn().GetResponse().GetUserHandle()
			assert.Equal(t, test.wantUserID, gotUserID, "FIDO2Login user ID mismatch")
			assert.Equal(t, test.wantUser, gotUser, "FIDO2Login user mismatch")
		})
	}
}

type countingPrompt struct {
	wancli.LoginPrompt
	count, ackCount int
}

func (cp *countingPrompt) PromptTouch() (wancli.TouchAcknowledger, error) {
	cp.count++
	ack, err := cp.LoginPrompt.PromptTouch()
	return func() error {
		cp.ackCount++
		return ack()
	}, err
}

func TestFIDO2Login_PromptTouch(t *testing.T) {
	resetFIDO2AfterTests(t)

	const rpID = "example.com"
	const origin = "https://example.com"

	// auth1 is a FIDO2 authenticator without a PIN configured.
	auth1 := mustNewFIDO2Device("/auth1", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})
	// pin1 is a FIDO2 authenticator with a PIN and resident credentials.
	pin1 := mustNewFIDO2Device("/pin1", "supersecretpin1", &libfido2.DeviceInfo{
		Options: pinOpts,
	}, &libfido2.Credential{
		ID: []byte{1, 1, 1, 1},
		User: libfido2.User{
			ID:   []byte("alpacaID"),
			Name: "alpaca",
		},
	})
	// bio1 is a biometric authenticator with configured resident credentials.
	bio1 := mustNewFIDO2Device("/bio1", "supersecretBIO1pin", &libfido2.DeviceInfo{
		Options: bioOpts,
	}, &libfido2.Credential{
		ID: []byte{1, 1, 1, 2},
		User: libfido2.User{
			ID:   []byte("llamaID"),
			Name: "llama",
		},
	}, &libfido2.Credential{
		ID: []byte{1, 1, 1, 3},
		User: libfido2.User{
			ID:   []byte("alpacaID"),
			Name: "alpaca",
		},
	})

	mfaAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: rpID,
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: auth1.credentialID(),
				},
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: pin1.credentialID(),
				},
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: bio1.credentialID(),
				},
			},
		},
	}
	pwdlessAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        make([]byte, 32),
			RelyingPartyID:   rpID,
			UserVerification: protocol.VerificationRequired,
		},
	}

	tests := []struct {
		name        string
		fido2       *fakeFIDO2
		assertion   *wantypes.CredentialAssertion
		prompt      wancli.LoginPrompt
		opts        *wancli.LoginOpts
		wantTouches int
	}{
		{
			name:        "MFA requires single touch",
			fido2:       newFakeFIDO2(auth1, pin1, bio1),
			assertion:   mfaAssertion,
			prompt:      auth1,
			wantTouches: 1,
		},
		{
			name:        "Passwordless PIN plugged requires two touches",
			fido2:       newFakeFIDO2(pin1),
			assertion:   pwdlessAssertion,
			prompt:      pin1,
			wantTouches: 2,
		},
		{
			name:        "Passwordless PIN not plugged requires two touches",
			fido2:       newFakeFIDO2(pin1),
			assertion:   pwdlessAssertion,
			prompt:      pin1,
			wantTouches: 2,
		},
		{
			name:      "Passwordless Bio requires one touch",
			fido2:     newFakeFIDO2(bio1),
			assertion: pwdlessAssertion,
			prompt:    bio1,
			opts: &wancli.LoginOpts{
				User: "llama",
			},
			wantTouches: 1,
		},
		{
			name:        "Passwordless with multiple devices requires two touches",
			fido2:       newFakeFIDO2(pin1, bio1),
			assertion:   pwdlessAssertion,
			prompt:      pin1,
			wantTouches: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fido2.setCallbacks()

			// Set a timeout, just in case.
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			prompt := &countingPrompt{LoginPrompt: test.prompt}
			_, _, err := wancli.FIDO2Login(ctx, origin, test.assertion, prompt, test.opts)
			require.NoError(t, err, "FIDO2Login errored")
			assert.Equal(t, test.wantTouches, prompt.count, "FIDO2Login did an unexpected number of touch prompts")
			assert.Equal(t, test.wantTouches, prompt.ackCount, "FIDO2Login did an unexpected number of touch acknowledgements")
		})
	}
}

func TestFIDO2Login_u2fDevice(t *testing.T) {
	resetFIDO2AfterTests(t)

	dev := mustNewFIDO2Device("/u2f", "" /* pin */, nil /* info */)
	dev.u2fOnly = true

	f2 := newFakeFIDO2(dev)
	f2.setCallbacks()

	const rpID = "example.com"
	const origin = "https://example.com"

	// Set a ctx timeout in case something goes wrong.
	// Under normal circumstances the test gets nowhere near this timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: rpID,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "rp name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
			},
			User: wantypes.UserEntity{
				ID: []byte{1, 2, 3, 4, 1}, // arbitrary,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "user name",
				},
				DisplayName: "user display name",
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	dev.setUP() // simulate touch
	ccr, err := wancli.FIDO2Register(ctx, origin, cc, dev /* prompt */)
	require.NoError(t, err, "FIDO2Register errored")

	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID: rpID,
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: ccr.GetWebauthn().GetRawId(),
				},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}

	dev.setUP() // simulate touch
	_, _, err = wancli.FIDO2Login(ctx, origin, assertion, dev /* prompt */, nil /* opts */)
	assert.NoError(t, err, "FIDO2Login errored")
}

// TestFIDO2Login_u2fDeviceNotRegistered tests assertions with a non-registered
// U2F device plugged.
//
// U2F devices error immediately when not registered, which makes their behavior
// distinct from FIDO2 and requires additional logic to be correctly handled.
//
// This test captures an U2F assertion regression.
func TestFIDO2Login_u2fDeviceNotRegistered(t *testing.T) {
	resetFIDO2AfterTests(t)

	u2fDev := mustNewFIDO2Device("/u2f", "" /* pin */, nil /* info */)
	u2fDev.u2fOnly = true

	registeredDev := mustNewFIDO2Device("/dev2", "" /* pin */, &libfido2.DeviceInfo{
		Options: bioOpts,
	})

	f2 := newFakeFIDO2(u2fDev, registeredDev)
	f2.setCallbacks()

	const rpID = "example.com"
	const origin = "https://example.com"

	// Set a ctx timeout in case something goes wrong.
	// Under normal circumstances the test gets nowhere near this timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Register our "registeredDev".
	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: rpID,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "rp name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
			},
			User: wantypes.UserEntity{
				ID: []byte{1, 2, 3, 4, 1}, // arbitrary,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "user name",
				},
				DisplayName: "user display name",
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}
	registeredDev.setUP() // simulate touch
	ccr, err := wancli.FIDO2Register(ctx, origin, cc, registeredDev /* prompt */)
	require.NoError(t, err, "FIDO2Register errored")

	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID: rpID,
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: ccr.GetWebauthn().GetRawId(),
				},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}

	tests := []struct {
		name    string
		prompt  wancli.LoginPrompt
		timeout time.Duration
		wantErr error
	}{
		{
			name:   "registered device touched",
			prompt: &delayedPrompt{registeredDev}, // Give the U2F device time to fail.
		},
		{
			name:    "no devices touched",
			prompt:  noopPrompt{}, // `registered` not touched, U2F won't blink.
			timeout: 10 * time.Millisecond,
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Apply custom timeout.
			ctx := ctx
			if test.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), test.timeout)
				defer cancel()
			}

			_, _, err := wancli.FIDO2Login(ctx, origin, assertion, test.prompt, nil /* opts */)
			assert.ErrorIs(t, err, test.wantErr, "FIDO2Login error mismatch")
		})
	}
}

type delayedPrompt struct {
	wancli.LoginPrompt
}

func (p *delayedPrompt) PromptTouch() (wancli.TouchAcknowledger, error) {
	const delay = 100 * time.Millisecond
	time.Sleep(delay)
	return p.LoginPrompt.PromptTouch()
}

func TestFIDO2Login_bioErrorHandling(t *testing.T) {
	resetFIDO2AfterTests(t)

	// bio is a biometric authenticator with configured resident credentials.
	bio := mustNewFIDO2Device("/bio", "supersecretBIOpin", &libfido2.DeviceInfo{
		Options: bioOpts,
	}, &libfido2.Credential{
		User: libfido2.User{
			ID:   []byte{1, 2, 3, 4, 5}, // unimportant
			Name: "llama",
		},
	})

	f2 := newFakeFIDO2(bio)
	f2.setCallbacks()

	// Prepare a passwordless assertion.
	// MFA would do as well; both are realistic here.
	const origin = "https://example.com"
	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:          []byte{1, 2, 3, 4, 5},
			RelyingPartyID:     "example.com",
			AllowedCredentials: nil,                           // passwordless
			UserVerification:   protocol.VerificationRequired, // passwordless
		},
	}

	tests := []struct {
		name               string
		setAssertionErrors func()
		wantMsg            string
	}{
		{
			name:               "success (sanity check)",
			setAssertionErrors: func() { bio.assertionErrors = nil },
		},
		{
			name: "libfido2 error 60 fails with custom message",
			setAssertionErrors: func() {
				bio.assertionErrors = []error{
					libfido2.Error{Code: 60},
				}
			},
			wantMsg: "user verification function",
		},
		{
			name: "libfido2 error 63 retried",
			setAssertionErrors: func() {
				bio.assertionErrors = []error{
					libfido2.Error{Code: 63},
					libfido2.Error{Code: 63},
				}
			},
		},
		{
			name: "error retry has a limit",
			setAssertionErrors: func() {
				bio.assertionErrors = []error{
					libfido2.Error{Code: 63},
					libfido2.Error{Code: 63},
					libfido2.Error{Code: 63},
					libfido2.Error{Code: 63},
					libfido2.Error{Code: 63},
				}
			},
			wantMsg: "libfido2 error 63",
		},
		{
			name: "retry on operation denied",
			setAssertionErrors: func() {
				bio.assertionErrors = []error{
					// Note: this happens only for UV=false assertions. UV=true failures
					// return error 63.
					libfido2.ErrOperationDenied,
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setAssertionErrors()

			// Use a ctx with timeout just to be safe. We shouldn't hit the timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, _, err := wancli.FIDO2Login(ctx, origin, assertion, bio /* prompt */, nil /* opts */)
			if test.wantMsg == "" {
				require.NoError(t, err, "FIDO2Login returned non-nil error")
			} else {
				require.ErrorContains(t, err, test.wantMsg, "FIDO2Login returned an unexpected error")
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
	okAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []wantypes.CredentialDescriptor{
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
		assertion *wantypes.CredentialAssertion
		prompt    wancli.LoginPrompt
		wantErr   string
	}{
		{
			name:      "nil origin",
			assertion: okAssertion,
			prompt:    prompt,
			wantErr:   "origin",
		},
		{
			name:      "nil prompt",
			origin:    origin,
			assertion: okAssertion,
			wantErr:   "prompt",
		},
		{
			name:      "assertion without challenge",
			origin:    origin,
			assertion: &nilChallengeAssertion,
			prompt:    prompt,
			wantErr:   "challenge",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, _, err := wancli.FIDO2Login(ctx, test.origin, test.assertion, test.prompt, nil /* opts */)
			require.Error(t, err, "FIDO2Login returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "FIDO2Login returned err = %q, want %q", err, test.wantErr)
		})
	}
}

// TestFIDO2_LoginRegister_interactionErrors tests scenarios where the user
// picks a security key that cannot be used for that particular flow (and the
// subsequent error message).
func TestFIDO2_LoginRegister_interactionErrors(t *testing.T) {
	resetFIDO2AfterTests(t)

	notRegistered := mustNewFIDO2Device("/notregistered", "mysupersecretpinLLAMA", &libfido2.DeviceInfo{
		Options: pinOpts,
	})
	// PIN capable but unset.
	noPIN := mustNewFIDO2Device("/nouv", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})
	// fictional PIN capable, RK incapable device.
	noRK := mustNewFIDO2Device("/nork", "" /* pin */, &libfido2.DeviceInfo{
		Options: []libfido2.Option{
			{Name: "rk", Value: "false"},       // not capable
			{Name: "up", Value: "true"},        // expected setting
			{Name: "plat", Value: "false"},     // expected setting
			{Name: "clientPin", Value: "true"}, // supported and configured
		},
	})
	// U2F only device (no FIDO2 capabilities).
	u2f := mustNewFIDO2Device("/u2f", "" /* pin */, nil /* info */)
	u2f.u2fOnly = true

	f2 := newFakeFIDO2(notRegistered, noPIN, noRK, u2f)
	f2.setCallbacks()

	const rpID = "goteleport.com"
	const origin = "https://goteleport.com"
	mfaCC := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingParty: wantypes.RelyingPartyEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "Teleport",
				},
				ID: rpID,
			},
			User: wantypes.UserEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "llama",
				},
				DisplayName: "Llama",
				ID:          []byte{1, 1, 1, 1, 1}, // arbitrary
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				RequireResidentKey: protocol.ResidentKeyNotRequired(),
				ResidentKey:        protocol.ResidentKeyRequirementDiscouraged,
				UserVerification:   protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	// Setup: register all devices (except "notRegistered") for MFA.
	// A typical MFA registration allows all kinds of devices.
	ctx := context.Background()
	var registeredCreds [][]byte
	for _, dev := range []*fakeFIDO2Device{noPIN, noRK, u2f} {
		resp, err := wancli.FIDO2Register(ctx, origin, mfaCC, dev)
		if err != nil {
			t.Fatalf("FIDO2Register failed, device %v: %v", dev.path, err)
		}
		registeredCreds = append(registeredCreds, resp.GetWebauthn().RawId)
	}

	mfaAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID:   rpID,
			UserVerification: protocol.VerificationDiscouraged,
		},
	}
	for _, cred := range registeredCreds {
		mfaAssertion.Response.AllowedCredentials = append(mfaAssertion.Response.AllowedCredentials, wantypes.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred,
		})
	}

	pwdlessAssertion := *mfaAssertion
	pwdlessAssertion.Response.AllowedCredentials = nil
	pwdlessAssertion.Response.UserVerification = protocol.VerificationRequired

	// FIDO2Login interaction tests.
	for _, test := range []struct {
		name            string
		createAssertion func() *wantypes.CredentialAssertion
		prompt          wancli.LoginPrompt
		wantErr         string
	}{
		{
			name:            "no registered credential",
			createAssertion: func() *wantypes.CredentialAssertion { return mfaAssertion },
			prompt:          notRegistered,
			wantErr:         wancli.ErrUsingNonRegisteredDevice.Error(),
		},
		{
			// Theoretically could happen, but not something we do today.
			name: "mfa lacks UV",
			createAssertion: func() *wantypes.CredentialAssertion {
				mfaUV := *mfaAssertion
				mfaUV.Response.UserVerification = protocol.VerificationRequired
				return &mfaUV
			},
			prompt:  noPIN, // PIN unset means it cannot do UV
			wantErr: "user verification",
		},
		{
			name:            "passwordless lacks UV",
			createAssertion: func() *wantypes.CredentialAssertion { return &pwdlessAssertion },
			prompt:          noPIN, // PIN unset means it cannot do UV
			wantErr:         "passwordless",
		},
		{
			// Fictional scenario, no real-world authenticators match.
			name:            "passwordless lacks RK",
			createAssertion: func() *wantypes.CredentialAssertion { return &pwdlessAssertion },
			prompt:          noRK,
			wantErr:         "passwordless",
		},
		{
			name:            "passwordless U2F",
			createAssertion: func() *wantypes.CredentialAssertion { return &pwdlessAssertion },
			prompt:          u2f,
			wantErr:         "cannot do passwordless",
		},
	} {
		t.Run("login/"+test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			_, _, err := wancli.FIDO2Login(ctx, origin, test.createAssertion(), test.prompt, nil /* opts */)
			assert.ErrorContains(t, err, test.wantErr, "FIDO2Login error mismatch")
		})
	}

	excludeCC := *mfaCC
	for _, cred := range registeredCreds {
		excludeCC.Response.CredentialExcludeList = append(excludeCC.Response.CredentialExcludeList, wantypes.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred,
		})
	}

	pwdlessCC := *mfaCC
	pwdlessCC.Response.AuthenticatorSelection.RequireResidentKey = protocol.ResidentKeyRequired()
	pwdlessCC.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
	pwdlessCC.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired

	// FIDO2Register interaction tests.
	for _, test := range []struct {
		name     string
		createCC func() *wantypes.CredentialCreation
		prompt   wancli.RegisterPrompt
		wantErr  string
	}{
		{
			name:     "excluded credential",
			createCC: func() *wantypes.CredentialCreation { return &excludeCC },
			prompt:   noPIN,
			wantErr:  "registered credential",
		},
		{
			name:     "excluded credential (U2F)",
			createCC: func() *wantypes.CredentialCreation { return &excludeCC },
			prompt:   u2f,
			wantErr:  "registered credential",
		},
		{
			name:     "passwordless lacks UV",
			createCC: func() *wantypes.CredentialCreation { return &pwdlessCC },
			prompt:   noPIN, // PIN unset means it cannot do UV
			wantErr:  "user verification",
		},
		{
			// Fictional scenario, no real-world authenticators match.
			name:     "passwordless lacks RK",
			createCC: func() *wantypes.CredentialCreation { return &pwdlessCC },
			prompt:   noRK,
			wantErr:  "resident key",
		},
		{
			name:     "passwordless U2F",
			createCC: func() *wantypes.CredentialCreation { return &pwdlessCC },
			prompt:   u2f,
			wantErr:  "cannot do passwordless",
		},
	} {
		t.Run("register/"+test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			_, err := wancli.FIDO2Register(ctx, origin, test.createCC(), test.prompt)
			assert.ErrorContains(t, err, test.wantErr, "FIDO2Register error mismatch")
		})
	}
}

func TestFIDO2Register(t *testing.T) {
	resetFIDO2AfterTests(t)

	const rpID = "example.com"
	const origin = "https://example.com"

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
	// bio1 is a biometric authenticator.
	bio1 := mustNewFIDO2Device("/bio1", "supersecretBIOpin", &libfido2.DeviceInfo{
		Options: []libfido2.Option{
			{Name: "rk", Value: "true"},
			{Name: "up", Value: "true"},
			{Name: "uv", Value: "true"},
			{Name: "plat", Value: "false"},
			{Name: "alwaysUv", Value: "true"},
			{Name: "bioEnroll", Value: "true"}, // supported and configured
			{Name: "clientPin", Value: "true"}, // supported and configured
		},
	})
	// u2f1 is an authenticator that uses fido-u2f attestation.
	u2f1 := mustNewFIDO2Device("/u2f1", "" /* pin */, &libfido2.DeviceInfo{Options: authOpts})
	u2f1.format = "fido-u2f"
	// none1 is an authenticator that returns no attestation data.
	none1 := mustNewFIDO2Device("/none1", "" /* pin */, &libfido2.DeviceInfo{Options: authOpts})
	none1.format = "none"

	challenge, err := wantypes.CreateChallenge()
	require.NoError(t, err, "CreateChallenge failed")

	baseCC := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: challenge,
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: rpID,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "rp name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			User: wantypes.UserEntity{
				ID: []byte{1, 2, 3, 4, 1}, // arbitrary,
				CredentialEntity: protocol.CredentialEntity{
					Name: "user name",
				},
				DisplayName: "user display name",
			},
			Attestation: protocol.PreferDirectAttestation,
		},
	}
	pwdlessCC := *baseCC
	pwdlessCC.Response.RelyingParty.Name = "Teleport"
	pwdlessCC.Response.User = wantypes.UserEntity{
		CredentialEntity: wantypes.CredentialEntity{
			Name: "llama",
		},
		DisplayName: "Llama",
		ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
	}
	pwdlessRRK := true
	pwdlessCC.Response.AuthenticatorSelection.RequireResidentKey = &pwdlessRRK
	pwdlessCC.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired

	tests := []struct {
		name             string
		timeout          time.Duration
		fido2            *fakeFIDO2
		setUP            func()
		createCredential func() *wantypes.CredentialCreation
		prompt           wancli.RegisterPrompt
		wantErr          error
		assertResponse   func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject)
	}{
		{
			name:  "single device, packed attestation",
			fido2: newFakeFIDO2(auth1),
			setUP: auth1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, auth1.credentialID(), ccr.RawId, "RawId mismatch")

				// Assert attestation algorithm and signature.
				require.Equal(t, "packed", attObj.Format, "attestation format mismatch")
				assert.Equal(t, int64(webauthncose.AlgES256), attObj.AttStatement["alg"], "attestation alg mismatch")
				assert.Equal(t, makeCredentialSig, attObj.AttStatement["sig"], "attestation sig mismatch")

				// Assert attestation certificate.
				x5cInterface := attObj.AttStatement["x5c"]
				x5c, ok := x5cInterface.([]interface{})
				require.True(t, ok, "attestation x5c type mismatch (got %T)", x5cInterface)
				assert.Len(t, x5c, 1, "attestation x5c length mismatch")
				assert.Equal(t, auth1.cert(), x5c[0], "attestation cert mismatch")
			},
		},
		{
			name:  "fido-u2f attestation",
			fido2: newFakeFIDO2(u2f1),
			setUP: u2f1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				// Assert attestation signature.
				require.Equal(t, "fido-u2f", attObj.Format, "attestation format mismatch")
				assert.Equal(t, makeCredentialSig, attObj.AttStatement["sig"], "attestation sig mismatch")

				// Assert attestation certificate.
				x5cInterface := attObj.AttStatement["x5c"]
				x5c, ok := x5cInterface.([]interface{})
				require.True(t, ok, "attestation x5c type mismatch (got %T)", x5cInterface)
				assert.Len(t, x5c, 1, "attestation x5c length mismatch")
				assert.Equal(t, u2f1.cert(), x5c[0], "attestation cert mismatch")
			},
		},
		{
			name:  "none attestation",
			fido2: newFakeFIDO2(none1),
			setUP: none1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, "none", attObj.Format, "attestation format mismatch")
			},
		},
		{
			name:  "pin device",
			fido2: newFakeFIDO2(pin1),
			setUP: pin1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			prompt: pin1,
		},
		{
			name:  "multiple valid devices",
			fido2: newFakeFIDO2(auth1, pin1, pin2, bio1),
			setUP: bio1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, bio1.credentialID(), ccr.RawId, "RawId mismatch (want bio1)")
			},
		},
		{
			name:  "multiple devices, uses pin",
			fido2: newFakeFIDO2(auth1, pin1, pin2, bio1),
			setUP: pin2.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				return &cp
			},
			prompt: pin2,
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, pin2.credentialID(), ccr.RawId, "RawId mismatch (want pin2)")
			},
		},
		{
			name:  "excluded devices, single valid",
			fido2: newFakeFIDO2(auth1, bio1),
			setUP: bio1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				cp.Response.CredentialExcludeList = []wantypes.CredentialDescriptor{
					{
						Type:         protocol.PublicKeyCredentialType,
						CredentialID: auth1.credentialID(),
					},
				}
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, bio1.credentialID(), ccr.RawId, "RawId mismatch (want bio1)")
			},
		},
		{
			name:  "excluded devices, multiple valid",
			fido2: newFakeFIDO2(auth1, pin1, pin2, bio1),
			setUP: bio1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := *baseCC
				cp.Response.CredentialExcludeList = []wantypes.CredentialDescriptor{
					{
						Type:         protocol.PublicKeyCredentialType,
						CredentialID: pin1.credentialID(),
					},
					{
						Type:         protocol.PublicKeyCredentialType,
						CredentialID: pin2.credentialID(),
					},
				}
				return &cp
			},
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				assert.Equal(t, bio1.credentialID(), ccr.RawId, "RawId mismatch (want bio1)")
			},
		},
		{
			name:  "passwordless pin device",
			fido2: newFakeFIDO2(pin2),
			setUP: pin2.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := pwdlessCC
				return &cp
			},
			prompt: pin2,
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				require.NotEmpty(t, pin2.credentials, "no resident credentials added to pin2")
				cred := pin2.credentials[len(pin2.credentials)-1]
				assert.Equal(t, cred.ID, ccr.RawId, "RawId mismatch (want pin2 resident credential)")
			},
		},
		{
			name:  "passwordless bio device",
			fido2: newFakeFIDO2(bio1),
			setUP: bio1.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := pwdlessCC
				return &cp
			},
			prompt: bio1,
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				require.NotEmpty(t, bio1.credentials, "no resident credentials added to bio1")
				cred := bio1.credentials[len(bio1.credentials)-1]
				assert.Equal(t, cred.ID, ccr.RawId, "RawId mismatch (want bio1 resident credential)")
			},
		},
		{
			name:  "passwordless ResidentKey=required",
			fido2: newFakeFIDO2(pin2),
			setUP: pin2.setUP,
			createCredential: func() *wantypes.CredentialCreation {
				cp := pwdlessCC
				cp.Response.AuthenticatorSelection.RequireResidentKey = nil
				cp.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
				return &cp
			},
			prompt: pin2,
			assertResponse: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, attObj *protocol.AttestationObject) {
				require.NotEmpty(t, pin2.credentials, "no resident credentials added to pin2")
				cred := pin2.credentials[len(pin2.credentials)-1]
				assert.Equal(t, cred.ID, ccr.RawId, "RawId mismatch (want pin2 resident credential)")
			},
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
			mfaResp, err := wancli.FIDO2Register(ctx, origin, test.createCredential(), prompt)
			switch {
			case test.wantErr != nil && err == nil:
				t.Fatalf("FIDO2Register returned err = nil, wantErr %q", test.wantErr)
			case test.wantErr != nil:
				require.ErrorIs(t, err, test.wantErr, "FIDO2Register returned err = %q, wantErr %q", err, test.wantErr)
				return
			default:
				require.NoError(t, err, "FIDO2Register failed")
				require.NotNil(t, mfaResp, "mfaResp nil")
			}

			// Do a few baseline checks, tests can assert further.
			got := mfaResp.GetWebauthn()
			require.NotNil(t, got, "credential response nil")
			require.NotNil(t, got.Response, "attestation response nil")
			assert.NotNil(t, got.Response.ClientDataJson, "ClientDataJSON nil")
			want := &wanpb.CredentialCreationResponse{
				Type:  string(protocol.PublicKeyCredentialType),
				RawId: got.RawId,
				Response: &wanpb.AuthenticatorAttestationResponse{
					ClientDataJson:    got.Response.ClientDataJson,
					AttestationObject: got.Response.AttestationObject,
				},
				Extensions: got.Extensions,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("FIDO2Register()/CredentialCreationResponse mismatch (-want +got):\n%v", diff)
			}

			attObj := &protocol.AttestationObject{}
			err = cbor.Unmarshal(got.Response.AttestationObject, attObj)
			require.NoError(t, err, "Failed to unmarshal AttestationObject")
			assert.Equal(t, makeCredentialAuthDataRaw, attObj.RawAuthData, "RawAuthData mismatch")

			if test.assertResponse != nil {
				test.assertResponse(t, got, attObj)
			}
		})
	}
}

func TestFIDO2Register_errors(t *testing.T) {
	resetFIDO2AfterTests(t)

	// Make sure we won't call the real libfido2.
	f2 := newFakeFIDO2()
	f2.setCallbacks()

	const origin = "https://example.com"
	okCC := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "rp name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			User: wantypes.UserEntity{
				ID: []byte{1, 2, 3, 4, 1}, // arbitrary,
				CredentialEntity: wantypes.CredentialEntity{
					Name: "user name",
				},
				DisplayName: "user display name",
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	pwdlessOK := *okCC
	pwdlessOK.Response.RelyingParty.Name = "Teleport"
	pwdlessOK.Response.User = wantypes.UserEntity{
		CredentialEntity: wantypes.CredentialEntity{
			Name: "llama",
		},
		DisplayName: "Llama",
		ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
	}
	rrk := true
	pwdlessOK.Response.AuthenticatorSelection.RequireResidentKey = &rrk
	pwdlessOK.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired

	var prompt noopPrompt

	tests := []struct {
		name     string
		origin   string
		createCC func() *wantypes.CredentialCreation
		prompt   wancli.RegisterPrompt
		wantErr  string
	}{
		{
			name:     "nil origin",
			createCC: func() *wantypes.CredentialCreation { return okCC },
			prompt:   prompt,
			wantErr:  "origin",
		},
		{
			name:   "cc without challenge",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			prompt:  prompt,
			wantErr: "challenge",
		},
		{
			name:   "cc unsupported parameters",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.Parameters = []wantypes.CredentialParameter{
					{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgEdDSA},
				}
				return &cp
			},
			prompt:  prompt,
			wantErr: "ES256",
		},
		{
			name:     "nil pinPrompt",
			origin:   origin,
			createCC: func() *wantypes.CredentialCreation { return okCC },
			wantErr:  "prompt",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, err := wancli.FIDO2Register(ctx, test.origin, test.createCC(), test.prompt)
			require.Error(t, err, "FIDO2Register returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "FIDO2Register returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func TestFIDO2Register_u2fExcludedCredentials(t *testing.T) {
	resetFIDO2AfterTests(t)

	u2fDev := mustNewFIDO2Device("/u2f", "" /* pin */, nil /* info */)
	u2fDev.u2fOnly = true

	// otherDev is FIDO2 in this test, but it could be any non-registered device.
	otherDev := mustNewFIDO2Device("/fido2", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})

	f2 := newFakeFIDO2(u2fDev, otherDev)
	f2.setCallbacks()

	const origin = "https://example.com"
	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "rp name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			User: wantypes.UserEntity{
				ID: []byte{1, 2, 3, 4, 1}, // arbitrary,
				CredentialEntity: protocol.CredentialEntity{
					Name: "user name",
				},
				DisplayName: "user display name",
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	ctx := context.Background()

	// Setup: register the U2F device.
	resp, err := wancli.FIDO2Register(ctx, origin, cc, u2fDev)
	require.NoError(t, err, "FIDO2Register errored")

	// Setup: mark the registered credential as excluded.
	cc.Response.CredentialExcludeList = append(cc.Response.CredentialExcludeList, wantypes.CredentialDescriptor{
		Type:         protocol.PublicKeyCredentialType,
		CredentialID: resp.GetWebauthn().GetRawId(),
	})

	// Register a new device, making sure a failed excluded credential assertion
	// won't break the ceremony.
	_, err = wancli.FIDO2Register(ctx, origin, cc, otherDev)
	require.NoError(t, err, "FIDO2Register errored, expected a successful registration")
}

// TestFIDO2Login_u2fInternalError tests the scenario described by issue
// https://github.com/gravitational/teleport/issues/44912.
func TestFIDO2Login_u2fInternalError(t *testing.T) {
	resetFIDO2AfterTests(t)

	dev1 := mustNewFIDO2Device("/dev1", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})
	dev2 := mustNewFIDO2Device("/dev2", "" /* pin */, &libfido2.DeviceInfo{
		Options: authOpts,
	})
	u2fDev := mustNewFIDO2Device("/u2f", "" /* pin */, nil /* info */)
	u2fDev.u2fOnly = true
	u2fDev.errorOnUnknownCredential = true

	f2 := newFakeFIDO2(dev1, dev2, u2fDev)
	f2.setCallbacks()

	const origin = "https://example.com"
	ctx := context.Background()

	// Register all authenticators.
	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				CredentialEntity: protocol.CredentialEntity{
					Name: "example.com",
				},
				ID: "example.com",
			},
			User: wantypes.UserEntity{
				CredentialEntity: protocol.CredentialEntity{
					Name: "alpaca",
				},
				DisplayName: "Alpaca",
				ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				RequireResidentKey: protocol.ResidentKeyNotRequired(),
				ResidentKey:        protocol.ResidentKeyRequirementDiscouraged,
				UserVerification:   protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}
	allowedCreds := make([]wantypes.CredentialDescriptor, 0, len(f2.devices))
	for _, dev := range f2.devices {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		mfaResp, err := wancli.FIDO2Register(ctx, origin, cc, dev)
		cancel()
		require.NoError(t, err, "FIDO2Register failed")

		allowedCreds = append(allowedCreds, wantypes.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: mfaResp.GetWebauthn().RawId,
		})
	}

	// Sanity check: authenticator errors in the presence of unknown credentials.
	u2fDev.open()
	_, err := u2fDev.Assertion(
		"example.com",
		[]byte(`55cde2973243a946b85a477d2e164a35d2e4f3daaeb11ac5e9a1c4cf3297033e`), // clientDataHash
		[][]byte{
			u2fDev.credentialID(),
			bytes.Repeat([]byte("A"), 96),
		},
		"", // pin
		&libfido2.AssertionOpts{UP: libfido2.False},
	)
	require.ErrorIs(t, err, libfido2.ErrInternal, "u2fDev.Assert error mismatch")
	u2fDev.Close()

	t.Run("login with multiple credentials", func(t *testing.T) {
		assertion := &wantypes.CredentialAssertion{
			Response: wantypes.PublicKeyCredentialRequestOptions{
				Challenge:          make([]byte, 32),
				RelyingPartyID:     "example.com",
				AllowedCredentials: allowedCreds,
				UserVerification:   protocol.VerificationDiscouraged,
			},
		}

		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		_, _, err := wancli.FIDO2Login(ctx, origin, assertion, u2fDev, &wancli.LoginOpts{
			User: "alpaca",
		})
		require.NoError(t, err, "FIDO2Login failed")
	})
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
	*wancli.FIDODeviceLocations = f.DeviceLocations
	*wancli.FIDONewDevice = f.NewDevice
}

func (f *fakeFIDO2) DeviceLocations() ([]*libfido2.DeviceLocation, error) {
	return f.locs, nil
}

func (f *fakeFIDO2) NewDevice(path string) (wancli.FIDODevice, error) {
	if dev, ok := f.devices[path]; ok {
		dev.open()
		return dev, nil
	}
	// go-libfido2 doesn't actually error here, but we do for simplicity.
	return nil, errors.New("not found")
}

type fakeFIDO2Device struct {
	simplePicker

	// Set to true to cause "unsupported option" UV errors, regardless of other
	// conditions.
	failUV bool

	// Set to true to simulate an U2F-only device.
	// Causes libfido2.ErrNotFIDO2 on Info.
	u2fOnly bool

	// errorOnUnknownCredential makes the device fail assertions if an unknown
	// credential is present.
	errorOnUnknownCredential bool

	// assertionErrors is a chain of errors to return from Assertion.
	// Errors are returned from start to end and removed, one-by-one, on each
	// invocation of the Assertion method.
	// If the slice is empty, Assertion runs normally.
	assertionErrors []error

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

	// cond guards up and cancel.
	cond               *sync.Cond
	up, cancel, opened bool
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

	return &fakeFIDO2Device{
		path:        path,
		pin:         pin,
		credentials: creds,
		format:      "packed",
		info:        info,
		key:         key,
		pubKey:      pubKeyCBOR,
		cond:        sync.NewCond(&sync.Mutex{}),
	}, nil
}

func (f *fakeFIDO2Device) PromptPIN() (string, error) {
	return f.pin, nil
}

func (f *fakeFIDO2Device) PromptTouch() (wancli.TouchAcknowledger, error) {
	f.setUP()
	return func() error { return nil }, nil
}

func (f *fakeFIDO2Device) credentialID() []byte {
	return f.key.KeyHandle
}

func (f *fakeFIDO2Device) cert() []byte {
	return f.key.Cert
}

func (f *fakeFIDO2Device) open() {
	f.cond.L.Lock()
	// Keep the `f.up` value from before open(), it makes tests simpler.
	f.cancel = false
	f.opened = true
	f.cond.L.Unlock()
}

func (f *fakeFIDO2Device) verifyOpen() error {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()
	if !f.opened {
		return errors.New("device closed")
	}
	return nil
}

func (f *fakeFIDO2Device) setUP() {
	f.cond.L.Lock()
	// Set up regardless of opened, makes testing simpler.
	f.up = true
	f.cond.L.Unlock()
	f.cond.Broadcast()
}

func (f *fakeFIDO2Device) Cancel() error {
	f.cond.L.Lock()
	// Ignore cancels while closed, as this mirrors go-libfido2.
	if f.opened {
		f.cancel = true
	}
	f.cond.L.Unlock()
	f.cond.Broadcast()
	return nil
}

func (f *fakeFIDO2Device) Close() error {
	f.cond.L.Lock()
	f.opened = false
	f.cond.L.Unlock()
	f.cond.Broadcast() // Unblock any ongoing goroutines.
	return nil
}

func (f *fakeFIDO2Device) Info() (*libfido2.DeviceInfo, error) {
	if err := f.verifyOpen(); err != nil {
		return nil, err
	}
	if f.u2fOnly {
		return nil, libfido2.ErrNotFIDO2
	}
	return f.info, nil
}

func (f *fakeFIDO2Device) IsFIDO2() (bool, error) {
	if err := f.verifyOpen(); err != nil {
		return false, err
	}
	return !f.u2fOnly, nil
}

func (f *fakeFIDO2Device) SetTimeout(d time.Duration) error {
	if err := f.verifyOpen(); err != nil {
		return err
	}
	return nil
}

func (f *fakeFIDO2Device) MakeCredential(
	clientDataHash []byte,
	rp libfido2.RelyingParty,
	user libfido2.User,
	typ libfido2.CredentialType,
	pin string,
	opts *libfido2.MakeCredentialOpts,
) (*libfido2.Attestation, error) {
	if err := f.verifyOpen(); err != nil {
		return nil, err
	}

	switch {
	case len(clientDataHash) == 0:
		return nil, errors.New("clientDataHash required")
	case rp.ID == "":
		return nil, errors.New("rp.ID required")
	case typ != libfido2.ES256:
		return nil, errors.New("bad credential type")
	case opts.UV == libfido2.False: // can only be empty or true
		return nil, libfido2.ErrUnsupportedOption
	case opts.UV == libfido2.True && !f.hasUV():
		return nil, libfido2.ErrUnsupportedOption // PIN authenticators don't like UV
	case opts.RK == libfido2.True && !f.hasRK():
		// TODO(codingllama): Confirm scenario with a real authenticator.
		return nil, libfido2.ErrUnsupportedOption
	}

	// Validate PIN regardless of opts.
	// This is in line with how current YubiKeys behave.
	if err := f.validatePIN(pin); err != nil {
		return nil, err
	}

	if err := f.maybeLockUntilInteraction(true /* up */); err != nil {
		return nil, err
	}

	cert, sig := f.cert(), makeCredentialSig
	if f.format == "none" {
		// Do not return attestation data in case of "none".
		// This is a hypothetical scenario, as I haven't seen device that does this.
		cert, sig = nil, nil
	}

	// Did we create a resident credential? Create a new ID for it and record it.
	cID := f.key.KeyHandle
	if opts.RK == libfido2.True {
		cID = make([]byte, 16) // somewhat arbitrary
		if _, err := rand.Read(cID); err != nil {
			return nil, err
		}
		f.credentials = append(f.credentials, &libfido2.Credential{
			ID:   cID,
			Type: libfido2.ES256,
			User: user,
		})
	}

	return &libfido2.Attestation{
		ClientDataHash: clientDataHash,
		AuthData:       makeCredentialAuthDataCBOR,
		CredentialID:   cID,
		CredentialType: libfido2.ES256,
		PubKey:         f.pubKey,
		Cert:           cert,
		Sig:            sig,
		Format:         f.format,
	}, nil
}

func (f *fakeFIDO2Device) Assertion(
	rpID string,
	clientDataHash []byte,
	credentialIDs [][]byte,
	pin string,
	opts *libfido2.AssertionOpts,
) ([]*libfido2.Assertion, error) {
	if err := f.verifyOpen(); err != nil {
		return nil, err
	}

	// Give preference to simulated errors.
	if len(f.assertionErrors) > 0 {
		err := f.assertionErrors[0]
		f.assertionErrors = f.assertionErrors[1:]
		return nil, err
	}

	switch {
	case rpID == "":
		return nil, errors.New("rp.ID required")
	case len(clientDataHash) == 0:
		return nil, errors.New("clientDataHash required")
	}

	// Validate UV.
	switch {
	case opts.UV == "": // OK, actually works as false.
	case opts.UV == libfido2.True && f.failUV:
		// Emulate UV failures, as seen in some devices regardless of other
		// settings.
		return nil, libfido2.ErrUnsupportedOption
	case opts.UV == libfido2.True && f.isBio(): // OK.
	case opts.UV == libfido2.True && f.hasClientPin() && pin != "": // OK, doubles as UV.
	default: // Anything else is invalid, including libfido2.False.
		return nil, libfido2.ErrUnsupportedOption
	}

	// Validate PIN only if present and UP is required.
	// This is in line with how current YubiKeys behave.
	// TODO(codingllama): This should probably take UV into consideration.
	privilegedAccess := f.isBio()
	if pin != "" && opts.UP == libfido2.True {
		if err := f.validatePIN(pin); err != nil {
			return nil, err
		}
		privilegedAccess = true
	}

	// U2F only: exit without user interaction if there are no credentials.
	if f.u2fOnly {
		found := false
		for _, cid := range credentialIDs {
			if bytes.Equal(cid, f.key.KeyHandle) {
				found = true
				break
			}
			if f.errorOnUnknownCredential {
				return nil, fmt.Errorf("failed to get assertion: %w", libfido2.ErrInternal)
			}
		}
		if !found {
			return nil, libfido2.ErrNoCredentials
		}

		// TODO(codingllama): Verify f.wantRPID in here as well?
		//  We don't exercise this particular scenario presently, so it's not coded
		//  either.
	}

	// Block for user presence before accessing any credential data.
	if err := f.maybeLockUntilInteraction(opts.UP == libfido2.True); err != nil {
		return nil, err
	}

	// Does our explicitly set RPID match?
	// Used to simulate U2F App ID.
	if f.wantRPID != "" && f.wantRPID != rpID {
		return nil, libfido2.ErrNoCredentials
	}

	// Index credentialIDs for easier use.
	credIDs := make(map[string]struct{})
	for _, cred := range credentialIDs {
		credIDs[string(cred)] = struct{}{}

		// Simulate "internal error" on unknown credential handles.
		// Sometimes happens with Yubikeys firmware 4.1.8.
		// Requires a tap to happen.
		if f.errorOnUnknownCredential && !bytes.Equal(cred, f.key.KeyHandle) {
			return nil, fmt.Errorf("failed to get assertion: %w", libfido2.ErrInternal)
		}
	}

	// Assemble one assertion for each allowed credential we hold.
	var assertions []*libfido2.Assertion

	// "base" credential. Only add an assertion if explicitly requested.
	if _, ok := credIDs[string(f.key.KeyHandle)]; ok {
		// Simulate Yubikey4 and require UP, even if UP==false is set.
		if f.u2fOnly && opts.UP == libfido2.False {
			return nil, libfido2.ErrUserPresenceRequired
		}

		assertions = append(assertions, &libfido2.Assertion{
			AuthDataCBOR: assertionAuthDataCBOR,
			Sig:          assertionSig,
			CredentialID: f.key.KeyHandle,
			User:         libfido2.User{
				// We don't hold data about the user for the "base" credential / MFA
				// scenario.
				// A typical authenticator might choose to save some data within the
				// key handle itself.
			},
		})
	}

	// Resident credentials.
	if privilegedAccess {
		for _, resident := range f.credentials {
			allowed := len(credIDs) == 0
			if !allowed {
				_, allowed = credIDs[string(resident.ID)]
			}
			if !allowed {
				continue
			}
			assertions = append(assertions, &libfido2.Assertion{
				AuthDataCBOR: assertionAuthDataCBOR,
				Sig:          assertionSig,
				HMACSecret:   []byte{},
				CredentialID: resident.ID,
				User: libfido2.User{
					ID:          resident.User.ID,
					Name:        resident.User.Name,
					DisplayName: resident.User.DisplayName,
					Icon:        resident.User.Icon,
				},
			})
		}
	}

	switch len(assertions) {
	case 0:
		return nil, libfido2.ErrNoCredentials
	case 1:
		// Remove user name / display name / icon.
		// See the authenticatorGetAssertion response structure, user member (0x04):
		// https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html#authenticatorgetassertion-response-structure
		assertions[0].User.Name = ""
		assertions[0].User.DisplayName = ""
		assertions[0].User.Icon = ""
		return assertions, nil
	default:
		return assertions, nil
	}
}

type fakeTouchRequest struct {
	dev  *fakeFIDO2Device
	done bool // guarded by the device's lock
}

func (f *fakeFIDO2Device) TouchBegin() (wancli.TouchRequest, error) {
	return &fakeTouchRequest{dev: f}, nil
}

func (r *fakeTouchRequest) Status(timeout time.Duration) (touched bool, err error) {
	r.dev.cond.L.Lock()

	// Read/reset up.
	up := r.dev.up
	if up {
		r.dev.up = false
		r.done = true
	}

	// Read/reset cancel.
	cancel := r.dev.cancel
	if cancel {
		r.dev.cancel = false
		r.done = true
	}

	r.dev.cond.L.Unlock()

	if cancel {
		return false, libfido2.ErrKeepaliveCancel
	}
	if up {
		return true, nil
	}

	time.Sleep(1 * time.Millisecond) // Take a quick sleep to avoid tight loops.
	return false, nil
}

func (r *fakeTouchRequest) Stop() error {
	r.dev.cond.L.Lock()
	if r.done {
		r.dev.cond.L.Unlock()
		return nil
	}
	r.done = true
	r.dev.cond.L.Unlock()

	return r.dev.Cancel()
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

func (f *fakeFIDO2Device) hasClientPin() bool {
	return f.hasBoolOpt("clientPin")
}

func (f *fakeFIDO2Device) hasRK() bool {
	return f.hasBoolOpt("rk")
}

func (f *fakeFIDO2Device) hasUV() bool {
	return f.hasBoolOpt("uv")
}

func (f *fakeFIDO2Device) isBio() bool {
	return f.hasBoolOpt("bioEnroll")
}

func (f *fakeFIDO2Device) hasBoolOpt(name string) bool {
	if f.info == nil {
		return false
	}

	for _, opt := range f.info.Options {
		if opt.Name == name {
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
	f.cond.L.Lock()
	for !f.up && !f.cancel {
		f.cond.Wait()
	}
	defer f.cond.L.Unlock()

	// Record/reset state.
	isCancel := f.cancel
	f.up = false
	f.cancel = false

	if isCancel {
		return libfido2.ErrKeepaliveCancel
	}
	return nil
}
