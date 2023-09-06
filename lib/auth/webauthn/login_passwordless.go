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

package webauthn

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/gravitational/teleport/api/types"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// PasswordlessIdentity represents the subset of Identity methods used by
// PasswordlessFlow.
type PasswordlessIdentity interface {
	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error

	UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wanpb.SessionData) error
	GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wanpb.SessionData, error)
	DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error
	GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error)
}

// PasswordlessFlow provides passwordless authentication.
type PasswordlessFlow struct {
	Webauthn                 *types.Webauthn
	Identity                 PasswordlessIdentity
	UserVerificationRequired bool
}

// Begin is the first step of the passwordless login flow.
// It works similarly to LoginFlow.Begin, but it doesn't require a Teleport
// username nor implies a previous password-validation step.
func (f *PasswordlessFlow) Begin(ctx context.Context) (*wantypes.CredentialAssertion, error) {
	lf := &loginFlow{
		Webauthn:                 f.Webauthn,
		identity:                 passwordlessIdentity{f.Identity},
		sessionData:              (*globalSessionStorage)(f),
		UserVerificationRequired: f.UserVerificationRequired,
	}
	return lf.begin(ctx, "" /* user */, true /* passwordless */)
}

// Finish is the last step of the passwordless login flow.
// It works similarly to LoginFlow.Finish, but the user identity is established
// via the response UserHandle, instead of an explicit Teleport username.
func (f *PasswordlessFlow) Finish(ctx context.Context, resp *wantypes.CredentialAssertionResponse) (*types.MFADevice, string, error) {
	lf := &loginFlow{
		Webauthn:                 f.Webauthn,
		identity:                 passwordlessIdentity{f.Identity},
		sessionData:              (*globalSessionStorage)(f),
		UserVerificationRequired: f.UserVerificationRequired,
	}
	return lf.finish(ctx, "" /* user */, resp, true /* passwordless */)
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

func (g *globalSessionStorage) Upsert(ctx context.Context, user string, sd *wanpb.SessionData) error {
	id := base64.RawURLEncoding.EncodeToString(sd.Challenge)
	return g.Identity.UpsertGlobalWebauthnSessionData(ctx, scopeLogin, id, sd)
}

func (g *globalSessionStorage) Get(ctx context.Context, user string, challenge string) (*wanpb.SessionData, error) {
	return g.Identity.GetGlobalWebauthnSessionData(ctx, scopeLogin, challenge)
}

func (g *globalSessionStorage) Delete(ctx context.Context, user string, challenge string) error {
	return g.Identity.DeleteGlobalWebauthnSessionData(ctx, scopeLogin, challenge)
}
