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

package touchid_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func init() {
	// Make tests silent.
	touchid.PromptWriter = io.Discard
}

type simplePicker struct{}

func (p simplePicker) PromptCredential(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
	return creds[0], nil
}

func TestRegisterAndLogin(t *testing.T) {
	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	const llamaUser = "llama"

	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Teleport",
		RPID:          "teleport",
		RPOrigins:     []string{"https://goteleport.com"},
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		webUser         *fakeUser
		origin, user    string
		modifyAssertion func(a *wantypes.CredentialAssertion)
		wantUser        string
	}{
		{
			name:    "passwordless",
			webUser: &fakeUser{id: []byte{1, 2, 3, 4, 5}, name: llamaUser},
			origin:  web.Config.RPOrigins[0],
			modifyAssertion: func(a *wantypes.CredentialAssertion) {
				a.Response.AllowedCredentials = nil // aka passwordless
			},
			wantUser: llamaUser,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeNative{}
			*touchid.Native = fake

			webUser := test.webUser
			origin := test.origin
			user := test.user

			// Registration section.
			cc, sessionData, err := web.BeginRegistration(webUser)
			require.NoError(t, err)

			reg, err := touchid.Register(origin, wantypes.CredentialCreationFromProtocol(cc))
			require.NoError(t, err, "Register failed")
			assert.Equal(t, 1, fake.userPrompts, "unexpected number of Registration prompts")

			// We have to marshal and parse ccr due to an unavoidable quirk of the
			// webauthn API.
			body, err := json.Marshal(reg.CCR)
			require.NoError(t, err)
			parsedCCR, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
			require.NoError(t, err, "ParseCredentialCreationResponseBody failed")

			cred, err := web.CreateCredential(webUser, *sessionData, parsedCCR)
			require.NoError(t, err, "CreateCredential failed")
			// Save credential for Login test below.
			webUser.credentials = append(webUser.credentials, *cred)

			// Confirm client-side registration.
			require.NoError(t, reg.Confirm())

			// Login section.
			a, sessionData, err := web.BeginLogin(webUser)
			require.NoError(t, err, "BeginLogin failed")
			assertion := wantypes.CredentialAssertionFromProtocol(a)
			test.modifyAssertion(assertion)

			assertionResp, actualUser, err := touchid.Login(origin, user, assertion, simplePicker{})
			require.NoError(t, err, "Login failed")
			assert.Equal(t, test.wantUser, actualUser, "actualUser mismatch")
			assert.Equal(t, 2, fake.userPrompts, "unexpected number of Login prompts")

			// Same as above: easiest way to validate the assertion is to marshal
			// and then parse the body.
			body, err = json.Marshal(assertionResp)
			require.NoError(t, err)
			parsedAssertion, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
			require.NoError(t, err, "ParseCredentialRequestResponseBody failed")

			_, err = web.ValidateLogin(webUser, *sessionData, parsedAssertion)
			require.NoError(t, err, "ValidatLogin failed")
		})
	}
}

func TestRegister_rollback(t *testing.T) {
	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	fake := &fakeNative{}
	*touchid.Native = fake

	// WebAuthn and CredentialCreation setup.
	const llamaUser = "llama"
	const origin = "https://goteleport.com"
	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Teleport",
		RPID:          "teleport",
		RPOrigins:     []string{origin},
	})
	require.NoError(t, err)
	cc, _, err := web.BeginRegistration(&fakeUser{
		id:   []byte{1, 2, 3, 4, 5},
		name: llamaUser,
	})
	require.NoError(t, err)

	// Register and then Rollback a credential.
	reg, err := touchid.Register(origin, wantypes.CredentialCreationFromProtocol(cc))
	require.NoError(t, err, "Register failed")
	require.NoError(t, reg.Rollback(), "Rollback failed")

	// Verify non-interactive deletion in fake.
	require.Contains(t, fake.nonInteractiveDelete, reg.CCR.ID, "Credential ID not found in (fake) nonInteractiveDeletes")

	// Attempt to authenticate.
	_, _, err = touchid.Login(origin, llamaUser, &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // doesn't matter as long as it's not empty
			RelyingPartyID:   web.Config.RPID,
			UserVerification: "required",
		},
	}, simplePicker{})
	require.Equal(t, touchid.ErrCredentialNotFound, err, "unexpected Login error")
}

func TestLogin_findsCorrectCredential(t *testing.T) {
	// The "correct" login credential is the newest credential for the specified
	// user
	// In case of MFA, it's the "newest" allowed credential.
	// In case of Passwordless, it's the newest credential.
	// Credentials from different users shouldn't mix together.

	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	var timeCounter int64
	fake := &fakeNative{
		timeNow: func() time.Time {
			timeCounter++
			return time.Unix(timeCounter, 0)
		},
	}
	*touchid.Native = fake

	// Users.
	userLlama := &fakeUser{
		id:   []byte{1, 1, 1, 1, 1},
		name: "llama",
	}
	userAlpaca := &fakeUser{
		id:   []byte{1, 1, 1, 1, 2},
		name: "alpaca",
	}

	// WebAuthn setup.
	const origin = "https://goteleport.com"
	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Teleport",
		RPID:          "teleport",
		RPOrigins:     []string{origin},
	})
	require.NoError(t, err)

	// Credential setup, in temporal order.
	for i, u := range []*fakeUser{userAlpaca, userLlama, userLlama, userLlama, userAlpaca} {
		cc, _, err := web.BeginRegistration(u)
		require.NoError(t, err, "BeginRegistration #%v failed, user %v", i+1, u.name)

		reg, err := touchid.Register(origin, wantypes.CredentialCreationFromProtocol(cc))
		require.NoError(t, err, "Register #%v failed, user %v", i+1, u.name)
		require.NoError(t, reg.Confirm(), "Confirm failed")
	}

	// Register a few credentials for a second RPID.
	// If everything is correct this shouldn't interfere with the test, despite
	// happening last.
	web2, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "TeleportO",
		RPID:          "teleportO",
		RPOrigins:     []string{"https://goteleportO.com"},
	})
	require.NoError(t, err)
	for _, u := range []*fakeUser{userAlpaca, userLlama} {
		cc, _, err := web2.BeginRegistration(u)
		require.NoError(t, err, "web2 BeginRegistration failed")

		reg, err := touchid.Register(web2.Config.RPOrigins[0], wantypes.CredentialCreationFromProtocol(cc))
		require.NoError(t, err, "web2 Register failed")
		require.NoError(t, reg.Confirm(), "Confirm failed")
	}

	require.GreaterOrEqual(t, len(fake.creds), 5, "creds len sanity check failed")
	alpaca1 := fake.creds[0]
	llama1 := fake.creds[1]
	llama2 := fake.creds[2]
	llama3 := fake.creds[3]
	alpaca2 := fake.creds[4]
	// Log credentials so it's possible to understand eventual test failures.
	t.Logf("llama1 = %v", llama1)
	t.Logf("llama2 = %v", llama2)
	t.Logf("llama3 = %v", llama3)
	t.Logf("alpaca1 = %v", alpaca1)
	t.Logf("alpaca2 = %v", alpaca2)

	// All tests run against the "web" configuration.
	tests := []struct {
		name         string
		user         string
		allowedCreds []credentialHandle
		// wantUser is only present if it's different from user.
		wantUser       string
		wantCredential string
	}{
		{
			name:           "prefers newer credential (alpaca)",
			user:           userAlpaca.name,
			wantCredential: alpaca2.id,
		},
		{
			name:           "prefers newer credential (llama)",
			user:           userLlama.name,
			wantCredential: llama3.id,
		},
		{
			name:           "prefers newer credential (no user)",
			wantUser:       userAlpaca.name,
			wantCredential: alpaca2.id,
		},
		{
			name:           "allowed credentials first",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama1},
			wantCredential: llama1.id,
		},
		{
			name:           "allowed credentials second",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama1, llama2},
			wantCredential: llama2.id,
		},
		{
			name:           "allowed credentials last (1)",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama1, llama2, llama3},
			wantCredential: llama3.id,
		},
		{
			name:           "allowed credentials last (2)",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama2, llama3},
			wantCredential: llama3.id,
		},
		{
			name:           "allowed credentials last (3)",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama1, llama3},
			wantCredential: llama3.id,
		},
		{
			name:           "allowed credentials last (4)",
			user:           userLlama.name,
			allowedCreds:   []credentialHandle{llama3},
			wantCredential: llama3.id,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var allowedCreds []wantypes.CredentialDescriptor
			for _, cred := range test.allowedCreds {
				allowedCreds = append(allowedCreds, wantypes.CredentialDescriptor{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: []byte(cred.id),
				})
			}

			_, gotUser, err := touchid.Login(origin, test.user, &wantypes.CredentialAssertion{
				Response: wantypes.PublicKeyCredentialRequestOptions{
					Challenge:          []byte{1, 2, 3, 4, 5}, // arbitrary
					RelyingPartyID:     web.Config.RPID,
					AllowedCredentials: allowedCreds,
				},
			}, simplePicker{})
			require.NoError(t, err, "Login failed")

			wantUser := test.wantUser
			if wantUser == "" {
				wantUser = test.user
			}
			assert.Equal(t, wantUser, gotUser, "Login user mismatch")
			assert.Equal(t, test.wantCredential, fake.lastAuthnCredential, "Login credential mismatch")
		})
	}
}

func TestLogin_noCredentials_failsWithoutUserInteraction(t *testing.T) {
	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	fake := &fakeNative{}
	*touchid.Native = fake

	const origin = "https://goteleport.com"
	baseAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID:   "goteleport.com",
			UserVerification: protocol.VerificationRequired,
		},
	}
	mfaAssertion := *baseAssertion
	mfaAssertion.Response.UserVerification = protocol.VerificationDiscouraged
	mfaAssertion.Response.AllowedCredentials = []wantypes.CredentialDescriptor{
		{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: []byte{1, 2, 3, 4, 5}, // arbitrary
		},
	}

	// Run empty credentials tests first.
	for _, test := range []struct {
		name      string
		user      string
		assertion *wantypes.CredentialAssertion
	}{
		{
			name:      "passwordless empty user",
			assertion: baseAssertion,
		},
		{
			name:      "passwordless explicit user",
			user:      "llama",
			assertion: baseAssertion,
		},
		{
			name:      "MFA empty user",
			user:      "", // Typically MFA comes with an empty user
			assertion: &mfaAssertion,
		},
		{
			name:      "MFA explicit user",
			user:      "llama",
			assertion: &mfaAssertion,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake.userPrompts = 0 // reset before test
			_, _, err := touchid.Login(origin, test.user, test.assertion, simplePicker{})
			assert.ErrorIs(t, err, touchid.ErrCredentialNotFound, "Login error mismatch")
			assert.Zero(t, fake.userPrompts, "Login caused user interaction with no credentials")
		})
	}

	// Register a couple of credentials for the following tests.
	const userLlama = "llama"
	const userAlpaca = "alpaca"
	rrk := true
	cc1 := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: []byte{1, 2, 3, 4, 5}, // arbitrary, not important here
			RelyingParty: wantypes.RelyingPartyEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "Teleport",
				},
				ID: baseAssertion.Response.RelyingPartyID,
			},
			User: wantypes.UserEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: userLlama,
				},
				DisplayName: "Llama",
				ID:          []byte{1, 1, 1, 1, 1},
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				RequireResidentKey: &rrk,
				UserVerification:   protocol.VerificationRequired,
			},
			Attestation: protocol.PreferDirectAttestation,
		},
	}
	cc2 := *cc1
	cc2.Response.User = wantypes.UserEntity{
		CredentialEntity: wantypes.CredentialEntity{
			Name: userAlpaca,
		},
		DisplayName: "Alpaca",
		ID:          []byte{1, 1, 1, 1, 2},
	}
	for _, cc := range []*wantypes.CredentialCreation{cc1, &cc2} {
		reg, err := touchid.Register(origin, cc)
		require.NoError(t, err, "Register failed")
		require.NoError(t, reg.Confirm(), "Confirm failed")
	}

	mfaAllCreds := mfaAssertion
	mfaAllCreds.Response.AllowedCredentials = nil
	for _, c := range fake.creds {
		mfaAllCreds.Response.AllowedCredentials = append(mfaAllCreds.Response.AllowedCredentials, wantypes.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: []byte(c.id),
		})
	}

	// Test absence of user prompts with existing credentials.
	for _, test := range []struct {
		name      string
		user      string
		assertion *wantypes.CredentialAssertion
	}{
		{
			name:      "passwordless existing credentials",
			user:      "camel", // not registered
			assertion: baseAssertion,
		},
		{
			name:      "MFA unknown credential IDs (1)",
			user:      "",            // any user
			assertion: &mfaAssertion, // missing correct credential IDs
		},
		{
			name:      "MFA unknown credential IDs (2)",
			user:      userLlama,     // known user
			assertion: &mfaAssertion, // missing correct credential IDs
		},
		{
			name:      "MFA credentials for another user",
			user:      "camel",      // unknown user
			assertion: &mfaAllCreds, // credential IDs correct but for other users
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake.userPrompts = 0 // reset before test
			_, _, err := touchid.Login(origin, test.user, test.assertion, simplePicker{})
			assert.ErrorIs(t, err, touchid.ErrCredentialNotFound, "Login error mismatch")
			assert.Zero(t, fake.userPrompts, "Login caused user interaction with no credentials")
		})
	}
}

type funcToPicker func([]*touchid.CredentialInfo) (*touchid.CredentialInfo, error)

func (f funcToPicker) PromptCredential(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
	return f(creds)
}

func pickByName(name string) func([]*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
	return func(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
		for _, c := range creds {
			if c.User.Name == name {
				return c, nil
			}
		}
		return nil, fmt.Errorf("user %v not found", name)
	}
}

func TestLogin_credentialPicker(t *testing.T) {
	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	// Use monotonically-increasing time.
	// Newer credentials are preferred.
	var timeCounter int64
	fake := &fakeNative{
		timeNow: func() time.Time {
			timeCounter++
			return time.Unix(timeCounter, 0)
		},
	}
	*touchid.Native = fake

	const rpID = "goteleport.com"
	const origin = "https://goteleport.com"
	baseAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // arbitrary
			RelyingPartyID:   rpID,
			UserVerification: protocol.VerificationRequired,
		},
	}
	newAssertion := func(allowedCreds [][]byte) *wantypes.CredentialAssertion {
		cp := *baseAssertion
		for _, id := range allowedCreds {
			cp.Response.AllowedCredentials = append(cp.Response.AllowedCredentials, wantypes.CredentialDescriptor{
				Type:         protocol.PublicKeyCredentialType,
				CredentialID: id,
			})
		}
		return &cp
	}

	// Test results vary depending on registered credentials, so instead of a
	// single table we'll build scenarios little-by-litte.
	type pickerTest struct {
		name         string
		allowedCreds [][]byte
		user         string
		picker       func([]*touchid.CredentialInfo) (*touchid.CredentialInfo, error)
		wantID       string
		wantUser     string
	}
	runTests := func(t *testing.T, tests []pickerTest) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				fake.userPrompts = 0 // reset before test

				assertion := newAssertion(test.allowedCreds)
				picker := funcToPicker(test.picker)

				resp, actualUser, err := touchid.Login(origin, test.user, assertion, picker)
				require.NoError(t, err, "Login failed")
				assert.Equal(t, test.wantID, resp.ID, "credential ID mismatch")
				assert.Equal(t, test.wantUser, actualUser, "user mismatch")
				assert.Equal(t, 1, fake.userPrompts, "Login prompted an unexpected number of times")
			})
		}
	}

	const llamaUser = "llama"
	llamaHandle := []byte{1, 1, 1, 1, 1}
	const alpacaUser = "alpaca"
	alpacaHandle := []byte{1, 1, 1, 1, 2}
	const camelUser = "camel"
	camelHandle := []byte{1, 1, 1, 1, 3}

	// Single user, single credential.
	llama1, err := fake.Register(rpID, llamaUser, llamaHandle)
	require.NoError(t, err)
	runTests(t, []pickerTest{
		{
			name:     "single user, single credential, empty user",
			wantID:   llama1.CredentialID,
			wantUser: llamaUser,
		},
		{
			name:     "single user, single credential, explicit user",
			user:     llamaUser,
			wantID:   llama1.CredentialID,
			wantUser: llamaUser,
		},
		{
			name: "MFA single credential",
			allowedCreds: [][]byte{
				[]byte(llama1.CredentialID),
			},
			user:     llamaUser,
			wantID:   llama1.CredentialID,
			wantUser: llamaUser,
		},
	})

	// Single user, multi credentials.
	llama2, err := fake.Register(rpID, llamaUser, llamaHandle)
	_ = llama2 // unused apart from registration
	require.NoError(t, err)
	llama3, err := fake.Register(rpID, llamaUser, llamaHandle)
	require.NoError(t, err)
	runTests(t, []pickerTest{
		{
			name:     "single user, multi credential, empty user",
			wantID:   llama3.CredentialID, // latest registered credential
			wantUser: llamaUser,
		},
		{
			name:     "single user, multi credential, explicit user",
			user:     llamaUser,
			wantID:   llama3.CredentialID,
			wantUser: llamaUser,
		},
	})

	// Multi user, multi credentials.
	alpaca1, err := fake.Register(rpID, alpacaUser, alpacaHandle)
	require.NoError(t, err)
	camel1, err := fake.Register(rpID, camelUser, camelHandle)
	require.NoError(t, err)
	camel2, err := fake.Register(rpID, camelUser, camelHandle)
	require.NoError(t, err)
	runTests(t, []pickerTest{
		{
			name:     "multi user, multi credential, explicit user (1)",
			user:     llamaUser,
			wantID:   llama3.CredentialID, // latest credential for llama
			wantUser: llamaUser,
		},
		{
			name:     "multi user, multi credential, explicit user (2)",
			user:     camelUser,
			wantID:   camel2.CredentialID, // latest credential for camel
			wantUser: camelUser,
		},
		{
			name:     "credential picker (1)",
			picker:   pickByName(llamaUser),
			wantID:   llama3.CredentialID,
			wantUser: llamaUser,
		},
		{
			name:     "credential picker (2)",
			picker:   pickByName(alpacaUser),
			wantID:   alpaca1.CredentialID,
			wantUser: alpacaUser,
		},
		{
			name: "MFA multiple credentials (1)",
			allowedCreds: [][]byte{
				[]byte(llama1.CredentialID),
				[]byte(camel1.CredentialID),
			},
			user:     llamaUser,
			wantID:   llama1.CredentialID,
			wantUser: llamaUser,
		},
		{
			name: "MFA multiple credentials (2)",
			allowedCreds: [][]byte{
				[]byte(llama1.CredentialID),
				[]byte(camel1.CredentialID),
			},
			user:     camelUser,
			wantID:   camel1.CredentialID,
			wantUser: camelUser,
		},
	})

	// Verify that deduping is working as expected.
	// Tests above already cover all users.
	t.Run("number of credentials is correct", func(t *testing.T) {
		picker := funcToPicker(func(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
			// 3 = llama + alpaca + camel
			assert.Len(t, creds, 3, "unexpected number of picker credentials")
			return pickByName(llamaUser)(creds)
		})

		_, actualUser, err := touchid.Login(origin, "" /* user */, baseAssertion, picker)
		assert.NoError(t, err, "Login failed")
		assert.Equal(t, llamaUser, actualUser, "Login user mismatch")
	})

	errUnexpected := errors.New("the llamas escaped")

	// Finally, let's take advantage of the complete setup and run a few error
	// tests.
	for _, test := range []struct {
		name         string
		allowedCreds [][]byte
		user         string
		picker       func([]*touchid.CredentialInfo) (*touchid.CredentialInfo, error)
		// At least one of wantErr or wantMsg should be supplied.
		wantErr error
		wantMsg string
	}{
		{
			name: "credential picker error",
			picker: func(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
				return nil, errUnexpected
			},
			wantErr: errUnexpected,
		},
		{
			name: "credential picker returns bad pointer",
			picker: func(creds []*touchid.CredentialInfo) (*touchid.CredentialInfo, error) {
				// Returned pointer not part of creds.
				return &touchid.CredentialInfo{
					CredentialID: creds[0].CredentialID,
					User:         creds[0].User,
				}, nil
			},
			wantMsg: "returned invalid credential",
		},
		{
			name:    "unknown user requested",
			user:    "whoami",
			wantErr: touchid.ErrCredentialNotFound,
		},
		{
			name: "MFA no credentials allowed",
			allowedCreds: [][]byte{
				[]byte("notme"),
				[]byte("alsonotme"),
			},
			user:    llamaUser,
			wantErr: touchid.ErrCredentialNotFound,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.True(t, test.wantErr != nil || test.wantMsg != "", "Sanity check failed")

			assertion := newAssertion(test.allowedCreds)
			picker := funcToPicker(test.picker)

			_, _, err := touchid.Login(origin, test.user, assertion, picker)
			require.Error(t, err, "Login succeeded unexpectedly")
			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr, "Login error mismatch")
			}
			if test.wantMsg != "" {
				assert.ErrorContains(t, err, test.wantMsg, "Login error mismatch")
			}
		})
	}
}

type credentialHandle struct {
	rpID, user string
	id         string
	userHandle []byte
	createTime time.Time
	key        *ecdsa.PrivateKey
}

type fakeNative struct {
	timeNow              func() time.Time
	creds                []credentialHandle
	nonInteractiveDelete []string

	// lastAuthnCredential is the last credential ID used in a successful
	// Authenticate call.
	lastAuthnCredential string

	// userPrompts counts the number of user-visible prompts that would be caused
	// by various methods.
	userPrompts int
}

func (f *fakeNative) Diag() (*touchid.DiagResult, error) {
	return &touchid.DiagResult{
		HasCompileSupport:       true,
		HasSignature:            true,
		HasEntitlements:         true,
		PassedLAPolicyTest:      true,
		PassedSecureEnclaveTest: true,
		IsAvailable:             true,
	}, nil
}

type fakeAuthContext struct {
	countPrompts func(ctx touchid.AuthContext)
	prompted     bool
}

func (c *fakeAuthContext) Guard(fn func()) error {
	c.countPrompts(c)
	fn()
	return nil
}

func (c *fakeAuthContext) Close() {
	c.prompted = false
}

func (f *fakeNative) NewAuthContext() touchid.AuthContext {
	return &fakeAuthContext{
		countPrompts: f.countPrompts,
	}
}

func (f *fakeNative) Authenticate(actx touchid.AuthContext, credentialID string, data []byte) ([]byte, error) {
	var key *ecdsa.PrivateKey
	for _, cred := range f.creds {
		if cred.id == credentialID {
			key = cred.key
			break
		}
	}
	if key == nil {
		return nil, touchid.ErrCredentialNotFound
	}

	f.countPrompts(actx)
	sig, err := key.Sign(rand.Reader, data, crypto.SHA256)
	if err != nil {
		return nil, err
	}
	f.lastAuthnCredential = credentialID
	return sig, nil
}

func (f *fakeNative) countPrompts(actx touchid.AuthContext) {
	switch c, ok := actx.(*fakeAuthContext); {
	case ok && c.prompted:
		return // Already prompted
	case ok:
		c.prompted = true
		fallthrough
	default:
		f.userPrompts++
	}
}

func (f *fakeNative) DeleteCredential(credentialID string) error {
	return errors.New("not implemented")
}

func (f *fakeNative) DeleteNonInteractive(credentialID string) error {
	for i, cred := range f.creds {
		if cred.id != credentialID {
			continue
		}
		f.nonInteractiveDelete = append(f.nonInteractiveDelete, credentialID)
		f.creds = append(f.creds[:i], f.creds[i+1:]...)
		return nil
	}
	return touchid.ErrCredentialNotFound
}

func (f *fakeNative) FindCredentials(rpID, user string) ([]touchid.CredentialInfo, error) {
	var resp []touchid.CredentialInfo
	for _, cred := range f.creds {
		if cred.rpID == rpID && (user == "" || cred.user == user) {
			resp = append(resp, touchid.CredentialInfo{
				CredentialID: cred.id,
				RPID:         cred.rpID,
				User: touchid.UserInfo{
					UserHandle: cred.userHandle,
					Name:       cred.user,
				},
				PublicKey:  &cred.key.PublicKey,
				CreateTime: cred.createTime,
			})
		}
	}
	return resp, nil
}

func (f *fakeNative) ListCredentials() ([]touchid.CredentialInfo, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeNative) Register(rpID, user string, userHandle []byte) (*touchid.CredentialInfo, error) {
	key, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	cred := credentialHandle{
		rpID:       rpID,
		user:       user,
		id:         id,
		userHandle: userHandle,
		key:        key,
	}
	if f.timeNow != nil {
		cred.createTime = f.timeNow()
	}
	f.creds = append(f.creds, cred)

	// Marshal key into the raw Apple format.
	pubKeyApple := make([]byte, 1+32+32)
	pubKeyApple[0] = 0x04
	key.X.FillBytes(pubKeyApple[1:33])
	key.Y.FillBytes(pubKeyApple[33:])

	info := &touchid.CredentialInfo{
		CredentialID: id,
	}
	info.SetPublicKeyRaw(pubKeyApple)
	return info, nil
}

type fakeUser struct {
	id          []byte
	name        string
	credentials []webauthn.Credential
}

func (u *fakeUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (u *fakeUser) WebAuthnDisplayName() string {
	return u.name
}

func (u *fakeUser) WebAuthnID() []byte {
	return u.id
}

func (u *fakeUser) WebAuthnIcon() string {
	return ""
}

func (u *fakeUser) WebAuthnName() string {
	return u.name
}
