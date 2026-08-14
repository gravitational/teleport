package storage

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
)

type deviceToken struct {
	PlainToken     []byte
	HashedToken    []byte
	SafePlainToken string
}

// createDeviceToken creates a token suitable for use as a DeviceWebToken or
// DeviceConfirmationToken.
func createDeviceToken() (*deviceToken, error) {
	const deviceTokenLen = 32

	plainToken := make([]byte, deviceTokenLen)
	if _, err := rand.Read(plainToken); err != nil {
		return nil, trace.Wrap(err, "generating device token")
	}

	hashedToken, err := bcrypt.GenerateFromPassword(plainToken, bcrypt.DefaultCost)
	if err != nil {
		return nil, trace.Wrap(err, "hashing device token as a password")
	}

	return &deviceToken{
		PlainToken:     plainToken,
		HashedToken:    hashedToken,
		SafePlainToken: base64.RawURLEncoding.EncodeToString(plainToken),
	}, nil
}

func matchDeviceToken(safePlainToken string, hashedToken []byte) error {
	plainToken, err := base64.RawURLEncoding.DecodeString(safePlainToken)
	if err != nil {
		// Re-wrap as a BadParameter.
		return trace.BadParameter("%s", err)
	}

	if err := bcrypt.CompareHashAndPassword(hashedToken, plainToken); err != nil {
		// err swallowed on purpose.
		return trace.BadParameter("invalid device token")
	}

	return nil
}
