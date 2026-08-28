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
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	wan "github.com/go-webauthn/webauthn/webauthn"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// RegistrationIdentity represents the subset of Identity methods used by
// RegistrationFlow.
type RegistrationIdentity interface {
	UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error
	GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error)
	GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error)

	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error
	UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error
	GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error)
	DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error
}

// WithInMemorySessionData returns a RegistrationIdentity implementation that
// keeps SessionData in memory.
func WithInMemorySessionData(identity RegistrationIdentity) RegistrationIdentity {
	return &inMemoryIdentity{
		RegistrationIdentity: identity,
		sessionData:          make(map[string]*wantypes.SessionData),
	}
}

type inMemoryIdentity struct {
	RegistrationIdentity

	// mu guards the fields below it.
	// We don't foresee concurrent use for inMemoryIdentity, but it's easy enough
	// to play it safe.
	mu          sync.RWMutex
	sessionData map[string]*wantypes.SessionData
}

func (identity *inMemoryIdentity) UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error {
	identity.mu.Lock()
	defer identity.mu.Unlock()
	identity.sessionData[sessionDataKey(user, sessionID)] = sd
	return nil
}

func (identity *inMemoryIdentity) GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error) {
	identity.mu.RLock()
	defer identity.mu.RUnlock()
	sd, ok := identity.sessionData[sessionDataKey(user, sessionID)]
	if !ok {
		return nil, trace.NotFound("session data for user %v not found ", user)
	}
	// The only known caller of GetWebauthnSessionData is the webauthn package
	// itself, so we trust it to not modify the SessionData we are handing back.
	return sd, nil
}

func (identity *inMemoryIdentity) DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error {
	key := sessionDataKey(user, sessionID)
	identity.mu.Lock()
	defer identity.mu.Unlock()
	if _, ok := identity.sessionData[key]; !ok {
		return trace.NotFound("session data for user %v not found ", user)
	}
	delete(identity.sessionData, key)
	return nil
}

func sessionDataKey(user, sessionID string) string {
	return fmt.Sprintf("%v/%v", user, sessionID)
}

// RegistrationFlow represents the WebAuthn registration ceremony.
//
// Registration consists of:
//
//  1. Client requests a CredentialCreation (containing a challenge and various
//     settings that may constrain allowed authenticators).
//  2. Server runs Begin(), generates a credential creation.
//  3. Client validates the credential creation, performs a user presence test
//     (usually by asking the user to touch a secure token), and replies with a
//     CredentialCreationResponse (containing the signed challenge and
//     information about the credential and authenticator)
//  4. Server runs Finish()
//  5. If all server-side checks are successful, then registration is complete
//     and the authenticator may now be used to login.
type RegistrationFlow struct {
	Webauthn *types.Webauthn
	Identity RegistrationIdentity
}

// Begin is the first step of the registration ceremony.
// The CredentialCreation created is relayed back to the client, who in turn
// performs a user presence check and signs the challenge contained within it.
// If passwordless is set, then registration asks the authenticator for a
// resident key.
// As a side effect Begin may assign (and record in storage) a WebAuthn ID for
// the user.
func (f *RegistrationFlow) Begin(ctx context.Context, user string, passwordless bool) (*wantypes.CredentialCreation, error) {
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	// Exclude known devices from the ceremony.
	devices, err := f.Identity.GetMFADevices(ctx, user, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var exclusions []protocol.CredentialDescriptor
	for _, dev := range devices {
		// Skip existing U2F devices, letting users "upgrade" their registration is
		// good for us.
		if dev.GetU2F() != nil {
			continue
		}

		// Let authenticator "upgrades" from non-resident (MFA) to resident
		// (passwordless) happen, but prevent "downgrades" from resident to
		// non-resident.
		//
		// Modern passkey implementations will "disobey" our MFA registrations and
		// actually create passkeys, silently replacing the old passkey with the new
		// "MFA" key, which can make Teleport confused (for example, by letting the
		// "MFA" key be deleted because Teleport thinks the passkey still exists).
		if webDev := dev.GetWebauthn(); webDev != nil && !webDev.ResidentKey && passwordless {
			continue
		}

		cred, ok := deviceToCredential(dev, true /* idOnly */, nil /* currentFlags */)
		if !ok {
			continue
		}
		exclusions = append(exclusions, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred.ID,
		})
	}

	webID, err := upsertOrGetWebID(ctx, user, f.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := newWebUser(webUserOpts{
		name:             user,
		webID:            webID,
		credentialIDOnly: true,
	})

	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    f.Webauthn.RPID,
		requireResidentKey:      passwordless,
		requireUserVerification: passwordless,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cc, sessionData, err := web.BeginRegistration(
		u,
		wan.WithExclusions(exclusions),
		wan.WithExtensions(protocol.AuthenticationExtensions{
			// Query authenticator on whether the resulting credential is resident,
			// despite our requirements.
			wantypes.CredPropsExtension: true,
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Send U2F App ID back in creation requests too. Useful to
	//  detect duplicate devices.

	sd, err := wantypes.SessionDataFromProtocol(sessionData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := f.Identity.UpsertWebauthnSessionData(ctx, user, scopeSession, sd); err != nil {
		return nil, trace.Wrap(err)
	}

	return wantypes.CredentialCreationFromProtocol(cc), nil
}

func upsertOrGetWebID(ctx context.Context, user string, identity RegistrationIdentity) ([]byte, error) {
	wla, err := identity.GetWebauthnLocalAuth(ctx, user)
	switch {
	case trace.IsNotFound(err): // first-time user, create a new ID
		webID := []byte(uuid.New().String())
		err := identity.UpsertWebauthnLocalAuth(ctx, user, &types.WebauthnLocalAuth{
			UserID: webID,
		})
		return webID[:], trace.Wrap(err)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	// Attempt to fix the webID->user index, if necessary.
	// This applies to legacy (Teleport 8.x) registrations and to eventual bad
	// writes.
	indexedUser, err := identity.GetTeleportUserByWebauthnID(ctx, wla.UserID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if indexedUser != user {
		// Re-write wla to force an index update.
		if err := identity.UpsertWebauthnLocalAuth(ctx, user, wla); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return wla.UserID, nil
}

// RegisterResponse represents fields needed to finish registering a new
// WebAuthn device.
type RegisterResponse struct {
	// User is the device owner.
	User string
	// DeviceName is the name for the new device.
	DeviceName string
	// CreationResponse is the response from the new device.
	CreationResponse *wantypes.CredentialCreationResponse
	// Passwordless is true if this is expected to be a passwordless registration.
	// Callers may make certain concessions when processing passwordless
	// registration (such as skipping password validation), this flag reflects that.
	// The data stored in the Begin SessionData must match the passwordless flag,
	// otherwise the registration is denied.
	Passwordless bool
}

// Finish is the second and last step of the registration ceremony.
// If successful, it returns the created MFADevice. Finish has the side effect
// or writing the device to storage (using its Identity interface).
func (f *RegistrationFlow) Finish(ctx context.Context, req RegisterResponse) (*types.MFADevice, error) {
	switch {
	case req.User == "":
		return nil, trace.BadParameter("user required")
	case req.DeviceName == "":
		return nil, trace.BadParameter("device name required")
	case req.CreationResponse == nil:
		return nil, trace.BadParameter("credential creation response required")
	}

	parsedResp, err := parseCredentialCreationResponse(req.CreationResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	origin := parsedResp.Response.CollectedClientData.Origin
	if err := validateOrigin(origin, f.Webauthn.RPID); err != nil {
		log.DebugContext(ctx, "origin validation failed", "error", err)
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Verify that the public key matches the allowed
	//  credential params? It doesn't look like duo-labs/webauthn does that.

	wla, err := f.Identity.GetWebauthnLocalAuth(ctx, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := newWebUser(webUserOpts{
		name:             req.User,
		webID:            wla.UserID,
		credentialIDOnly: true,
	})

	sd, err := f.Identity.GetWebauthnSessionData(ctx, req.User, scopeSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionData := wantypes.SessionDataToProtocol(sd)

	// Activate passwordless switches (resident key, user verification) if we
	// required verification in the begin step.
	passwordless := sessionData.UserVerification == protocol.VerificationRequired
	if req.Passwordless && !passwordless {
		return nil, trace.BadParameter("passwordless registration failed, requested CredentialCreation was for an MFA registration")
	}

	web, err := newWebAuthn(webAuthnParams{
		cfg:                     f.Webauthn,
		rpID:                    f.Webauthn.RPID,
		origin:                  origin,
		requireResidentKey:      passwordless,
		requireUserVerification: passwordless,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credential, err := web.CreateCredential(u, *sessionData, parsedResp)
	if err != nil {
		// Use a more friendly message for certain verification errors.
		protocolErr := &protocol.Error{}
		if errors.As(err, &protocolErr) &&
			protocolErr.Type == protocol.ErrVerification.Type &&
			passwordless &&
			!parsedResp.Response.AttestationObject.AuthData.Flags.UserVerified() {
			log.DebugContext(ctx, "WebAuthn: Replacing verification error with PIN message", "error", err)
			return nil, trace.BadParameter("authenticator doesn't support passwordless, setting up a PIN may fix this")
		}

		return nil, trace.Wrap(err)
	}

	// Finally, check against attestation settings, if any.
	// This runs after web.CreateCredential so we can take advantage of the
	// attestation format validation it performs.
	if err := verifyAttestation(f.Webauthn, parsedResp.Response.AttestationObject); err != nil {
		return nil, trace.Wrap(err)
	}

	newDevice, err := types.NewMFADevice(req.DeviceName, uuid.NewString() /* id */, time.Now() /* addedAt */, &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:      credential.ID,
			PublicKeyCbor:     credential.PublicKey,
			AttestationType:   credential.AttestationType,
			Aaguid:            credential.Authenticator.AAGUID,
			SignatureCounter:  credential.Authenticator.SignCount,
			AttestationObject: req.CreationResponse.AttestationResponse.AttestationObject,
			ResidentKey:       req.Passwordless || hasCredPropsRK(req.CreationResponse),
			CredentialRpId:    f.Webauthn.RPID,
			CredentialBackupEligible: &gogotypes.BoolValue{
				Value: credential.Flags.BackupEligible,
			},
			CredentialBackedUp: &gogotypes.BoolValue{
				Value: credential.Flags.BackupState,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We delegate a few checks to identity, including:
	// * The validity of the created MFADevice
	// * Uniqueness validation of the deviceName
	// * Uniqueness validation of the Webauthn credential ID.
	if err := f.Identity.UpsertMFADevice(ctx, req.User, newDevice); err != nil {
		return nil, trace.Wrap(err)
	}

	// Registration complete, remove the registration challenge we just used.
	if err := f.Identity.DeleteWebauthnSessionData(ctx, req.User, scopeSession); err != nil {
		log.WarnContext(ctx, "failed to delete registration SessionData for user", "user", req.User, "error", err)
	}

	return newDevice, nil
}

func parseCredentialCreationResponse(resp *wantypes.CredentialCreationResponse) (*protocol.ParsedCredentialCreationData, error) {
	// Remove extensions before marshaling, duo-labs/webauthn isn't expecting it.
	exts := resp.Extensions
	resp.Extensions = nil
	defer func() {
		resp.Extensions = exts
	}()

	// This is a roundabout way of getting resp validated, but unfortunately the
	// APIs don't provide a better method (and it seems better than duplicating
	// library code here).
	body, err := json.Marshal(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedResp, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	return parsedResp, trace.Wrap(err)
}

func hasCredPropsRK(ccr *wantypes.CredentialCreationResponse) bool {
	return ccr != nil &&
		ccr.Extensions != nil &&
		ccr.Extensions.CredProps != nil &&
		ccr.Extensions.CredProps.RK
}
