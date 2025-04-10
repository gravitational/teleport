package scim

import (
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
)

func checkBearerToken(scimToken string, authHeader string) error {
	bearerToken, err := extractBearerToken(authHeader)
	if err != nil {
		return trace.Wrap(err)
	}

	expectedHash := []byte(scimToken)
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
