package webauthn

import (
	"context"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	wan "github.com/duo-labs/webauthn/webauthn"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
)

// CredentialAssertion is the payload sent to authenticators to initiate login.
type CredentialAssertion = protocol.CredentialAssertion

// CredentialAssertionResponse is the reply from authenticators to complete
// login.
type CredentialAssertionResponse = protocol.CredentialAssertionResponse

// loginSessionID is used as the per-user session identifier.
// A fixed identifier means, in essence, that only one concurrent login is
// allowed.
const loginSessionID = "login"

// loginIdentity represents the subset of Identity methods used by LoginFlow.
// It exists to better scope LoginFlow's use of Identity and to facilitate
// testing.
type loginIdentity interface {
	GetUser(user string, withSecrets bool) (types.User, error)
	GetMFADevices(ctx context.Context, user string) ([]*types.MFADevice, error)
	UpsertWebAuthnSessionData(user, sessionID string, sd *wantypes.SessionData) error
}

// LoginFlow represents the WebAuthn login procedure (aka authentication).
//
// The login flow consists of:
//
// 1. Client requests a CredentialAssertion (containing, among other info, a
//    challenge to be signed)
// 2. Server runs Begin(), generates a credential assertion.
// 3. Client validates the assertion, performs a user presence test (usually by
//    asking the user to touch a secure token), and replies with
//    CredentialAssertionResponse (containing the signed challenge)
// 4. Server runs Finish()
// 5. If all server-side checks are successful, then login/authentication is
//    complete.
type LoginFlow struct {
	U2F      *types.U2F
	Webauthn *Config
	// Identity is typically an implementation of the Identity service, ie, an
	// object with access to user, device and MFA storage.
	Identity loginIdentity
}

// Begin is the first step of the LoginFlow.
// The CredentialAssertion created is relayed back to the client, who in turn
// performs a user presence check and signs the challenge contained within the
// assertion.
// As a side effect Begin may assign (and record in storage) a WebAuthn ID for
// the user.
func (f *LoginFlow) Begin(ctx context.Context, user string) (*CredentialAssertion, error) {
	// Fetch existing user devices. We need the devices both to set the allowed
	// credentials for the user (webUser.credentials) and to determine if the U2F
	// appid extension is necessary.
	devices, err := f.Identity.GetMFADevices(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var opts []wan.LoginOption
	if f.U2F != nil && f.U2F.AppID != "" {
		// See https://www.w3.org/TR/webauthn-2/#sctn-appid-extension.
		opts = append(opts, wan.WithAssertionExtensions(protocol.AuthenticationExtensions{
			"appid": f.U2F.AppID,
		}))
	}

	// Fetch the user with secrets, their WebAuthn ID is inside.
	storedUser, err := f.Identity.GetUser(user, true /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := newWebUser(storedUser, true /* idOnly */, devices)

	// Create the WebAuthn object and create a new challenge.
	web, err := newWebAuthn(f.Webauthn, f.Webauthn.RPID, "" /* origin */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assertion, sessionData, err := web.BeginLogin(u, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Store SessionData - it's checked against the user response by
	// LoginFlow.Finish().
	sessionDataPB, err := sessionToPB(sessionData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := f.Identity.UpsertWebAuthnSessionData(user, loginSessionID, sessionDataPB); err != nil {
		return nil, trace.Wrap(err)
	}

	return assertion, nil
}
