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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"sort"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	wan "github.com/go-webauthn/webauthn/webauthn"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
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

func (f *loginFlow) begin(ctx context.Context, user string, challengeExtensions *mfav1.ChallengeExtensions) (*wantypes.CredentialAssertion, error) {
	if challengeExtensions == nil {
		return nil, trace.BadParameter("requested challenge extensions must be supplied.")
	}

	if challengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES && challengeExtensions.Scope != mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION {
		return nil, trace.BadParameter("mfa challenges with scope %s cannot allow reuse", challengeExtensions.Scope)
	}

	// discoverableLogin identifies logins started with an unknown/empty user.
	discoverableLogin := challengeExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN
	if user == "" && !discoverableLogin {
		return nil, trace.BadParameter("user required")
	}

	var u *webUser
	if discoverableLogin {
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

		// Filter devices with the wrong RPID and log an error.
		foundInvalid := false
		for i := 0; i < len(devices); i++ {
			webDev := devices[i].GetWebauthn()
			if webDev == nil || webDev.CredentialRpId == "" || webDev.CredentialRpId == f.Webauthn.RPID {
				continue
			}

			const msg = "User device has unexpected RPID, excluding from allowed credentials. " +
				"RPID changes are not supported by WebAuthn, this is likely to cause permanent authentication problems for your users. " +
				"Consider reverting the change or reset your users so they may register their devices again."
			log.ErrorContext(ctx, msg,
				"user", user,
				"device", devices[i].GetName(),
				"rpid", webDev.CredentialRpId,
			)

			// "Cut" device from slice.
			devices = slices.Delete(devices, i, i+1)
			i--

			foundInvalid = true
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

		u = newWebUser(webUserOpts{
			name:             user,
			webID:            webID,
			devices:          devices,
			credentialIDOnly: true,
		})

		// Let's make sure we have at least one registered credential here, since we
		// have to allow zero credentials for passwordless below.
		if len(u.credentials) == 0 {
			if foundInvalid {
				return nil, trace.Wrap(ErrInvalidCredentials)
			}
			return nil, trace.NotFound("found no credentials for user %q", user)
		}
	}

	// TODO(codingllama): Use the "official" appid impl by duo-labs/webauthn.
	var opts []wan.LoginOption
	if f.U2F != nil && f.U2F.AppID != "" {
		// See https://www.w3.org/TR/webauthn-2/#sctn-appid-extension.
		opts = append(opts, wan.WithAssertionExtensions(protocol.AuthenticationExtensions{
			wantypes.AppIDExtension: f.U2F.AppID,
		}))
	}
	// Set the user verification requirement, if present, only for
	// non-discoverable logins.
	// For discoverable logins we rely on the wan.WebAuthn default set below.
	if !discoverableLogin && challengeExtensions.UserVerificationRequirement != "" {
		uvr := protocol.UserVerificationRequirement(challengeExtensions.UserVerificationRequirement)
		opts = append(opts, wan.WithUserVerification(uvr))
	}

	// Create the WebAuthn object and issue a new challenge.
	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    f.Webauthn.RPID,
		requireUserVerification: discoverableLogin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var assertion *protocol.CredentialAssertion
	var sessionData *wan.SessionData
	if discoverableLogin {
		assertion, sessionData, err = web.BeginDiscoverableLogin(opts...)
	} else {
		assertion, sessionData, err = web.BeginLogin(u, opts...)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Store SessionData - it's checked against the user response by Finish.
	sd, err := wantypes.SessionDataFromProtocol(sessionData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sd.ChallengeExtensions = &mfatypes.ChallengeExtensions{
		Scope:                       challengeExtensions.Scope,
		AllowReuse:                  challengeExtensions.AllowReuse,
		UserVerificationRequirement: challengeExtensions.UserVerificationRequirement,
	}

	if err := f.sessionData.Upsert(ctx, user, sd); err != nil {
		return nil, trace.Wrap(err)
	}

	return wantypes.CredentialAssertionFromProtocol(assertion), nil
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

// LoginData is data gathered from a successful webauthn login.
type LoginData struct {
	// User is the Teleport user.
	User string
	// Device is the MFA device used to authenticate the user.
	Device *types.MFADevice
	// AllowReuse is whether the webauthn challenge used for this login
	// can be reused by the user for subsequent logins, until it expires.
	AllowReuse mfav1.ChallengeAllowReuse
}

func (f *loginFlow) finish(ctx context.Context, user string, resp *wantypes.CredentialAssertionResponse, requiredExtensions *mfav1.ChallengeExtensions) (*LoginData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("requested challenge extensions must be supplied.")
	}

	discoverableLogin := requiredExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN

	switch {
	case user == "" && !discoverableLogin:
		return nil, trace.BadParameter("user required")
	case resp == nil:
		// resp != nil is good enough to proceed, we leave remaining validations to
		// duo-labs/webauthn.
		return nil, trace.BadParameter("credential assertion response required")
	}

	parsedResp, err := parseCredentialResponse(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	origin := parsedResp.Response.CollectedClientData.Origin
	if err := validateOrigin(origin, f.Webauthn.RPID); err != nil {
		log.DebugContext(ctx, "origin validation failed", "error", err)
		return nil, trace.Wrap(err)
	}

	var webID []byte
	if discoverableLogin {
		webID = parsedResp.Response.UserHandle
		if len(webID) == 0 {
			return nil, trace.BadParameter("webauthn user handle required for passwordless")
		}

		// Fetch user from WebAuthn UserHandle (aka User ID).
		teleportUser, err := f.identity.GetTeleportUserByWebauthnID(ctx, webID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user = teleportUser
	} else {
		webID, err = f.getWebID(ctx, user)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Find the device used to sign the credentials. It must be a previously
	// registered device.
	devices, err := f.identity.GetMFADevices(ctx, user, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dev, ok := findDeviceByID(devices, parsedResp.RawID)
	if !ok {
		return nil, trace.BadParameter(
			"unknown device credential: %q", base64.RawURLEncoding.EncodeToString(parsedResp.RawID))
	}

	// Is an U2F device trying to login? If yes, use RPID = App ID.
	// Technically browsers should reply with the appid extension set to true[1],
	// but in actuality they don't send anything.
	// [1] https://www.w3.org/TR/webauthn-2/#sctn-appid-extension.
	rpID := f.Webauthn.RPID
	switch {
	case dev.GetU2F() != nil && f.U2F == nil:
		return nil, trace.BadParameter("U2F device attempted login, but U2F configuration not present")
	case dev.GetU2F() != nil:
		rpID = f.U2F.AppID
	}
	u := newWebUser(webUserOpts{
		name:    user,
		webID:   webID,
		devices: []*types.MFADevice{dev},
		currentFlags: &credentialFlags{
			BE: parsedResp.Response.AuthenticatorData.Flags.HasBackupEligible(),
			BS: parsedResp.Response.AuthenticatorData.Flags.HasBackupState(),
		},
	})

	// Fetch the previously-stored SessionData, so it's checked against the user
	// response.
	challenge := parsedResp.Response.CollectedClientData.Challenge
	sd, err := f.sessionData.Get(ctx, user, challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if the given scope is satisfied by the challenge scope.
	if requiredExtensions.Scope != sd.ChallengeExtensions.Scope && requiredExtensions.Scope != mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED {
		return nil, trace.AccessDenied("required scope %q is not satisfied by the given webauthn session with scope %q", requiredExtensions.Scope, sd.ChallengeExtensions.Scope)
	}

	// If this session is reusable, but this login forbids reusable sessions, return an error.
	if requiredExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO && sd.ChallengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		return nil, trace.AccessDenied("the given webauthn session allows reuse, but reuse is not permitted in this context")
	}

	// Verify (and possibly correct) the user verification requirement.
	// A mismatch here could indicate a programming error or even foul play.
	uvr := protocol.UserVerificationRequirement(requiredExtensions.UserVerificationRequirement)
	if (discoverableLogin || uvr == protocol.VerificationRequired) && sd.UserVerification != string(protocol.VerificationRequired) {
		// This is not a failure yet, but will likely become one.
		sd.UserVerification = string(protocol.VerificationRequired)
		const msg = "User verification required by extensions but not by challenge. " +
			"Increased SessionData.UserVerification."
		log.WarnContext(ctx, msg, "user_verification", sd.UserVerification)
	}

	sessionData := wantypes.SessionDataToProtocol(sd)

	// Make sure _all_ credentials in the session are accounted for by the user.
	// webauthn.ValidateLogin requires it.
	for _, allowedCred := range sessionData.AllowedCredentialIDs {
		if bytes.Equal(parsedResp.RawID, allowedCred) {
			continue
		}
		u.credentials = append(u.credentials, wan.Credential{
			ID: allowedCred,
		})
	}

	// Create a WebAuthn matching the expected RPID and Origin, then verify the
	// signed challenge.
	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    rpID,
		origin:                  origin,
		requireUserVerification: discoverableLogin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var credential *wan.Credential
	if discoverableLogin {
		discoverUser := func(_, _ []byte) (wan.User, error) { return u, nil }
		credential, err = web.ValidateDiscoverableLogin(discoverUser, *sessionData, parsedResp)
	} else {
		credential, err = web.ValidateLogin(u, *sessionData, parsedResp)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if credential.Authenticator.CloneWarning {
		log.WarnContext(ctx, "Clone warning detected for device, the device counter may be malfunctioning",
			"user", user,
			"device", dev.GetName(),
		)
	}

	// Update last used timestamp and device counter.
	if err := updateCredentialAndTimestamps(dev, credential, discoverableLogin); err != nil {
		return nil, trace.Wrap(err)
	}
	// Retroactively write the credential RPID, now that it cleared authn.
	if webDev := dev.GetWebauthn(); webDev != nil && webDev.CredentialRpId == "" {
		log.DebugContext(ctx, "Recording RPID in device",
			"rpid", rpID,
			"user", user,
			"device", dev.GetName(),
		)
		webDev.CredentialRpId = rpID
	}

	if err := f.identity.UpsertMFADevice(ctx, user, dev); err != nil {
		return nil, trace.Wrap(err)
	}

	// The user just solved the challenge, so let's make sure it won't be used
	// again, unless reuse is explicitly allowed.
	// Note that even reusable sessions are deleted when their expiration time
	// passes.
	if sd.ChallengeExtensions.AllowReuse != mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		if err := f.sessionData.Delete(ctx, user, challenge); err != nil {
			log.WarnContext(ctx, "failed to delete login SessionData for user",
				"user", user,
				"scope", sd.ChallengeExtensions.Scope,
			)
		}
	}

	return &LoginData{
		User:       user,
		Device:     dev,
		AllowReuse: sd.ChallengeExtensions.AllowReuse,
	}, nil
}

func parseCredentialResponse(resp *wantypes.CredentialAssertionResponse) (*protocol.ParsedCredentialAssertionData, error) {
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

func updateCredentialAndTimestamps(
	dest *types.MFADevice,
	credential *wan.Credential,
	discoverableLogin bool,
) error {
	switch d := dest.Device.(type) {
	case *types.MFADevice_U2F:
		d.U2F.Counter = credential.Authenticator.SignCount

	case *types.MFADevice_Webauthn:
		d.Webauthn.SignatureCounter = credential.Authenticator.SignCount

		// Backfill ResidentKey field.
		// This may happen if an authenticator created for "MFA" was actually
		// resident all along (eg, Safari/Touch ID registrations).
		if discoverableLogin && !d.Webauthn.ResidentKey {
			d.Webauthn.ResidentKey = true
		}

		// Backfill BE/BS bits.
		if d.Webauthn.CredentialBackupEligible == nil {
			d.Webauthn.CredentialBackupEligible = &gogotypes.BoolValue{
				Value: credential.Flags.BackupEligible,
			}
			log.DebugContext(context.Background(), "Backfilled Webauthn device BE flag",
				"device", dest.GetName(),
				"be", credential.Flags.BackupEligible,
			)
		}
		if d.Webauthn.CredentialBackedUp == nil {
			d.Webauthn.CredentialBackedUp = &gogotypes.BoolValue{
				Value: credential.Flags.BackupState,
			}
			log.DebugContext(context.Background(), "Backfilled Webauthn device BS flag",
				"device", dest.GetName(),
				"bs", credential.Flags.BackupState,
			)
		}

	default:
		return trace.BadParameter("unexpected device type for webauthn: %T", d)
	}
	dest.LastUsed = time.Now()
	return nil
}
