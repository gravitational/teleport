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

package touchid_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/auth/touchid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestRegisterAndLogin(t *testing.T) {
	n := *touchid.Native
	t.Cleanup(func() {
		*touchid.Native = n
	})

	const llamaUser = "llama"

	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Teleport",
		RPID:          "teleport",
		RPOrigin:      "https://goteleport.com",
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		webUser         *fakeUser
		origin, user    string
		modifyAssertion func(a *wanlib.CredentialAssertion)
		wantUser        string
	}{
		{
			name:    "passwordless",
			webUser: &fakeUser{id: []byte{1, 2, 3, 4, 5}, name: llamaUser},
			origin:  web.Config.RPOrigin,
			modifyAssertion: func(a *wanlib.CredentialAssertion) {
				a.Response.AllowedCredentials = nil // aka passwordless
			},
			wantUser: llamaUser,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			*touchid.Native = &fakeNative{}

			webUser := test.webUser
			origin := test.origin
			user := test.user

			// Registration section.
			cc, sessionData, err := web.BeginRegistration(webUser)
			require.NoError(t, err)

			reg, err := touchid.Register(origin, (*wanlib.CredentialCreation)(cc))
			require.NoError(t, err, "Register failed")

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
			assertion := (*wanlib.CredentialAssertion)(a)
			test.modifyAssertion(assertion)

			assertionResp, actualUser, err := touchid.Login(origin, user, assertion)
			require.NoError(t, err, "Login failed")
			assert.Equal(t, test.wantUser, actualUser, "actualUser mismatch")

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
		RPOrigin:      origin,
	})
	require.NoError(t, err)
	cc, _, err := web.BeginRegistration(&fakeUser{
		id:   []byte{1, 2, 3, 4, 5},
		name: llamaUser,
	})
	require.NoError(t, err)

	// Register and then Rollback a credential.
	reg, err := touchid.Register(origin, (*wanlib.CredentialCreation)(cc))
	require.NoError(t, err, "Register failed")
	require.NoError(t, reg.Rollback(), "Rollback failed")

	// Verify non-interactive deletion in fake.
	require.Contains(t, fake.nonInteractiveDelete, reg.CCR.ID, "Credential ID not found in (fake) nonInteractiveDeletes")

	// Attempt to authenticate.
	_, _, err = touchid.Login(origin, llamaUser, &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:        []byte{1, 2, 3, 4, 5}, // doesn't matter as long as it's not empty
			RelyingPartyID:   web.Config.RPID,
			UserVerification: "required",
		},
	})
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
		RPOrigin:      origin,
	})
	require.NoError(t, err)

	// Credential setup, in temporal order.
	for i, u := range []*fakeUser{userAlpaca, userLlama, userLlama, userLlama, userAlpaca} {
		cc, _, err := web.BeginRegistration(u)
		require.NoError(t, err, "BeginRegistration #%v failed, user %v", i+1, u.name)

		reg, err := touchid.Register(origin, (*wanlib.CredentialCreation)(cc))
		require.NoError(t, err, "Register #%v failed, user %v", i+1, u.name)
		require.NoError(t, reg.Confirm(), "Confirm failed")
	}

	// Register a few credentials for a second RPID.
	// If everything is correct this shouldn't interfere with the test, despite
	// happening last.
	web2, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "TeleportO",
		RPID:          "teleportO",
		RPOrigin:      "https://goteleportO.com",
	})
	require.NoError(t, err)
	for _, u := range []*fakeUser{userAlpaca, userLlama} {
		cc, _, err := web2.BeginRegistration(u)
		require.NoError(t, err, "web2 BeginRegistration failed")

		reg, err := touchid.Register(web2.Config.RPOrigin, (*wanlib.CredentialCreation)(cc))
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
			var allowedCreds []protocol.CredentialDescriptor
			for _, cred := range test.allowedCreds {
				allowedCreds = append(allowedCreds, protocol.CredentialDescriptor{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: []byte(cred.id),
				})
			}

			_, gotUser, err := touchid.Login(origin, test.user, &wanlib.CredentialAssertion{
				Response: protocol.PublicKeyCredentialRequestOptions{
					Challenge:          []byte{1, 2, 3, 4, 5}, // arbitrary
					RelyingPartyID:     web.Config.RPID,
					AllowedCredentials: allowedCreds,
				},
			})
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

func (f *fakeNative) Authenticate(credentialID string, data []byte) ([]byte, error) {
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

	sig, err := key.Sign(rand.Reader, data, crypto.SHA256)
	if err != nil {
		return nil, err
	}
	f.lastAuthnCredential = credentialID
	return sig, nil
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
				UserHandle:   cred.userHandle,
				CredentialID: cred.id,
				RPID:         cred.rpID,
				User:         cred.user,
				PublicKey:    &cred.key.PublicKey,
				CreateTime:   cred.createTime,
			})
		}
	}
	return resp, nil
}

func (f *fakeNative) ListCredentials() ([]touchid.CredentialInfo, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeNative) Register(rpID, user string, userHandle []byte) (*touchid.CredentialInfo, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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
