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

package webauthn_test

import (
	"bytes"
	"context"
	"encoding/pem"
	"sort"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestRegistrationFlow_BeginFinish(t *testing.T) {
	const user = "llama"
	const rpID = "localhost"
	const origin = "https://localhost"
	identity := newFakeIdentity(user)
	webRegistration := wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
		Identity: identity,
	}

	ctx := context.Background()
	tests := []struct {
		name                 string
		user                 string
		deviceName           string
		passwordless         bool
		wantUserVerification protocol.UserVerificationRequirement
		wantResidentKey      bool
	}{
		{
			name:                 "MFA",
			user:                 user,
			deviceName:           "webauthn1",
			wantUserVerification: protocol.VerificationDiscouraged,
		},
		{
			name:                 "passwordless",
			user:                 user,
			deviceName:           "webauthn2",
			passwordless:         true,
			wantUserVerification: protocol.VerificationRequired,
			wantResidentKey:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity.UpdatedDevices = nil // Clear "stored" devices

			// Begin is the first step in registration.
			credentialCreation, err := webRegistration.Begin(ctx, user, test.passwordless)
			require.NoError(t, err)
			// Assert some parts of the credentialCreation (it's framework-created, no
			// need to go too deep).
			require.NotNil(t, credentialCreation)
			require.NotEmpty(t, credentialCreation.Response.Challenge)
			require.Equal(t, webRegistration.Webauthn.RPID, credentialCreation.Response.RelyingParty.ID)
			// Are we using the correct authenticator selection settings?
			require.Equal(t, test.wantResidentKey, *credentialCreation.Response.AuthenticatorSelection.RequireResidentKey)
			if test.wantResidentKey {
				require.Equal(t, protocol.ResidentKeyRequirementRequired, credentialCreation.Response.AuthenticatorSelection.ResidentKey)
			} else {
				require.Equal(t, protocol.ResidentKeyRequirementDiscouraged, credentialCreation.Response.AuthenticatorSelection.ResidentKey)
			}
			require.Equal(t, test.wantUserVerification, credentialCreation.Response.AuthenticatorSelection.UserVerification)
			// Did we record the SessionData in storage?
			require.NotEmpty(t, identity.SessionData)

			// Sign CredentialCreation, typically requires user interaction.
			dev, err := mocku2f.Create()
			require.NoError(t, err)
			dev.SetUV = test.wantUserVerification == protocol.VerificationRequired
			dev.AllowResidentKey = test.wantResidentKey
			ccr, err := dev.SignCredentialCreation(origin, credentialCreation)
			require.NoError(t, err)

			// Finish is the final step in registration.
			newDevice, err := webRegistration.Finish(ctx, wanlib.RegisterResponse{
				User:             user,
				DeviceName:       test.deviceName,
				CreationResponse: ccr,
				Passwordless:     test.passwordless,
			})
			require.NoError(t, err)
			require.Equal(t, test.deviceName, newDevice.GetName())
			// Did we get a proper WebauthnDevice?
			gotDevice := newDevice.GetWebauthn()
			require.NotNil(t, gotDevice)
			require.NotEmpty(t, gotDevice.PublicKeyCbor) // validated indirectly via authentication
			wantDevice := &types.WebauthnDevice{
				CredentialId:             dev.KeyHandle,
				PublicKeyCbor:            gotDevice.PublicKeyCbor,
				AttestationType:          gotDevice.AttestationType,
				Aaguid:                   make([]byte, 16), // 16 zeroes
				SignatureCounter:         0,
				AttestationObject:        ccr.AttestationResponse.AttestationObject,
				ResidentKey:              test.wantResidentKey,
				CredentialRpId:           rpID,
				CredentialBackupEligible: &gogotypes.BoolValue{Value: false},
				CredentialBackedUp:       &gogotypes.BoolValue{Value: false},
			}
			if diff := cmp.Diff(wantDevice, gotDevice); diff != "" {
				t.Errorf("Finish() mismatch (-want +got):\n%s", diff)
			}
			// SessionData was cleared?
			require.Empty(t, identity.SessionData)
			// Device created in storage?
			require.Len(t, identity.UpdatedDevices, 1)
			require.Equal(t, newDevice, identity.UpdatedDevices[0])
		})
	}
}

func TestRegistrationFlow_Begin_excludeList(t *testing.T) {
	const user = "llama"
	const rpID = "localhost"

	u2fID := []byte{1, 1, 1}     // U2F
	mfaID := []byte{1, 1, 2}     // WebAuthn / MFA
	passkeyID := []byte{1, 1, 3} // WebAuthn / passwordless
	u2fDev := &types.MFADevice{
		Device: &types.MFADevice_U2F{
			U2F: &types.U2FDevice{
				KeyHandle: u2fID,
			},
		},
	}
	mfaDev := &types.MFADevice{
		Device: &types.MFADevice_Webauthn{
			Webauthn: &types.WebauthnDevice{
				CredentialId: mfaID,
			},
		},
	}
	passkeyDev := &types.MFADevice{
		Device: &types.MFADevice_Webauthn{
			Webauthn: &types.WebauthnDevice{
				CredentialId: passkeyID,
				ResidentKey:  true,
			},
		},
	}
	identity := newFakeIdentity(user, u2fDev, mfaDev, passkeyDev)

	rf := wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
		Identity: identity,
	}

	ctx := context.Background()
	tests := []struct {
		name            string
		passwordless    bool
		wantExcludeList [][]byte
	}{
		{
			name: "MFA",
			wantExcludeList: [][]byte{
				mfaID,
				passkeyID, // Prevents "downgrades"
			},
		},
		{
			name:         "passwordless",
			passwordless: true,
			wantExcludeList: [][]byte{
				passkeyID,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cc, err := rf.Begin(ctx, user, test.passwordless)
			require.NoError(t, err, "Begin")

			got := cc.Response.CredentialExcludeList
			sort.Slice(got, func(i, j int) bool {
				return bytes.Compare(got[i].CredentialID, got[j].CredentialID) == -1
			})

			want := make([]wantypes.CredentialDescriptor, len(test.wantExcludeList))
			for i, id := range test.wantExcludeList {
				want[i] = wantypes.CredentialDescriptor{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: id,
				}
			}
			sort.Slice(want, func(i, j int) bool {
				return bytes.Compare(want[i].CredentialID, want[j].CredentialID) == -1
			})

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Begin() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRegistrationFlow_Begin_webID(t *testing.T) {
	const rpID = "localhost"
	ctx := context.Background()

	alpacaWebID := []byte{1, 2, 3, 4, 5}
	tests := []struct {
		name             string
		user, mappedUser string
		wla              *types.WebauthnLocalAuth
	}{
		{
			name: "user without webID", // first registration or U2F user
			user: "llama",
		},
		{
			name:       "user without webID mapping", // aka legacy or inconsistent storage
			user:       "alpaca",
			mappedUser: "", // missing mapping
			wla:        &types.WebauthnLocalAuth{UserID: alpacaWebID},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := test.user

			// Prepare identity/user according to test parameters.
			identity := newFakeIdentity(user)
			if test.wla != nil {
				err := identity.UpsertWebauthnLocalAuth(ctx, user, test.wla)
				require.NoError(t, err, "failed to upsert WebauthnLocalAuth")
			}
			identity.MappedUser = test.mappedUser

			// Begin registration; this should create/fix the webID and webID->user
			// mappings in storage.
			webRegistration := &wanlib.RegistrationFlow{
				Webauthn: &types.Webauthn{RPID: rpID},
				Identity: identity,
			}
			_, err := webRegistration.Begin(ctx, user, false /* passwordless */)
			require.NoError(t, err, "Begin failed")

			// Verify that we have both the webID and the correct webID->user
			// mapping.
			wla, err := identity.GetWebauthnLocalAuth(ctx, user)
			require.NoError(t, err, "failed to read WebauthnLocalAuth")
			require.NotEmpty(t, wla.UserID)

			gotUser, err := identity.GetTeleportUserByWebauthnID(ctx, wla.UserID)
			require.NoError(t, err, "failed to get user from webID")
			require.Equal(t, user, gotUser)
		})
	}
}

func TestRegistrationFlow_Begin_errors(t *testing.T) {
	const user = "llama"
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		Identity: newFakeIdentity(user),
	}

	ctx := context.Background()
	_, err := webRegistration.Begin(ctx, "" /* user */, false /* passwordless */)
	require.True(t, trace.IsBadParameter(err)) // user required
}

func TestRegistrationFlow_Finish_errors(t *testing.T) {
	const user = "llama"
	const webOrigin = "https://localhost"
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		Identity: newFakeIdentity(user),
	}

	ctx := context.Background()

	cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
	require.NoError(t, err)
	key, err := mocku2f.Create()
	require.NoError(t, err)
	okCCR, err := key.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)

	tests := []struct {
		name             string
		user, deviceName string
		createResp       func() *wantypes.CredentialCreationResponse
		wantErr          string
		passwordless     bool
	}{
		{
			name:       "NOK user empty",
			user:       "",
			deviceName: "webauthn2",
			createResp: func() *wantypes.CredentialCreationResponse { return okCCR },
			wantErr:    "user required",
		},
		{
			name:       "NOK device name empty",
			user:       user,
			deviceName: "",
			createResp: func() *wantypes.CredentialCreationResponse { return okCCR },
			wantErr:    "device name required",
		},
		{
			name:       "NOK credential response nil",
			user:       user,
			deviceName: "webauthn2",
			createResp: func() *wantypes.CredentialCreationResponse { return nil },
			wantErr:    "response required",
		},
		{
			name:       "NOK credential with bad origin",
			user:       user,
			deviceName: "webauthn2",
			createResp: func() *wantypes.CredentialCreationResponse {
				resp, err := key.SignCredentialCreation("https://alpacasarerad.com" /* origin */, cc)
				require.NoError(t, err)
				return resp
			},
			wantErr: "origin",
		},
		{
			name:       "NOK credential with bad RPID",
			user:       user,
			deviceName: "webauthn2",
			createResp: func() *wantypes.CredentialCreationResponse {
				cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
				require.NoError(t, err)
				cc.Response.RelyingParty.ID = "badrpid.com"

				resp, err := key.SignCredentialCreation(webOrigin, cc)
				require.NoError(t, err)
				return resp
			},
			wantErr: "authenticator response",
		},
		{
			name:       "NOK credential with invalid signature",
			user:       user,
			deviceName: "webauthn2",
			createResp: func() *wantypes.CredentialCreationResponse {
				cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
				require.NoError(t, err)
				// Flip a challenge bit, this should be enough to consistently fail
				// signature checking.
				cc.Response.Challenge[0] = 1 ^ cc.Response.Challenge[0]

				resp, err := key.SignCredentialCreation(webOrigin, cc)
				require.NoError(t, err)
				return resp
			},
			wantErr: "validating challenge",
		},
		{
			name:         "NOK passwordless on Finish but not on Begin",
			user:         user,
			deviceName:   "webauthn2",
			passwordless: true,
			createResp: func() *wantypes.CredentialCreationResponse {
				cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
				require.NoError(t, err)
				resp, err := key.SignCredentialCreation(webOrigin, cc)
				require.NoError(t, err)
				return resp
			},
			wantErr: "passwordless registration failed",
		},
		{
			name:         "NOK passwordless using key without PIN",
			user:         user,
			deviceName:   "webauthn2",
			passwordless: true,
			createResp: func() *wantypes.CredentialCreationResponse {
				cc, err := webRegistration.Begin(ctx, user, true /* passwordless */)
				require.NoError(t, err)

				// "Trick" the authenticator into signing, regardless of resident key or
				// verification requirements.
				// Verified on Safari 16.5 (and likely other versions too).
				cc.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementDiscouraged
				cc.Response.AuthenticatorSelection.RequireResidentKey = protocol.ResidentKeyNotRequired()
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationDiscouraged

				resp, err := key.SignCredentialCreation(webOrigin, cc)
				require.NoError(t, err)
				return resp
			},
			wantErr: "doesn't support passwordless",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := webRegistration.Finish(ctx, wanlib.RegisterResponse{
				User:             test.user,
				DeviceName:       test.deviceName,
				CreationResponse: test.createResp(),
				Passwordless:     test.passwordless,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestRegistrationFlow_Finish_attestation(t *testing.T) {
	const rpID = "localhost"
	const origin = "https://localhost"
	const user = "llama"
	const devName = "web1" // OK to repeat, discarded between tests.

	dev1, err := mocku2f.Create()
	require.NoError(t, err)
	dev2, err := mocku2f.Create()
	require.NoError(t, err)
	dev3, err := mocku2f.Create()
	require.NoError(t, err)

	tests := []struct {
		name                  string
		allowedCAs, deniedCAs [][]byte
		dev                   *mocku2f.Key
		wantOK                bool
	}{
		{
			name:       "OK Device clears allow list",
			allowedCAs: [][]byte{dev1.Cert, dev2.Cert},
			dev:        dev1,
			wantOK:     true,
		},
		{
			name:      "OK Device clears deny list",
			deniedCAs: [][]byte{dev1.Cert, dev2.Cert},
			dev:       dev3,
			wantOK:    true,
		},
		{
			name:       "OK Device clears allow and deny lists",
			allowedCAs: [][]byte{dev1.Cert},
			deniedCAs:  [][]byte{dev2.Cert, dev3.Cert},
			dev:        dev1,
			wantOK:     true,
		},
		{
			name:       "NOK Device not allowed",
			allowedCAs: [][]byte{dev1.Cert, dev2.Cert},
			dev:        dev3,
		},
		{
			name:      "NOK Device denied",
			deniedCAs: [][]byte{dev1.Cert, dev2.Cert},
			dev:       dev1,
		},
		{
			name:       "NOK Device denied (allow plus deny version)",
			allowedCAs: [][]byte{dev1.Cert},
			// Usually, in this case, the allowed CA would be broader than the denied
			// CA, but we are going for a simplified (albeit odd) scenario in the
			// test.
			deniedCAs: [][]byte{dev1.Cert},
			dev:       dev1,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			webRegistration := wanlib.RegistrationFlow{
				Webauthn: &types.Webauthn{
					RPID:                  rpID,
					AttestationAllowedCAs: derToPEMs(test.allowedCAs),
					AttestationDeniedCAs:  derToPEMs(test.deniedCAs),
				},
				Identity: newFakeIdentity(user),
			}

			cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
			require.NoError(t, err)

			dev := test.dev
			ccr, err := dev.SignCredentialCreation(origin, cc)
			require.NoError(t, err)

			_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
				User:             user,
				DeviceName:       devName,
				CreationResponse: ccr,
			})
			if ok := err == nil; ok != test.wantOK {
				t.Errorf("Finish returned err = %v, wantOK = %v", err, test.wantOK)
			}
		})
	}
}

// TestIssue31187_errorParsingAttestationResponse reproduces the root cause of
// https://github.com/gravitational/teleport/issues/31187 by attempting to parse
// a current CCR created using a Chrome/Yubikey pair.
//
// The test exposes a poor interaction between go-webauthn/webauthn v0.8.6 and
// fxamacker/cbor/v2 v2.5.0.
func TestIssue31187_errorParsingAttestationResponse(t *testing.T) {
	// Captured from an actual Yubikey 5Ci registration request.
	const body = `{"id":"ibfM_71b4q2_xWPZDyvhZmJ_KU8f-mOCCLXHp-fTVoHZpDelym5lvBJDPr1EtD_l","type":"public-key","rawId":"ibfM_71b4q2_xWPZDyvhZmJ_KU8f-mOCCLXHp-fTVoHZpDelym5lvBJDPr1EtD_l","response":{"clientDataJSON":"eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIiwiY2hhbGxlbmdlIjoidEdiVFhEbzBGMXRNUVlmamRSLWNETlV1TUNvVURTX0w0OElSWmY4MUVuWSIsIm9yaWdpbiI6Imh0dHBzOi8vemFycXVvbi5kZXY6MzA4MCIsImNyb3NzT3JpZ2luIjpmYWxzZSwib3RoZXJfa2V5c19jYW5fYmVfYWRkZWRfaGVyZSI6ImRvIG5vdCBjb21wYXJlIGNsaWVudERhdGFKU09OIGFnYWluc3QgYSB0ZW1wbGF0ZS4gU2VlIGh0dHBzOi8vZ29vLmdsL3lhYlBleCJ9","attestationObject":"o2NmbXRkbm9uZWdhdHRTdG10oGhhdXRoRGF0YVjCnNjmsqMh0nu-_tuMkxVZkZShAhdoz0tK9evxg8ys9CLFAAAAAQAAAAAAAAAAAAAAAAAAAAAAMIm3zP-9W-Ktv8Vj2Q8r4WZifylPH_pjggi1x6fn01aB2aQ3pcpuZbwSQz69RLQ_5aUBAgMmIAEhWCCJt8z_vVvirb_FY9kPpoIwbfhER3VHTmOV0Y6xs7uHySJYIMFARJxlUoR4DbDzlKYnfJKitWgR3GHK9_Lz211z-128oWtjcmVkUHJvdGVjdAI"}}`

	_, err := protocol.ParseCredentialCreationResponseBody(strings.NewReader(body))
	require.NoError(t, err, "ParseCredentialCreationResponseBody failed")
}

func TestRegistrationFlow_credProps(t *testing.T) {
	key, err := mocku2f.Create()
	require.NoError(t, err, "Create failed")

	const user = "llama"
	const origin = "https://localhost"
	ctx := context.Background()

	rf := &wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		Identity: newFakeIdentity(user),
	}

	// Begin ceremony.
	cc, err := rf.Begin(ctx, user, false /* passwordless */)
	require.NoError(t, err, "Begin failed")

	// Verify that the server requested credProps.
	val, ok := cc.Response.Extensions[wantypes.CredPropsExtension]
	require.True(t, ok, "CredentialCreation lacks credProps extension", cc.Response.Extensions)
	credPropsRequested, ok := val.(bool)
	require.True(t, ok && credPropsRequested, "CredentialCreation: credProps not set to true", cc.Response.Extensions)

	// Sign with credProps.
	key.ReplyWithCredProps = true
	ccr, err := key.SignCredentialCreation(origin, cc)
	require.NoError(t, err, "SignCredentialCreation failed")

	// Finish ceremony and verify mfaDev.
	mfaDev, err := rf.Finish(ctx, wanlib.RegisterResponse{
		User:             user,
		DeviceName:       "mydevice",
		CreationResponse: ccr,
	})
	require.NoError(t, err, "Finish failed")
	assert.True(t, mfaDev.GetWebauthn().ResidentKey, "mfaDev.ResidentKey flag mismatch")
}

func derToPEMs(certs [][]byte) []string {
	res := make([]string, len(certs))
	for i, cert := range certs {
		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		})
		res[i] = string(certPEM)
	}
	return res
}
