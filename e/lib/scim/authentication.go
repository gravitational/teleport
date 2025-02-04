package scim

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/types"
)

// getStaticCreds searches the supplied static credentials DB for all
// credentials related to the supplied plugin
func getStaticCreds(ctx context.Context, creds CredentialsService, p types.Plugin) ([]types.PluginStaticCredentials, error) {
	staticCredsRef := p.GetCredentials().GetStaticCredentialsRef()
	if staticCredsRef == nil {
		return nil, trace.NotFound("no static credentials ref found")
	}

	staticCreds, err := creds.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return staticCreds, nil
}

func checkBearerToken(tokenCredential types.PluginStaticCredentials, authHeader string) error {
	bearerToken, err := extractBearerToken(authHeader)
	if err != nil {
		return trace.Wrap(err)
	}

	expectedHash := []byte(tokenCredential.GetAPIToken())
	actualBits := []byte(bearerToken)
	if err := bcrypt.CompareHashAndPassword(expectedHash, actualBits); err != nil {
		return trace.AccessDenied("invalid token")
	}

	return nil
}

func extractBearerToken(authHeader string) (string, error) {
	const (
		hdrBearer = "bearer"
		errMsg    = "malformed bearer token"
	)

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		return "", trace.BadParameter("%s", errMsg)
	}

	if strings.ToLower(parts[0]) != hdrBearer {
		return "", trace.BadParameter("%s", errMsg)
	}

	return strings.TrimSpace(parts[1]), nil
}
