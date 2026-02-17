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

// BeginSSOMFAChallenge creates a new SSO MFA auth request and session data for the given user and sso device.
func (a *Server) BeginSSOMFAChallenge(ctx context.Context, params mfatypes.BeginSSOMFAChallengeParams) (*proto.SSOChallenge, error) {
	chal := &proto.SSOChallenge{
		Device: params.SSO,
	}

	switch params.SSO.ConnectorType {
	case constants.SAML:
		resp, err := a.CreateSAMLAuthRequestForMFA(ctx, types.SAMLAuthRequest{
			ConnectorID:       params.SSO.ConnectorId,
			Type:              params.SSO.ConnectorType,
			ClientRedirectURL: params.SSOClientRedirectURL,
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
			ConnectorID:       params.SSO.ConnectorId,
			Type:              params.SSO.ConnectorType,
			ClientRedirectURL: params.SSOClientRedirectURL,
			ProxyAddress:      params.ProxyAddress,
			PkceVerifier:      codeVerifier,
			CheckUser:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		chal.RequestId = resp.StateToken
		chal.RedirectUrl = resp.RedirectURL
	default:
		return nil, trace.BadParameter("unsupported sso connector type %v", params.SSO.ConnectorType)
	}

	if err := a.upsertMFASession(ctx, upsertMFASessionParams{
		user:          params.User,
		sessionID:     chal.RequestId,
		connectorID:   params.SSO.ConnectorId,
		connectorType: params.SSO.ConnectorType,
		ext:           params.Ext,
		sip:           params.SIP,
		sourceCluster: params.SourceCluster,
		targetCluster: params.TargetCluster,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return chal, nil
}

// VerifySSOMFASession verifies that the given sso mfa token matches an existing MFA session
// for the user and session ID. It also checks the required extensions, and finishes by deleting
// the MFA session if reuse is not allowed.
// TODO(cthach): Refactor to accept a params struct since there are many parameters. Must be done after SSO MFA device
// support is added to lib/auth/authtest (https://github.com/gravitational/teleport/issues/62271).
func (a *Server) VerifySSOMFASession(ctx context.Context, username, sessionID, token string, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
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
		Device:        groupedDevs.SSO,
		User:          username,
		AllowReuse:    mfaSess.ChallengeExtensions.AllowReuse,
		Payload:       mfaSess.Payload,
		SourceCluster: mfaSess.SourceCluster,
		TargetCluster: mfaSess.TargetCluster,
	}, nil
}

// upsertMFASessionParams are the parameters for upsertMFASession.
type upsertMFASessionParams struct {
	user           string
	sessionID      string
	connectorID    string
	connectorType  string
	tshRedirectURL string
	ext            *mfav1.ChallengeExtensions
	sip            *mfav1.SessionIdentifyingPayload
	sourceCluster  string
	targetCluster  string
}

// upsertMFASession upserts a new unverified MFA session for the given username,
// sessionID, connector details, and challenge extensions. This is used by both
// SSO MFA and Browser MFA.
func (a *Server) upsertMFASession(ctx context.Context, params upsertMFASessionParams) error {
	data := &services.MFASessionData{
		Username:       params.user,
		RequestID:      params.sessionID,
		ConnectorID:    params.connectorID,
		ConnectorType:  params.connectorType,
		TSHRedirectURL: params.tshRedirectURL,
		ChallengeExtensions: &mfatypes.ChallengeExtensions{
			Scope:      params.ext.Scope,
			AllowReuse: params.ext.AllowReuse,
		},
		SourceCluster: params.sourceCluster,
		TargetCluster: params.targetCluster,
	}

	if params.sip != nil {
		data.Payload = &mfatypes.SessionIdentifyingPayload{
			SSHSessionID: params.sip.GetSshSessionId(),
		}
	}

	return trace.Wrap(a.UpsertMFASessionData(ctx, data))
}

// UpsertMFASessionWithToken upserts the given SSO MFA session with a random mfa token.
func (a *Server) UpsertMFASessionWithToken(ctx context.Context, sd *services.MFASessionData) (token string, err error) {
	sd.Token, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err := a.UpsertMFASessionData(ctx, sd); err != nil {
		return "", trace.Wrap(err)
	}

	return sd.Token, nil
}

// GetMFASession returns the MFA session for the given username and sessionID.
func (a *Server) GetMFASession(ctx context.Context, sessionID string) (*services.MFASessionData, error) {
	sd, err := a.GetSSOMFASessionData(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sd, nil
}

// TODO(danielashare): Remove these wrapper functions once `e` points to the renamed versions
func (a *Server) UpsertSSOMFASessionWithToken(ctx context.Context, sd *services.MFASessionData) (token string, err error) {
	return a.UpsertMFASessionWithToken(ctx, sd)
}

func (a *Server) GetSSOMFASession(ctx context.Context, sessionID string) (*services.MFASessionData, error) {
	return a.GetMFASession(ctx, sessionID)
}
