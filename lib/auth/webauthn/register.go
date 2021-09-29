// Copyright 2021 Gravitational, Inc
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	wan "github.com/duo-labs/webauthn/webauthn"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	log "github.com/sirupsen/logrus"
)

// registrationSessionID is used as the per-user session identifier for
// registrations.
// A fixed identifier means that only one concurrent registration is allowed
// (excluding registrations using in-memory SessionData storage).
const registrationSessionID = "registration"

// RegistrationIdentity represents the subset of Identity methods used by
// RegistrationFlow.
type RegistrationIdentity interface {
	userIDStorage

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
// 1. Client requests a CredentialCreation (containing a challenge and various
//    settings that may constrain allowed authenticators).
// 2. Server runs Begin(), generates a credential creation.
// 3. Client validates the credential creation, performs a user presence test
//    (usually by asking the user to touch a secure token), and replies with a
//    CredentialCreationResponse (containing the signed challenge and
//    information about the credential and authenticator)
// 4. Server runs Finish()
// 5. If all server-side checks are successful, then registration is complete
//    and the authenticator may now be used to login.
type RegistrationFlow struct {
	Webauthn *types.Webauthn
	Identity RegistrationIdentity
}

// Begin is the first step of the registration ceremony.
// The CredentialCreation created is relayed back to the client, who in turn
// performs a user presence check and signs the challenge contained within it.
// As a side effect Begin may assign (and record in storage) a WebAuthn ID for
// the user.
func (f *RegistrationFlow) Begin(ctx context.Context, user string) (*CredentialCreation, error) {
	switch {
	case f.Webauthn.Disabled:
		return nil, trace.BadParameter("webauthn disabled")
	case user == "":
		return nil, trace.BadParameter("user required")
	}

	// Exclude known devices from the ceremony.
	devices, err := f.Identity.GetMFADevices(ctx, user, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var exclusions []protocol.CredentialDescriptor
	for _, dev := range devices {
		cred, ok := deviceToCredential(dev, true /* idOnly */)
		if !ok {
			continue
		}
		exclusions = append(exclusions, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred.ID,
		})
	}

	webID, err := getOrCreateUserWebauthnID(ctx, user, f.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := newWebUser(user, webID, true /* credentialIDOnly */, nil /* devices */)

	web, err := newWebAuthn(f.Webauthn, f.Webauthn.RPID, "" /* origin */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentialCreation, sessionData, err := web.BeginRegistration(u, wan.WithExclusions(exclusions))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Send U2F App ID back in creation requests too. Useful to
	//  detect duplicate devices.

	sessionDataPB, err := sessionToPB(sessionData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := f.Identity.UpsertWebauthnSessionData(ctx, user, registrationSessionID, sessionDataPB); err != nil {
		return nil, trace.Wrap(err)
	}

	return (*CredentialCreation)(credentialCreation), nil
}

// Finish is the second and last step of the registration ceremony.
// If successful, it returns the created MFADevice. Finish has the side effect
// or writing the device to storage (using its Identity interface).
func (f *RegistrationFlow) Finish(ctx context.Context, user, deviceName string, resp *CredentialCreationResponse) (*types.MFADevice, error) {
	switch {
	case f.Webauthn.Disabled:
		return nil, trace.BadParameter("webauthn disabled")
	case user == "":
		return nil, trace.BadParameter("user required")
	case deviceName == "":
		return nil, trace.BadParameter("device name required")
	case resp == nil:
		return nil, trace.BadParameter("credential creation response required")
	}

	parsedResp, err := parseCredentialCreationResponse(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	origin := parsedResp.Response.CollectedClientData.Origin
	if err := validateOrigin(origin, f.Webauthn.RPID); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Verify that the public key matches the allowed
	//  credential params? It doesn't look like duo-labs/webauthn does that.

	wla, err := f.Identity.GetWebauthnLocalAuth(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := newWebUser(user, wla.UserID, true /* credentialIDOnly */, nil /* devices */)

	sessionDataPB, err := f.Identity.GetWebauthnSessionData(ctx, user, registrationSessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionData := sessionFromPB(sessionDataPB)

	web, err := newWebAuthn(f.Webauthn, f.Webauthn.RPID, origin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credential, err := web.CreateCredential(u, *sessionData, parsedResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Finally, check against attestation settings, if any.
	// This runs after web.CreateCredential so we can take advantage of the
	// attestation format validation it performs.
	if err := verifyAttestation(f.Webauthn, parsedResp.Response.AttestationObject); err != nil {
		return nil, trace.Wrap(err)
	}

	newDevice := types.NewMFADevice(deviceName, uuid.NewString() /* id */, time.Now() /* addedAt */)
	newDevice.Device = &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:     credential.ID,
			PublicKeyCbor:    credential.PublicKey,
			AttestationType:  credential.AttestationType,
			Aaguid:           credential.Authenticator.AAGUID,
			SignatureCounter: credential.Authenticator.SignCount,
		},
	}
	// We delegate a few checks to identity, including:
	// * The validity of the created MFADevice
	// * Uniqueness validation of the deviceName
	// * Uniqueness validation of the Webauthn credential ID.
	if err := f.Identity.UpsertMFADevice(ctx, user, newDevice); err != nil {
		return nil, trace.Wrap(err)
	}

	// Registration complete, remove the registration challenge we just used.
	if err := f.Identity.DeleteWebauthnSessionData(ctx, user, registrationSessionID); err != nil {
		log.Warnf("WebAuthn: failed to delete registration SessionData for user %v", user)
	}

	return newDevice, nil
}

func parseCredentialCreationResponse(resp *CredentialCreationResponse) (*protocol.ParsedCredentialCreationData, error) {
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
