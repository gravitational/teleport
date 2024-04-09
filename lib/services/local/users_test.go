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

package local_test

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func newIdentityService(t *testing.T, clock clockwork.Clock) *local.IdentityService {
	t.Helper()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clock,
	})
	require.NoError(t, err)
	return local.NewTestIdentityService(backend)
}

func TestRecoveryCodesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	identity := newIdentityService(t, clock)

	// Create a recovery codes resource.
	mockedCodes := []types.RecoveryCode{
		{HashedCode: []byte("code1")},
		{HashedCode: []byte("code2")},
		{HashedCode: []byte("code3")},
	}

	t.Run("upsert, get, delete recovery codes", func(t *testing.T) {
		username := "someuser"

		rc1, err := types.NewRecoveryCodes(mockedCodes, clock.Now(), username)
		require.NoError(t, err)

		// Test creation of codes.
		err = identity.UpsertRecoveryCodes(ctx, username, rc1)
		require.NoError(t, err)

		// Test fetching of codes.
		codes, err := identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Create new codes for same user.
		newMockedCodes := []types.RecoveryCode{
			{HashedCode: []byte("new-code1")},
			{HashedCode: []byte("new-code2")},
			{HashedCode: []byte("new-code3")},
		}
		rc2, err := types.NewRecoveryCodes(newMockedCodes, clock.Now(), username)
		require.NoError(t, err)

		// Test update of codes for same user.
		err = identity.UpsertRecoveryCodes(ctx, username, rc2)
		require.NoError(t, err)

		// Test codes have been updated for same user.
		codes, err = identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, newMockedCodes, codes.GetCodes())

		// Test without secrets.
		codes, err = identity.GetRecoveryCodes(ctx, username, false /* withSecrets */)
		require.NoError(t, err)
		require.Empty(t, codes.GetCodes())
	})

	t.Run("deleting user deletes recovery codes", func(t *testing.T) {
		username := "someuser2"

		// Create a user.
		userResource := &types.UserV2{}
		userResource.SetName(username)
		_, err := identity.CreateUser(ctx, userResource)
		require.NoError(t, err)

		// Test codes exist for user.
		rc1, err := types.NewRecoveryCodes(mockedCodes, clock.Now(), username)
		require.NoError(t, err)
		err = identity.UpsertRecoveryCodes(ctx, username, rc1)
		require.NoError(t, err)
		codes, err := identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Test deletion of recovery code along with user.
		err = identity.DeleteUser(ctx, username)
		require.NoError(t, err)
		_, err = identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("deleting user with common prefix", func(t *testing.T) {
		username1 := "test"
		username2 := "test1"

		// Create a user.
		userResource1 := &types.UserV2{}
		userResource1.SetName(username1)
		_, err := identity.CreateUser(ctx, userResource1)
		require.NoError(t, err)

		// Create another user whose username which is prefixed with
		// the previous username.
		userResource2 := &types.UserV2{}
		userResource2.SetName(username2)
		_, err = identity.CreateUser(ctx, userResource2)
		require.NoError(t, err)

		// Test codes exist for the first user.
		rc1, err := types.NewRecoveryCodes(mockedCodes, clock.Now(), username1)
		require.NoError(t, err)
		err = identity.UpsertRecoveryCodes(ctx, username1, rc1)
		require.NoError(t, err)
		codes, err := identity.GetRecoveryCodes(ctx, username1, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Test codes exist for the second user.
		rc2, err := types.NewRecoveryCodes(mockedCodes, clock.Now(), username2)
		require.NoError(t, err)
		err = identity.UpsertRecoveryCodes(ctx, username2, rc2)
		require.NoError(t, err)
		codes, err = identity.GetRecoveryCodes(ctx, username2, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Test deletion of recovery code along with the first user.
		err = identity.DeleteUser(ctx, username1)
		require.NoError(t, err)
		_, err = identity.GetRecoveryCodes(ctx, username1, true /* withSecrets */)
		require.True(t, trace.IsNotFound(err))

		// Test recovery code and user of the second user still exist.
		_, err = identity.GetRecoveryCodes(ctx, username2, true /* withSecrets */)
		require.NoError(t, err)
	})

	t.Run("deleting user ending with 'z'", func(t *testing.T) {
		// enable the sanitizer, and use a key ending with z,
		// which will produce an invalid backend key when we
		// compute the end range
		username := "xyz"
		identity.Backend = backend.NewSanitizer(identity.Backend)

		// Create a user.
		userResource := &types.UserV2{}
		userResource.SetName(username)
		_, err := identity.CreateUser(ctx, userResource)
		require.NoError(t, err)

		// Test codes exist for user.
		rc1, err := types.NewRecoveryCodes(mockedCodes, clock.Now(), username)
		require.NoError(t, err)
		err = identity.UpsertRecoveryCodes(ctx, username, rc1)
		require.NoError(t, err)
		codes, err := identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Test deletion of recovery code along with user.
		err = identity.DeleteUser(ctx, username)
		require.NoError(t, err)
		_, err = identity.GetRecoveryCodes(ctx, username, true /* withSecrets */)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestIdentityService_UpsertMFADevice(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	tests := []struct {
		name string
		user string
		dev  *types.MFADevice
	}{
		{
			name: "OK TOTP device",
			user: "llama",
			dev: &types.MFADevice{
				Metadata: types.Metadata{
					Name: "totp",
				},
				Id:       uuid.NewString(),
				AddedAt:  time.Now(),
				LastUsed: time.Now(),
				Device: &types.MFADevice_Totp{
					Totp: &types.TOTPDevice{
						Key: "supersecretkey",
					},
				},
			},
		},
		{
			name: "OK Webauthn device",
			user: "llama",
			dev: &types.MFADevice{
				Metadata: types.Metadata{
					Name: "webauthn",
				},
				Id:       uuid.NewString(),
				AddedAt:  time.Now(),
				LastUsed: time.Now(),
				Device: &types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:     []byte("credential ID"),
						PublicKeyCbor:    []byte("public key"),
						AttestationType:  "none",
						Aaguid:           []byte{1, 2, 3, 4, 5},
						SignatureCounter: 10,
					},
				},
			},
		},
	}
	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := identity.UpsertMFADevice(ctx, test.user, test.dev)
			require.NoError(t, err)

			devs, err := identity.GetMFADevices(ctx, test.user, true /* withSecrets */)
			require.NoError(t, err)
			found := false
			for _, dev := range devs {
				if dev.GetName() == test.dev.GetName() {
					found = true
					if diff := cmp.Diff(dev, test.dev); diff != "" {
						t.Fatalf("GetMFADevices() mismatch (-want +got):\n%s", diff)
					}
					break
				}
			}
			require.True(t, found, "device %q not found", test.dev.GetName())
		})
	}
}

func TestIdentityService_UpsertMFADevice_errors(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	totpDev := &types.MFADevice{
		Metadata: types.Metadata{
			Name: "totp",
		},
		Id:       uuid.NewString(),
		AddedAt:  time.Now(),
		LastUsed: time.Now(),
		Device: &types.MFADevice_Totp{
			Totp: &types.TOTPDevice{
				Key: "supersecretkey",
			},
		},
	}
	u2fDev := &types.MFADevice{
		Metadata: types.Metadata{
			Name: "u2f",
		},
		Id:       uuid.NewString(),
		AddedAt:  time.Now(),
		LastUsed: time.Now(),
		Device: &types.MFADevice_U2F{
			U2F: &types.U2FDevice{
				KeyHandle: []byte("u2f key handle"),
				PubKey:    []byte("u2f public key"),
			},
		},
	}
	webauthnDev := &types.MFADevice{
		Metadata: types.Metadata{
			Name: "webauthn",
		},
		Id:       uuid.NewString(),
		AddedAt:  time.Now(),
		LastUsed: time.Now(),
		Device: &types.MFADevice_Webauthn{
			Webauthn: &types.WebauthnDevice{
				CredentialId:  []byte("web credential ID"),
				PublicKeyCbor: []byte("web public key"),
			},
		},
	}
	ctx := context.Background()
	const user = "llama"
	for _, dev := range []*types.MFADevice{totpDev, u2fDev, webauthnDev} {
		err := identity.UpsertMFADevice(ctx, user, dev)
		require.NoError(t, err, "upsert device %q", dev.GetName())
	}

	tests := []struct {
		name      string
		createDev func() *types.MFADevice
		wantErr   string
	}{
		{
			name: "NOK invalid WebauthnDevice",
			createDev: func() *types.MFADevice {
				cp := *webauthnDev
				cp.Metadata.Name = "new webauthn"
				cp.Id = uuid.NewString()
				cp.Device = &types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:  nil, // NOK, required.
						PublicKeyCbor: []byte("unique public key ID"),
					},
				}
				return &cp
			},
			wantErr: "missing CredentialId",
		},
		{
			name: "NOK duplicate device name",
			createDev: func() *types.MFADevice {
				cp := *webauthnDev
				// cp.Metadata.Name is equal, everything else is valid.
				cp.Id = uuid.NewString()
				cp.Device = &types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:  []byte("unique credential ID"),
						PublicKeyCbor: []byte("unique public key ID"),
					},
				}
				return &cp
			},
			wantErr: "device with name",
		},
		{
			name: "NOK duplicate credential ID (U2F device)",
			createDev: func() *types.MFADevice {
				cp := *webauthnDev
				cp.Metadata.Name = "new webauthn"
				cp.Id = uuid.NewString()
				cp.Device = &types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:  u2fDev.GetU2F().KeyHandle,
						PublicKeyCbor: []byte("unique public key ID"),
					},
				}
				return &cp
			},
			wantErr: "credential ID already in use",
		},
		{
			name: "NOK duplicate credential ID (Webauthn device)",
			createDev: func() *types.MFADevice {
				cp := *webauthnDev
				cp.Metadata.Name = "new webauthn"
				cp.Id = uuid.NewString()
				cp.Device = &types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:  webauthnDev.GetWebauthn().CredentialId,
						PublicKeyCbor: []byte("unique public key ID"),
					},
				}
				return &cp
			},
			wantErr: "credential ID already in use",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := identity.UpsertMFADevice(ctx, user, test.createDev())
			require.Error(t, err)
			require.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestIdentityService_UpsertWebauthnLocalAuth(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	updateViaUser := func(ctx context.Context, user string, wal *types.WebauthnLocalAuth) error {
		u, err := types.NewUser(user)
		if err != nil {
			return err
		}
		las := u.GetLocalAuth()
		if las == nil {
			las = &types.LocalAuthSecrets{}
		}
		las.Webauthn = wal
		u.SetLocalAuth(las)

		_, err = identity.UpsertUser(ctx, u)
		return err
	}
	getViaUser := func(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
		u, err := identity.GetUser(ctx, user, true /* withSecrets */)
		if err != nil {
			return nil, err
		}
		return u.GetLocalAuth().Webauthn, nil
	}

	// Create a user to begin with.
	ctx := context.Background()
	const name = "llama"
	user, err := types.NewUser(name)
	require.NoError(t, err)
	_, err = identity.UpsertUser(ctx, user)
	require.NoError(t, err)

	// Try a few empty reads.
	_, err = identity.GetUser(ctx, name, true /* withSecrets */)
	require.NoError(t, err) // User read should be fine.
	_, err = identity.GetWebauthnLocalAuth(ctx, name)
	require.True(t, trace.IsNotFound(err)) // Direct WAL read should fail.

	// Try a few invalid updates.
	badWAL := &types.WebauthnLocalAuth{} // missing UserID
	err = identity.UpsertWebauthnLocalAuth(ctx, name, badWAL)
	require.True(t, trace.IsBadParameter(err))
	user.SetLocalAuth(&types.LocalAuthSecrets{Webauthn: badWAL})
	_, err = identity.UpdateUser(ctx, user)
	require.True(t, trace.IsBadParameter(err))

	// Update/Read tests.
	tests := []struct {
		name   string
		user   string
		wal    *types.WebauthnLocalAuth
		update func(context.Context, string, *types.WebauthnLocalAuth) error
		get    func(context.Context, string) (*types.WebauthnLocalAuth, error)
	}{
		{
			name:   "OK: Create WAL directly",
			user:   "llama",
			wal:    &types.WebauthnLocalAuth{UserID: []byte("webauthn user ID for llama")},
			update: identity.UpsertWebauthnLocalAuth,
			get:    identity.GetWebauthnLocalAuth,
		},
		{
			name:   "OK: Update WAL directly",
			user:   "llama", // same as above
			wal:    &types.WebauthnLocalAuth{UserID: []byte("another ID")},
			update: identity.UpsertWebauthnLocalAuth,
			get:    identity.GetWebauthnLocalAuth,
		},
		{
			name:   "OK: Create WAL via user",
			user:   "alpaca", // new user
			wal:    &types.WebauthnLocalAuth{UserID: []byte("webauthn user ID for alpaca")},
			update: updateViaUser,
			get:    getViaUser,
		},
		{
			name:   "OK: Update WAL via user",
			user:   "alpaca", // same as above
			wal:    &types.WebauthnLocalAuth{UserID: []byte("some other ID")},
			update: updateViaUser,
			get:    getViaUser,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.update(ctx, test.name, test.wal)
			require.NoError(t, err)

			wantWLA := test.wal
			gotWLA, err := test.get(ctx, test.name)
			require.NoError(t, err)
			if diff := cmp.Diff(wantWLA, gotWLA); diff != "" {
				t.Fatalf("WebauthnLocalAuth mismatch (-want +got):\n%s", diff)
			}

			gotUser, err := identity.GetTeleportUserByWebauthnID(ctx, gotWLA.UserID)
			require.NoError(t, err)
			require.Equal(t, test.name, gotUser)
		})
	}
}

func TestIdentityService_GetTeleportUserByWebauthnID(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	tests := []struct {
		name      string
		webID     []byte
		assertErr func(error) bool
	}{
		{
			name:      "NOK empty web ID",
			webID:     nil,
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "NOK unknown web ID",
			webID:     []byte{1, 2, 3, 4, 5},
			assertErr: trace.IsNotFound,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := identity.GetTeleportUserByWebauthnID(ctx, test.webID)
			require.Error(t, err)
			require.True(t, test.assertErr(err))
		})
	}
}

func TestIdentityService_WebauthnSessionDataCRUD(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	const user1 = "llama"
	const user2 = "alpaca"
	// Prepare a few different objects so we can assert that both "user" and
	// "session" key components are used correctly.
	user1Reg := &wantypes.SessionData{
		Challenge: []byte("challenge1-reg"),
		UserId:    []byte("llamaid"),
	}
	user1Login := &wantypes.SessionData{
		Challenge:        []byte("challenge1-login"),
		UserId:           []byte("llamaid"),
		AllowCredentials: [][]byte{[]byte("cred1"), []byte("cred2")},
	}
	user2Login := &wantypes.SessionData{
		Challenge: []byte("challenge2"),
		UserId:    []byte("alpacaid"),
	}

	// Usually there are only 2 sessions for each user: login and registration.
	const registerSession = "register"
	const loginSession = "login"
	params := []struct {
		user, session string
		sd            *wantypes.SessionData
	}{
		{user: user1, session: registerSession, sd: user1Reg},
		{user: user1, session: loginSession, sd: user1Login},
		{user: user2, session: loginSession, sd: user2Login},
	}

	// Verify upsert/create.
	ctx := context.Background()
	for _, p := range params {
		err := identity.UpsertWebauthnSessionData(ctx, p.user, p.session, p.sd)
		require.NoError(t, err)
	}

	// Verify read.
	for _, p := range params {
		got, err := identity.GetWebauthnSessionData(ctx, p.user, p.session)
		require.NoError(t, err)
		if diff := cmp.Diff(p.sd, got); diff != "" {
			t.Fatalf("GetWebauthnSessionData() mismatch (-want +got):\n%s", diff)
		}
	}

	// Verify upsert/update.
	user1Reg = &wantypes.SessionData{
		Challenge: []byte("challenge1reg--another"),
		UserId:    []byte("llamaid"),
	}
	err := identity.UpsertWebauthnSessionData(ctx, user1, registerSession, user1Reg)
	require.NoError(t, err)
	got, err := identity.GetWebauthnSessionData(ctx, user1, registerSession)
	require.NoError(t, err)
	if diff := cmp.Diff(user1Reg, got); diff != "" {
		t.Fatalf("GetWebauthnSessionData() mismatch (-want +got):\n%s", diff)
	}

	// Verify deletion.
	err = identity.DeleteWebauthnSessionData(ctx, user1, registerSession)
	require.NoError(t, err)
	_, err = identity.GetWebauthnSessionData(ctx, user1, registerSession)
	require.True(t, trace.IsNotFound(err))
	params = params[1:] // Remove user1/register from params
	for _, p := range params {
		_, err := identity.GetWebauthnSessionData(ctx, p.user, p.session)
		require.NoError(t, err) // Other keys preserved
	}
}

func TestIdentityService_GlobalWebauthnSessionDataCRUD(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	user1Login1 := &wantypes.SessionData{
		Challenge:        []byte("challenge1"),
		UserId:           []byte("user1-web-id"),
		UserVerification: string(protocol.VerificationRequired),
	}
	user1Login2 := &wantypes.SessionData{
		Challenge:        []byte("challenge2"),
		UserId:           []byte("user1-web-id"),
		UserVerification: string(protocol.VerificationRequired),
	}
	user1Registration := &wantypes.SessionData{
		Challenge:        []byte("challenge3"),
		UserId:           []byte("user1-web-id"),
		ResidentKey:      true,
		UserVerification: string(protocol.VerificationRequired),
	}
	user2Login := &wantypes.SessionData{
		Challenge:        []byte("challenge4"),
		UserId:           []byte("user2-web-id"),
		ResidentKey:      true,
		UserVerification: string(protocol.VerificationRequired),
	}

	const scopeLogin = "login"
	// Registration doesn't typically use global session data, used here for
	// testing purposes only.
	const scopeRegister = "register"
	params := []struct {
		scope, id string
		sd        *wantypes.SessionData
	}{
		{scope: scopeLogin, id: base64.RawURLEncoding.EncodeToString(user1Login1.Challenge), sd: user1Login1},
		{scope: scopeLogin, id: base64.RawURLEncoding.EncodeToString(user1Login2.Challenge), sd: user1Login2},
		{scope: scopeRegister, id: base64.RawURLEncoding.EncodeToString(user1Registration.Challenge), sd: user1Registration},
		{scope: scopeLogin, id: base64.RawURLEncoding.EncodeToString(user2Login.Challenge), sd: user2Login},
	}

	// Verify create.
	ctx := context.Background()
	for _, p := range params {
		require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, p.scope, p.id, p.sd))
	}

	// Verify read.
	for _, p := range params {
		got, err := identity.GetGlobalWebauthnSessionData(ctx, p.scope, p.id)
		require.NoError(t, err)
		if diff := cmp.Diff(p.sd, got); diff != "" {
			t.Errorf("GetGlobalWebauthnSessionData() mismatch (-want +got):\n%s", diff)
		}
	}

	// Verify update.
	p0 := &params[0]
	p0.sd.UserVerification = ""
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, p0.scope, p0.id, p0.sd))
	got, err := identity.GetGlobalWebauthnSessionData(ctx, p0.scope, p0.id)
	require.NoError(t, err)
	if diff := cmp.Diff(p0.sd, got); diff != "" {
		t.Errorf("GetGlobalWebauthnSessionData() mismatch (-want +got):\n%s", diff)
	}

	// Verify deletion.
	require.NoError(t, identity.DeleteGlobalWebauthnSessionData(ctx, p0.scope, p0.id))
	_, err = identity.GetGlobalWebauthnSessionData(ctx, p0.scope, p0.id)
	require.True(t, trace.IsNotFound(err))
	params = params[1:] // Remove p0 from params
	for _, p := range params {
		_, err := identity.GetGlobalWebauthnSessionData(ctx, p.scope, p.id)
		require.NoError(t, err) // Other keys preserved
	}
}

func TestIdentityService_UpsertGlobalWebauthnSessionData_maxLimit(t *testing.T) {
	// Don't t.Parallel()!

	sdMax := local.GlobalSessionDataMaxEntries
	sdClock := local.SessionDataLimiter.Clock
	sdReset := local.SessionDataLimiter.ResetPeriod
	defer func() {
		local.GlobalSessionDataMaxEntries = sdMax
		local.SessionDataLimiter.Clock = sdClock
		local.SessionDataLimiter.ResetPeriod = sdReset
		local.SessionDataLimiter.Reset()
	}()
	fakeClock := clockwork.NewFakeClock()
	period := 1 * time.Minute // arbitrary, applied to fakeClock
	local.GlobalSessionDataMaxEntries = 2
	local.SessionDataLimiter.Clock = fakeClock
	local.SessionDataLimiter.ResetPeriod = period

	// Add some randomness to the scopes to avoid high -count runs tripping on
	// each other.
	scopeLogin := "login" + uuid.NewString()
	scopeOther := "other" + uuid.NewString()
	const id1 = "challenge1"
	const id2 = "challenge2"
	const id3 = "challenge3"
	const id4 = "challenge4"
	sd := &wantypes.SessionData{
		Challenge:        []byte("supersecretchallenge"), // typically matches the key
		UserVerification: "required",
	}

	identity := newIdentityService(t, clockwork.NewFakeClock())
	ctx := context.Background()

	// OK: below limit.
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id1, sd))
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id2, sd))
	// NOK: limit reached.
	err := identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id3, sd)
	require.True(t, trace.IsLimitExceeded(err), "got err = %v, want LimitExceeded", err)

	// OK: different scope.
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeOther, id1, sd))
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeOther, id2, sd))
	// NOK: limit reached.
	err = identity.UpsertGlobalWebauthnSessionData(ctx, scopeOther, id3, sd)
	require.True(t, trace.IsLimitExceeded(err), "got err = %v, want LimitExceeded", err)

	// OK: keys removed.
	require.NoError(t, identity.DeleteGlobalWebauthnSessionData(ctx, scopeLogin, id1))
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id4, sd))

	// NOK: reach and double-check limits.
	require.Error(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id3, sd))
	require.Error(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeOther, id3, sd))
	// OK: passage of time resets limits.
	fakeClock.Advance(period)
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id3, sd))
	require.NoError(t, identity.UpsertGlobalWebauthnSessionData(ctx, scopeOther, id3, sd))
}

func TestIdentityService_SSODiagnosticInfoCrud(t *testing.T) {
	identity := newIdentityService(t, clockwork.NewFakeClock())
	ctx := context.Background()

	nilInfo, err := identity.GetSSODiagnosticInfo(ctx, types.KindSAML, "BAD_ID")
	require.Nil(t, nilInfo)
	require.Error(t, err)

	info := types.SSODiagnosticInfo{
		TestFlow: true,
		Error:    "foo bar baz",
		Success:  false,
		CreateUserParams: &types.CreateUserParams{
			ConnectorName: "bar",
			Username:      "baz",
		},
		SAMLAttributesToRoles: []types.AttributeMapping{
			{
				Name:  "foo",
				Value: "bar",
				Roles: []string{"baz"},
			},
		},
		SAMLAttributesToRolesWarnings: nil,
		SAMLAttributeStatements:       nil,
		SAMLAssertionInfo:             nil,
		SAMLTraitsFromAssertions:      nil,
		SAMLConnectorTraitMapping:     nil,
	}

	err = identity.CreateSSODiagnosticInfo(ctx, types.KindSAML, "MY_ID", info)
	require.NoError(t, err)

	infoGet, err := identity.GetSSODiagnosticInfo(ctx, types.KindSAML, "MY_ID")
	require.NoError(t, err)
	require.Equal(t, &info, infoGet)
}

func TestIdentityService_UpsertKeyAttestationData(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())
	ctx := context.Background()

	for _, tc := range []struct {
		name             string
		pubKeyPEM        string
		expectPubKeyHash string
	}{
		{
			name: "public key",
			pubKeyPEM: `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDCep78YgY5I8RrvhE5zra4k1hx
JZoZL1NsgqBz/f49OZsck24rcxurnC0lKAJmSGtKZrv54E/XZuPtatUkrXtIFKC6
shHLLAc/LAVtDX2/E/aLgM0srYtt1/kku9H1C9+Ou7RzOIdblRkNMYcbUOhKBNld
AnYsqjU9/7IaQSp8DwIDAQAB
-----END PUBLIC KEY-----`,
		}, {
			name: "public key with // in plain sha hash",
			pubKeyPEM: `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCwh1y2u/z8Rm4jD51oawtI00NO
yHPtEsk3AcetyxYXM5jXAZuQBJwFoxQa3tlJoumigfVEsdYhETu1zwJLZhjgmYOp
eKMx+eKGKvDF73w1Kfap+JrGA2d1+XtPfNZkmcjYThe+GY0yfinnIwcjd+lmqCqb
Tirv9LjajEBxUnuV+wIDAQAB
-----END PUBLIC KEY-----`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := pem.Decode([]byte(tc.pubKeyPEM))
			require.NotNil(t, p)
			pubDer := p.Bytes

			attestationData := &keys.AttestationData{
				PublicKeyDER:     pubDer,
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
			}

			err := identity.UpsertKeyAttestationData(ctx, attestationData, time.Hour)
			require.NoError(t, err, "UpsertKeyAttestationData failed")

			retrievedAttestationData, err := identity.GetKeyAttestationData(ctx, pubDer)
			require.NoError(t, err, "GetKeyAttestationData failed")
			require.Equal(t, attestationData, retrievedAttestationData, "GetKeyAttestationData mismatch")
		})
	}
}

func TestIdentityService_UpdateAndSwapUser(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())
	ctx := context.Background()

	type updateParams struct {
		user        string
		withSecrets bool
		fn          func(u types.User) (changed bool, err error)
	}

	tests := []struct {
		name     string
		makeUser func() (types.User, error) // if not nil, the user is created
		updateParams
		wantErr  string
		wantNoop bool
	}{
		{
			name: "update without secrets",
			makeUser: func() (types.User, error) {
				return types.NewUser("updateNoSecrets1")
			},
			updateParams: updateParams{
				fn: func(u types.User) (bool, error) {
					u.SetLogins([]string{"llama", "alpaca"})
					return true, nil
				},
			},
		},
		{
			name: "update without secrets can't write secrets",
			makeUser: func() (types.User, error) {
				return types.NewUser("updateNoSecrets2")
			},
			updateParams: updateParams{
				fn: func(u types.User) (bool, error) {
					u.SetLogins([]string{"llama", "alpaca"})
					u.SetLocalAuth(&types.LocalAuthSecrets{
						Webauthn: &types.WebauthnLocalAuth{
							UserID: []byte("superwebllama"),
						},
					})
					return true, nil
				},
			},
		},
		{
			name: "update with secrets",
			makeUser: func() (types.User, error) {
				return types.NewUser("updateWithSecrets")
			},
			updateParams: updateParams{
				withSecrets: true,
				fn: func(u types.User) (bool, error) {
					u.SetLogins([]string{"llama", "alpaca"})
					u.SetLocalAuth(&types.LocalAuthSecrets{
						Webauthn: &types.WebauthnLocalAuth{
							UserID: []byte("superwebllama"),
						},
					})
					return true, nil
				},
			},
		},
		{
			name: "noop fn",
			makeUser: func() (types.User, error) {
				return types.NewUser("noop1")
			},
			updateParams: updateParams{
				fn: func(u types.User) (changed bool, err error) {
					u.SetLogins([]string{"llama"}) // not written to storage!
					return false, nil
				},
			},
			wantNoop: true,
		},
		{
			name: "user not found",
			updateParams: updateParams{
				user: "unknown",
				fn:   func(u types.User) (changed bool, err error) { return false, nil },
			},
			wantErr: "not found",
		},
		{
			name: "fn error surfaced",
			makeUser: func() (types.User, error) {
				return types.NewUser("fnErr")
			},
			updateParams: updateParams{
				fn: func(u types.User) (changed bool, err error) {
					return false, errors.New("something really terrible happened")
				},
			},
			wantErr: "something really terrible",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var before types.User

			// Create user?
			if test.makeUser != nil {
				var err error
				before, err = test.makeUser()
				require.NoError(t, err, "makeUser failed")
				_, err = identity.CreateUser(ctx, before)
				require.NoError(t, err, "CreateUser failed")

				if test.user == "" {
					test.user = before.GetName()
				} else if test.user != before.GetName() {
					t.Fatal("Test has both makeUser and updateParams.user, but they don't match")
				}
			}

			updated, err := identity.UpdateAndSwapUser(ctx, test.user, test.withSecrets, test.fn)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "UpdateAndSwapUser didn't error")
				return
			}

			// Determine wanted user based on `before` and params.
			want := before
			if !test.wantNoop {
				test.fn(want)
			}
			if !test.withSecrets {
				want = want.WithoutSecrets().(types.User)
			}

			// Assert update response.
			if diff := cmp.Diff(want, updated, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")); diff != "" {
				t.Errorf("UpdateAndSwapUser return mismatch (-want +got)\n%s", diff)
			}

			// Assert stored.
			stored, err := identity.GetUser(ctx, test.user, test.withSecrets)
			require.NoError(t, err, "GetUser failed")
			if diff := cmp.Diff(want, stored, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")); diff != "" {
				t.Errorf("UpdateAndSwapUser storage mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestIdentityService_ListUsers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	identity := newIdentityService(t, clock)

	password, err := bcrypt.GenerateFromPassword([]byte("insecure"), bcrypt.MinCost)
	require.NoError(t, err, "creating password failed")

	dev1, err := services.NewTOTPDevice("otp1", base32.StdEncoding.EncodeToString([]byte("abc123")), time.Now())
	require.NoError(t, err, "creating otp device failed")
	dev2, err := services.NewTOTPDevice("otp2", base32.StdEncoding.EncodeToString([]byte("xyz789")), time.Now())
	require.NoError(t, err, "creating otp device failed")

	// Validate that no users returns an empty page.
	rsp, err := identity.ListUsers(ctx, &userspb.ListUsersRequest{})
	assert.NoError(t, err, "no error returned when no users exist")
	assert.Empty(t, rsp.Users, "users returned from listing when no users exist")
	assert.Empty(t, rsp.NextPageToken, "next page token returned from listing when no users exist")

	rsp, err = identity.ListUsers(ctx, &userspb.ListUsersRequest{
		WithSecrets: true,
	})
	assert.NoError(t, err, "no error returned when no users exist")
	assert.Empty(t, rsp.Users, "users returned from listing when no users exist")
	assert.Empty(t, rsp.NextPageToken, "next page token returned from listing when no users exist")

	// Validate that listing works when there is only a single user
	user, err := types.NewUser("fish0")
	require.NoError(t, err, "creating new user %s", user)

	user, err = identity.CreateUser(ctx, user)
	require.NoError(t, err, "creating user %s failed", user)
	expectedUsers := []*types.UserV2{user.(*types.UserV2)}

	rsp, err = identity.ListUsers(ctx, &userspb.ListUsersRequest{})
	assert.NoError(t, err, "no error returned when no users exist")
	assert.Empty(t, rsp.NextPageToken, "next page token returned from listing when no more users exist")
	assert.Empty(t, cmp.Diff(expectedUsers, rsp.Users, cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth")), "not all users returned from listing operation")

	rsp, err = identity.ListUsers(ctx, &userspb.ListUsersRequest{})
	assert.NoError(t, err, "no error returned when no users exist")
	assert.Empty(t, rsp.NextPageToken, "next page token returned from listing when no users exist")
	assert.Empty(t, cmp.Diff(expectedUsers, rsp.Users), "not all users returned from listing operation")

	// Create a number of users.
	usernames := []string{"llama", "alpaca", "fox", "fish", "fish+", "fish2"}
	for i, name := range usernames {
		user, err := types.NewUser(name)
		require.NoError(t, err, "creating new user %s", name)

		// Set varying number of devices so the listing logic
		// doesn't cleanly land on a user resource.
		devices := []*types.MFADevice{dev1}
		switch {
		case i%2 == 0:
			devices = append(devices, dev2)
		case i == 3:
			devices = nil
		case i == 1:
			devices = append(devices, dev2)
			for j := 0; j < 20; j++ {
				dev, err := services.NewTOTPDevice(uuid.NewString(), base32.StdEncoding.EncodeToString([]byte("abc123")), time.Now())
				require.NoError(t, err, "creating otp device failed")
				devices = append(devices, dev)
			}
		}

		user.SetLocalAuth(&types.LocalAuthSecrets{
			PasswordHash: password,
			MFA:          devices,
		})

		created, err := identity.CreateUser(ctx, user)
		require.NoError(t, err, "creating user %s failed", user)
		expectedUsers = append(expectedUsers, created.(*types.UserV2))
	}
	slices.SortFunc(expectedUsers, func(a, b *types.UserV2) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	// List a few users at a time and validate that all users are eventually returned.
	var retrieved []*types.UserV2
	req := userspb.ListUsersRequest{
		PageSize: 2,
	}
	for {
		rsp, err := identity.ListUsers(ctx, &req)
		require.NoError(t, err, "no error returned when no users exist")

		for _, user := range rsp.Users {
			assert.Empty(t, user.GetLocalAuth(), "expected no secrets to be returned with user %s", user.GetName())
		}

		retrieved = append(retrieved, rsp.Users...)

		req.PageToken = rsp.NextPageToken
		if req.PageToken == "" {
			break
		}

		if len(retrieved) > len(expectedUsers) {
			t.Fatalf("listing users has accumulated %d users even though only %d users exist", len(retrieved), len(expectedUsers))
		}
	}

	slices.SortFunc(retrieved, func(a, b *types.UserV2) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	assert.Empty(t, cmp.Diff(expectedUsers, retrieved, cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth")), "not all users returned from listing operation")

	// Validate that listing all users at once returns all expected users with secrets.
	rsp, err = identity.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize:    200,
		WithSecrets: true,
	})
	require.NoError(t, err, "unexpected error listing users")
	assert.Empty(t, rsp.NextPageToken, "got a next page token when page size was greater than number of items")

	users := rsp.Users

	slices.SortFunc(users, func(a, b *types.UserV2) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	devicesSort := func(a *types.MFADevice, b *types.MFADevice) bool { return a.GetName() < b.GetName() }

	require.Empty(t, cmp.Diff(expectedUsers, users, cmpopts.SortSlices(devicesSort)), "not all users returned from listing operation")

	// List a few users at a time and validate that all users are eventually returned with their secrets.
	retrieved = nil
	req = userspb.ListUsersRequest{
		PageSize:    2,
		WithSecrets: true,
	}
	for {
		rsp, err := identity.ListUsers(ctx, &req)
		require.NoError(t, err, "no error returned when no users exist")

		retrieved = append(retrieved, rsp.Users...)

		req.PageToken = rsp.NextPageToken
		if req.PageToken == "" {
			break
		}

		if len(retrieved) > len(expectedUsers) {
			t.Fatalf("listing users has accumulated %d users even though only %d users exist", len(retrieved), len(expectedUsers))
		}
	}

	slices.SortFunc(retrieved, func(a, b *types.UserV2) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	require.Empty(t, cmp.Diff(expectedUsers, retrieved, cmpopts.SortSlices(devicesSort)), "not all users returned from listing operation")

	ssoUser := expectedUsers[2]
	expectedUsers = slices.Delete(expectedUsers, 2, 3)
	ssoUser.SetExpiry(clock.Now().UTC().Add(time.Minute))

	_, err = identity.UpsertUser(ctx, ssoUser)
	assert.NoError(t, err, "failed to upsert SSO user")

	clock.Advance(time.Hour)

	rsp, err = identity.ListUsers(ctx, &userspb.ListUsersRequest{
		WithSecrets: true,
	})
	assert.NoError(t, err, "got an error while listing over an expired user")
	assert.Empty(t, rsp.NextPageToken, "next page token returned from listing all users")

	retrieved = rsp.Users

	slices.SortFunc(retrieved, func(a, b *types.UserV2) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	require.Empty(t, cmp.Diff(expectedUsers, retrieved, cmpopts.SortSlices(devicesSort)), "not all users returned from listing operation")
}

func TestCompareAndSwapUser(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctx := context.Background()

	identity := newIdentityService(t, clockwork.NewFakeClock())

	bob1, err := types.NewUser("bob")
	require.NoError(err)
	bob1.SetLogins([]string{"bob"})

	bob2, err := types.NewUser("bob")
	require.NoError(err)
	bob2.SetLogins([]string{"bob", "alice"})

	require.False(services.UsersEquals(bob1, bob2))

	currentBob, err := identity.UpsertUser(ctx, bob1)
	require.NoError(err)
	require.True(services.UsersEquals(currentBob, bob1))

	currentBob, err = identity.GetUser(ctx, "bob", false)
	require.NoError(err)
	require.True(services.UsersEquals(currentBob, bob1))

	err = identity.CompareAndSwapUser(ctx, bob2, bob1)
	require.NoError(err)

	currentBob, err = identity.GetUser(ctx, "bob", false)
	require.NoError(err)
	require.True(services.UsersEquals(currentBob, bob2))

	item, err := identity.Backend.Get(ctx, backend.Key(local.WebPrefix, local.UsersPrefix, "bob", local.ParamsPrefix))
	require.NoError(err)
	var m map[string]any
	require.NoError(json.Unmarshal(item.Value, &m))
	m["deprecated_field"] = 42
	item.Value, err = json.Marshal(m)
	require.NoError(err)
	_, err = identity.Backend.Put(ctx, *item)
	require.NoError(err)

	currentBob, err = identity.GetUser(ctx, "bob", false)
	require.NoError(err)
	require.True(services.UsersEquals(currentBob, bob2))

	err = identity.CompareAndSwapUser(ctx, bob1, bob2)
	require.NoError(err)

	currentBob, err = identity.GetUser(ctx, "bob", false)
	require.NoError(err)
	require.True(services.UsersEquals(currentBob, bob1))
}
