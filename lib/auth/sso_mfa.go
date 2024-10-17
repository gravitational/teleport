package auth

import (
	"context"

	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

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
