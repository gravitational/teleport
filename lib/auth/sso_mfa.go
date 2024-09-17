package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// beginSSOMFAChallenge creates a new SSO MFA auth request and session data for the given user and sso device.
func (a *Server) beginSSOMFAChallenge(ctx context.Context, user string, sso *types.SSOMFADevice, ssoClientRedirectURL string, ext *mfav1.ChallengeExtensions) (*proto.SSOChallenge, error) {
	chal := new(proto.SSOChallenge)
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
		resp, err := a.CreateOIDCAuthRequestForMFA(ctx, types.OIDCAuthRequest{
			ConnectorID:       sso.ConnectorId,
			Type:              sso.ConnectorType,
			ClientRedirectURL: ssoClientRedirectURL,
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

	err := a.UpsertSSOMFASessionData(ctx, &services.SSOMFASessionData{
		Username:            user,
		RequestID:           chal.RequestId,
		ConnectorID:         sso.ConnectorId,
		ConnectorType:       sso.ConnectorType,
		ChallengeExtensions: ext,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return chal, nil
}

// CreateSSOMFASession creates a new unverified SSO MFA session for the given username,
// sessionID, connector details, and challenge extensions.
func (a *Server) CreateSSOMFASession(ctx context.Context, user string, sessionID string, connectorID string, connectorType string, ext *mfav1.ChallengeExtensions) error {
	err := a.UpsertSSOMFASessionData(ctx, &services.SSOMFASessionData{
		Username:            user,
		RequestID:           sessionID,
		ConnectorID:         connectorID,
		ConnectorType:       connectorType,
		ChallengeExtensions: ext,
	})
	return trace.Wrap(err)
}

// GetSSOMFASession returns the SSO MFA session for the given username and sessionID.
func (a *Server) GetSSOMFASession(ctx context.Context, sessionID string) (*services.SSOMFASessionData, error) {
	sd, err := a.GetSSOMFASessionData(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sd, nil
}

// UpdateSSOMFASessionWithToken updates the given SSO MFA session with a random mfa token.
func (a *Server) UpdateSSOMFASessionWithToken(ctx context.Context, sd *services.SSOMFASessionData) (token string, err error) {
	sd.Token, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err := a.UpsertSSOMFASessionData(ctx, sd); err != nil {
		return "", trace.Wrap(err)
	}

	return sd.Token, nil
}
