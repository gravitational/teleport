/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webauthn

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	wan "github.com/duo-labs/webauthn/webauthn"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	log "github.com/sirupsen/logrus"
)

// loginIdentity contains the subset of services.Identity methods used by
// loginFlow.
type loginIdentity interface {
	GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) // MFA
	GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error)           // Passwordless

	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error
}

// sessionIdentity abstracts operations over SessionData storage.
// * MFA uses per-user variants
// (services.Identity.Update/Get/DeleteWebauthnSessionData methods).
// * Passwordless uses global variants
// (services.Identity.Update/Get/DeleteGlobalWebauthnSessionData methods).
type sessionIdentity interface {
	Upsert(ctx context.Context, user string, sd *wantypes.SessionData) error
	Get(ctx context.Context, user string, challenge string) (*wantypes.SessionData, error)
	Delete(ctx context.Context, user string, challenge string) error
}

// loginFlow implements both MFA and Passwordless authentication, exposing an
// interface that is the union of both login methods.
type loginFlow struct {
	U2F         *types.U2F
	Webauthn    *types.Webauthn
	identity    loginIdentity
	sessionData sessionIdentity
}

func (f *loginFlow) begin(ctx context.Context, user string, passwordless bool) (*CredentialAssertion, error) {
	if user == "" && !passwordless {
		return nil, trace.BadParameter("user required")
	}

	var u *webUser
	if passwordless {
		u = &webUser{} // Issue anonymous challenge.
	} else {
		webID, err := f.getWebID(ctx, user)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Use existing devices to set the allowed credentials.
		devices, err := f.identity.GetMFADevices(ctx, user, false /* withSecrets */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Sort non-resident keys first, which may cause clients to favor them for
		// MFA in some scenarios (eg, tsh).
		sort.Slice(devices, func(i, j int) bool {
			dev1, dev2 := devices[i], devices[j]
			web1, web2 := dev1.GetWebauthn(), dev2.GetWebauthn()
			resident1 := web1 != nil && web1.ResidentKey
			resident2 := web2 != nil && web2.ResidentKey
			return !resident1 && resident2
		})

		u = newWebUser(user, webID, true /* credentialIDOnly */, devices)

		// Let's make sure we have at least one registered credential here, since we
		// have to allow zero credentials for passwordless below.
		if len(u.credentials) == 0 {
			return nil, trace.NotFound("found no credentials for user %q", user)
		}
	}

	var opts []wan.LoginOption
	if f.U2F != nil && f.U2F.AppID != "" {
		// See https://www.w3.org/TR/webauthn-2/#sctn-appid-extension.
		opts = append(opts, wan.WithAssertionExtensions(protocol.AuthenticationExtensions{
			AppIDExtension: f.U2F.AppID,
		}))
	}

	// Create the WebAuthn object and issue a new challenge.
	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    f.Webauthn.RPID,
		requireUserVerification: passwordless,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assertion, sessionData, err := beginLogin(passwordless, web, u, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Store SessionData - it's checked against the user response by Finish.
	sessionDataPB, err := sessionToPB(sessionData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := f.sessionData.Upsert(ctx, user, sessionDataPB); err != nil {
		return nil, trace.Wrap(err)
	}

	return (*CredentialAssertion)(assertion), nil
}

func (f *loginFlow) getWebID(ctx context.Context, user string) ([]byte, error) {
	wla, err := f.identity.GetWebauthnLocalAuth(ctx, user)
	switch {
	case trace.IsNotFound(err):
		return nil, nil // OK, legacy U2F users may not have a webID.
	case err != nil:
		return nil, trace.Wrap(err)
	}
	return wla.UserID, nil
}

func beginLogin(
	passwordless bool,
	web *wan.WebAuthn, user *webUser, opts ...wan.LoginOption) (*protocol.CredentialAssertion, *wan.SessionData, error) {
	// web.BeginLogin does a length check in the users' credentials, but we have
	// no known credentials at this stage for passwordless logins.
	// This leaves us with two options: copy and modify BeginLogin, or code
	// around it so passwordless goes through. Since copying makes it harder to
	// apply or benefit from future library updates, coding around it is the
	// option of choice.

	if passwordless {
		// Add a mock credential to pass the BeginLogin check.
		user.credentials = append(user.credentials, wan.Credential{})
		defer func() { user.credentials = nil }()
	}

	assertion, sessionData, err := web.BeginLogin(user, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if passwordless {
		// Remove mock credential from resources.
		assertion.Response.AllowedCredentials = nil
		sessionData.AllowedCredentialIDs = nil
	}

	return assertion, sessionData, nil
}

func (f *loginFlow) finish(ctx context.Context, user string, resp *CredentialAssertionResponse, passwordless bool) (*types.MFADevice, string, error) {
	switch {
	case user == "" && !passwordless:
		return nil, "", trace.BadParameter("user required")
	case resp == nil:
		// resp != nil is good enough to proceed, we leave remaining validations to
		// duo-labs/webauthn.
		return nil, "", trace.BadParameter("credential assertion response required")
	}

	parsedResp, err := parseCredentialResponse(resp)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	origin := parsedResp.Response.CollectedClientData.Origin
	if err := validateOrigin(origin, f.Webauthn.RPID); err != nil {
		log.WithError(err).Debugf("WebAuthn: origin validation failed")
		return nil, "", trace.Wrap(err)
	}

	var webID []byte
	if passwordless {
		webID = parsedResp.Response.UserHandle
		if len(webID) == 0 {
			return nil, "", trace.BadParameter("webauthn user handle required for passwordless")
		}

		// Fetch user from WebAuthn UserHandle (aka User ID).
		teleportUser, err := f.identity.GetTeleportUserByWebauthnID(ctx, webID)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		user = teleportUser
	} else {
		var err error
		webID, err = f.getWebID(ctx, user)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	// Find the device used to sign the credentials. It must be a previously
	// registered device.
	devices, err := f.identity.GetMFADevices(ctx, user, false /* withSecrets */)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	dev, ok := findDeviceByID(devices, parsedResp.RawID)
	if !ok {
		return nil, "", trace.BadParameter(
			"unknown device credential: %q", base64.RawURLEncoding.EncodeToString(parsedResp.RawID))
	}

	// Is an U2F device trying to login? If yes, use RPID = App ID.
	// Technically browsers should reply with the appid extension set to true[1],
	// but in actuality they don't send anything.
	// [1] https://www.w3.org/TR/webauthn-2/#sctn-appid-extension.
	rpID := f.Webauthn.RPID
	switch {
	case dev.GetU2F() != nil && f.U2F == nil:
		return nil, "", trace.BadParameter("U2F device attempted login, but U2F configuration not present")
	case dev.GetU2F() != nil:
		rpID = f.U2F.AppID
	}
	u := newWebUser(user, webID, false /* credentialIDOnly */, []*types.MFADevice{dev})

	// Fetch the previously-stored SessionData, so it's checked against the user
	// response.
	challenge := parsedResp.Response.CollectedClientData.Challenge
	sessionDataPB, err := f.sessionData.Get(ctx, user, challenge)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	if passwordless {
		sessionDataPB.UserId = webID // Not known on Begin, so can't be recorded.
	}
	sessionData := sessionFromPB(sessionDataPB)

	// Create a WebAuthn matching the expected RPID and Origin, then verify the
	// signed challenge.
	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    rpID,
		origin:                  origin,
		requireUserVerification: passwordless,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	credential, err := web.ValidateLogin(u, *sessionData, parsedResp)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	if credential.Authenticator.CloneWarning {
		log.Warnf(
			"WebAuthn: Clone warning detected for user %q / device %q. Device counter may be malfunctioning.", user, dev.GetName())
	}

	// Update last used timestamp and device counter.
	if err := setCounterAndTimestamps(dev, credential); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if err := f.identity.UpsertMFADevice(ctx, user, dev); err != nil {
		return nil, "", trace.Wrap(err)
	}

	// The user just solved the challenge, so let's make sure it won't be used
	// again.
	if err := f.sessionData.Delete(ctx, user, challenge); err != nil {
		log.Warnf("WebAuthn: failed to delete login SessionData for user %v (passwordless = %v)", user, passwordless)
	}

	return dev, user, nil
}

func parseCredentialResponse(resp *CredentialAssertionResponse) (*protocol.ParsedCredentialAssertionData, error) {
	// Do not pass extensions on to duo-labs/webauthn, they won't go past JSON
	// unmarshal.
	exts := resp.Extensions
	resp.Extensions = nil
	defer func() { resp.Extensions = exts }()

	// This is a roundabout way of getting resp validated, but unfortunately the
	// APIs don't provide a better method (and it seems better than duplicating
	// library code here).
	body, err := json.Marshal(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
}

func findDeviceByID(devices []*types.MFADevice, id []byte) (*types.MFADevice, bool) {
	for _, dev := range devices {
		switch d := dev.Device.(type) {
		case *types.MFADevice_U2F:
			if bytes.Equal(d.U2F.KeyHandle, id) {
				return dev, true
			}
		case *types.MFADevice_Webauthn:
			if bytes.Equal(d.Webauthn.CredentialId, id) {
				return dev, true
			}
		}
	}
	return nil, false
}

func setCounterAndTimestamps(dev *types.MFADevice, credential *wan.Credential) error {
	switch d := dev.Device.(type) {
	case *types.MFADevice_U2F:
		d.U2F.Counter = credential.Authenticator.SignCount
	case *types.MFADevice_Webauthn:
		d.Webauthn.SignatureCounter = credential.Authenticator.SignCount
	default:
		return trace.BadParameter("unexpected device type for webauthn: %T", d)
	}
	dev.LastUsed = time.Now()
	return nil
}
