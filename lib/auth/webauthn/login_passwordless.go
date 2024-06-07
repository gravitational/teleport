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
	"encoding/base64"
	"errors"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// PasswordlessIdentity represents the subset of Identity methods used by
// PasswordlessFlow.
type PasswordlessIdentity interface {
	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error

	UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wantypes.SessionData) error
	GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wantypes.SessionData, error)
	DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error
	GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error)
}

// PasswordlessFlow provides passwordless authentication.
//
// PasswordlessFlow is used mainly for the initial passwordless login.
// For UV=1 assertions after login, use [LoginFlow.Begin] with the desired
// [mfav1.ChallengeExtensions.UserVerificationRequirement].
type PasswordlessFlow struct {
	Webauthn *types.Webauthn
	Identity PasswordlessIdentity
}

// Begin is the first step of the passwordless login flow.
// It works similarly to LoginFlow.Begin, but it doesn't require a Teleport
// username nor implies a previous password-validation step.
func (f *PasswordlessFlow) Begin(ctx context.Context) (*wantypes.CredentialAssertion, error) {
	lf := &loginFlow{
		Webauthn:    f.Webauthn,
		identity:    passwordlessIdentity{f.Identity},
		sessionData: (*globalSessionStorage)(f),
	}
	chalExt := &mfav1.ChallengeExtensions{
		Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
		AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
	}
	return lf.begin(ctx, "" /* user */, chalExt)
}

// Finish is the last step of the passwordless login flow.
// It works similarly to LoginFlow.Finish, but the user identity is established
// via the response UserHandle, instead of an explicit Teleport username.
func (f *PasswordlessFlow) Finish(ctx context.Context, resp *wantypes.CredentialAssertionResponse) (*LoginData, error) {
	lf := &loginFlow{
		Webauthn:    f.Webauthn,
		identity:    passwordlessIdentity{f.Identity},
		sessionData: (*globalSessionStorage)(f),
	}
	requiredExt := &mfav1.ChallengeExtensions{
		Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
		AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
	}
	return lf.finish(ctx, "" /* user */, resp, requiredExt)
}

type passwordlessIdentity struct {
	PasswordlessIdentity
}

func (p passwordlessIdentity) UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error {
	return errors.New("webauthn local auth not supported for passwordless")
}

func (p passwordlessIdentity) GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
	return nil, errors.New("webauthn local auth not supported for passwordless")
}

type globalSessionStorage PasswordlessFlow

func (g *globalSessionStorage) Upsert(ctx context.Context, user string, sd *wantypes.SessionData) error {
	id := base64.RawURLEncoding.EncodeToString(sd.Challenge)
	return g.Identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id, sd)
}

func (g *globalSessionStorage) Get(ctx context.Context, user string, challenge string) (*wantypes.SessionData, error) {
	return g.Identity.GetGlobalWebauthnSessionData(ctx, scopeLogin, challenge)
}

func (g *globalSessionStorage) Delete(ctx context.Context, user string, challenge string) error {
	return g.Identity.DeleteGlobalWebauthnSessionData(ctx, scopeLogin, challenge)
}
