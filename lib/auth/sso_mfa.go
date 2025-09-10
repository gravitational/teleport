// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth

import (
	"context"
	"crypto/subtle"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// beginSSOMFAChallenge creates a new SSO MFA auth request and session data for the given user and sso device.
func (a *Server) beginSSOMFAChallenge(ctx context.Context, user string, sso *types.SSOMFADevice, ssoClientRedirectURL, proxyAddress string, ext *mfav1.ChallengeExtensions) (*proto.SSOChallenge, error) {
	chal := &proto.SSOChallenge{
		Device: sso,
	}

	switch sso.ConnectorType {
	case constants.SAML:
		resp, err := a.CreateSAMLAuthRequestForMFA(ctx, types.SAMLAuthRequest{
			ConnectorID:       sso.ConnectorId,
			Type:              sso.ConnectorType,
			ClientRedirectURL: ssoClientRedirectURL,
			CheckUser:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		chal.RequestId = resp.ID
		chal.RedirectUrl = resp.RedirectURL
	case constants.OIDC:
		codeVerifier := oauth2.GenerateVerifier()

		resp, err := a.CreateOIDCAuthRequestForMFA(ctx, types.OIDCAuthRequest{
			ConnectorID:       sso.ConnectorId,
			Type:              sso.ConnectorType,
			ClientRedirectURL: ssoClientRedirectURL,
			ProxyAddress:      proxyAddress,
			PkceVerifier:      codeVerifier,
			CheckUser:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		chal.RequestId = resp.StateToken
		chal.RedirectUrl = resp.RedirectURL
	default:
		return nil, trace.BadParameter("unsupported sso connector type %v", sso.ConnectorType)
	}

	if err := a.upsertSSOMFASession(ctx, user, chal.RequestId, sso.ConnectorId, sso.ConnectorType, ext); err != nil {
		return nil, trace.Wrap(err)
	}

	return chal, nil
}

// verifySSOMFASession verifies that the given sso mfa token matches an existing MFA session
// for the user and session ID. It also checks the required extensions, and finishes by deleting
// the MFA session if reuse is not allowed.
func (a *Server) verifySSOMFASession(ctx context.Context, username, sessionID, token string, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("requested challenge extensions must be supplied.")
	}

	const notFoundErrMsg = "mfa sso session data not found"
	mfaSess, err := a.GetSSOMFASessionData(ctx, sessionID)
	if trace.IsNotFound(err) {
		return nil, trace.AccessDenied("%s", notFoundErrMsg)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify the user's name and sso device matches.
	if mfaSess.Username != username {
		return nil, trace.AccessDenied("%s", notFoundErrMsg)
	}

	// Check if the MFA session matches the user's SSO MFA settings.
	devs, err := a.Services.GetMFADevices(ctx, username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	groupedDevs := groupByDeviceType(devs)
	if groupedDevs.SSO == nil {
		return nil, trace.AccessDenied("invalid sso mfa session data; non-sso user")
	} else if groupedDevs.SSO.GetSso().ConnectorId != mfaSess.ConnectorID || groupedDevs.SSO.GetSso().ConnectorType != mfaSess.ConnectorType {
		return nil, trace.AccessDenied("invalid sso mfa session data; mismatched sso auth connector")
	}

	// Verify the token matches.
	if subtle.ConstantTimeCompare([]byte(mfaSess.Token), []byte(token)) == 0 {
		return nil, trace.AccessDenied("invalid SSO MFA challenge response")
	}

	// Check if the given scope is satisfied by the challenge scope.
	if requiredExtensions.Scope != mfaSess.ChallengeExtensions.Scope {
		return nil, trace.AccessDenied("required scope %q is not satisfied by the given sso mfa session with scope %q", requiredExtensions.Scope, mfaSess.ChallengeExtensions.Scope)
	}

	// If this session is reusable, but this context forbids reusable sessions, return an error.
	if requiredExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO && mfaSess.ChallengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		return nil, trace.AccessDenied("the given sso mfa session allows reuse, but reuse is not permitted in this context")
	}

	if mfaSess.ChallengeExtensions.AllowReuse != mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
		if err := a.DeleteSSOMFASessionData(ctx, sessionID); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &authz.MFAAuthData{
		Device:     groupedDevs.SSO,
		User:       username,
		AllowReuse: mfaSess.ChallengeExtensions.AllowReuse,
	}, nil
}

// upsertSSOMFASession upserts a new unverified SSO MFA session for the given username,
// sessionID, connector details, and challenge extensions.
func (a *Server) upsertSSOMFASession(ctx context.Context, user string, sessionID string, connectorID string, connectorType string, ext *mfav1.ChallengeExtensions) error {
	err := a.UpsertSSOMFASessionData(ctx, &services.SSOMFASessionData{
		Username:      user,
		RequestID:     sessionID,
		ConnectorID:   connectorID,
		ConnectorType: connectorType,
		ChallengeExtensions: &mfatypes.ChallengeExtensions{
			Scope:      ext.Scope,
			AllowReuse: ext.AllowReuse,
		},
	})
	return trace.Wrap(err)
}

// UpsertSSOMFASessionWithToken upserts the given SSO MFA session with a random mfa token.
func (a *Server) UpsertSSOMFASessionWithToken(ctx context.Context, sd *services.SSOMFASessionData) (token string, err error) {
	sd.Token, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err := a.UpsertSSOMFASessionData(ctx, sd); err != nil {
		return "", trace.Wrap(err)
	}

	return sd.Token, nil
}

// GetSSOMFASession returns the SSO MFA session for the given username and sessionID.
func (a *Server) GetSSOMFASession(ctx context.Context, sessionID string) (*services.SSOMFASessionData, error) {
	sd, err := a.GetSSOMFASessionData(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sd, nil
}
