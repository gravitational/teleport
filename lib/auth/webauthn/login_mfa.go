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

package webauthn

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// ErrInvalidCredentials is a special kind of credential "NotFound" error, where
// the user has only devices registered to other RPIDs.
// Possible fixes include reseting the affected users (likely the entire
// cluster), or rolling back to a good WebAuthn configuration (if still
// possible).
var ErrInvalidCredentials = errors.New("user has only invalid WebAuthn registrations, consider a user reset")

// LoginIdentity represents the subset of Identity methods used by LoginFlow.
// It exists to better scope LoginFlow's use of Identity and to facilitate
// testing.
type LoginIdentity interface {
	GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error)

	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error
	UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error
	GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error)
	DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error
}

// WithDevices returns a LoginIdentity backed by a fixed set of devices.
// The supplied devices are returned in all GetMFADevices calls.
func WithDevices(identity LoginIdentity, devs []*types.MFADevice) LoginIdentity {
	return &loginWithDevices{
		LoginIdentity: identity,
		devices:       devs,
	}
}

type loginWithDevices struct {
	LoginIdentity
	devices []*types.MFADevice
}

func (l *loginWithDevices) GetMFADevices(_ context.Context, _ string, _ bool) ([]*types.MFADevice, error) {
	return l.devices, nil
}

// LoginFlow represents the WebAuthn login procedure (aka authentication).
//
// The login flow consists of:
//
//  1. Client requests a CredentialAssertion (containing, among other info, a
//     challenge to be signed)
//  2. Server runs Begin(), generates a credential assertion.
//  3. Client validates the assertion, performs a user presence test (usually by
//     asking the user to touch a secure token), and replies with
//     CredentialAssertionResponse (containing the signed challenge)
//  4. Server runs Finish()
//  5. If all server-side checks are successful, then login/authentication is
//     complete.
type LoginFlow struct {
	U2F      *types.U2F
	Webauthn *types.Webauthn
	// Identity is typically an implementation of the Identity service, ie, an
	// object with access to user, device and MFA storage.
	Identity LoginIdentity
}

// Begin is the first step of the LoginFlow.
// The CredentialAssertion created is relayed back to the client, who in turn
// performs a user presence check and signs the challenge contained within the
// assertion.
// As a side effect Begin may assign (and record in storage) a WebAuthn ID for
// the user.
func (f *LoginFlow) Begin(ctx context.Context, user string) (*wantypes.CredentialAssertion, error) {
	lf := &loginFlow{
		U2F:         f.U2F,
		Webauthn:    f.Webauthn,
		identity:    mfaIdentity{f.Identity},
		sessionData: (*userSessionStorage)(f),
	}
	ext := mfav1.ChallengeExtensions{
		Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED,
		AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
	}
	return lf.begin(ctx, user, ext)
}

// Finish is the second and last step of the LoginFlow.
// It returns the MFADevice used to solve the challenge. If login is successful,
// Finish has the side effect of updating the counter and last used timestamp of
// the returned device.
func (f *LoginFlow) Finish(ctx context.Context, user string, resp *wantypes.CredentialAssertionResponse) (*types.MFADevice, error) {
	lf := &loginFlow{
		U2F:         f.U2F,
		Webauthn:    f.Webauthn,
		identity:    mfaIdentity{f.Identity},
		sessionData: (*userSessionStorage)(f),
	}
	dev, _, err := lf.finish(ctx, user, resp, false /* passwordless */)
	return dev, trace.Wrap(err)
}

type mfaIdentity struct {
	LoginIdentity
}

func (m mfaIdentity) GetTeleportUserByWebauthnID(_ context.Context, _ []byte) (string, error) {
	return "", errors.New("lookup by webauthn ID not supported for MFA")
}

// userSessionStorage implements sessionIdentity using LoginFlow.
type userSessionStorage LoginFlow

func (s *userSessionStorage) Upsert(ctx context.Context, user string, sd *wantypes.SessionData) error {
	return s.Identity.UpsertWebauthnSessionData(ctx, user, scopeLogin, sd)
}

func (s *userSessionStorage) Get(ctx context.Context, user string, _ string) (*wantypes.SessionData, error) {
	return s.Identity.GetWebauthnSessionData(ctx, user, scopeLogin)
}

func (s *userSessionStorage) Delete(ctx context.Context, user string, _ string) error {
	return s.Identity.DeleteWebauthnSessionData(ctx, user, scopeLogin)
}
