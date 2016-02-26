package utils

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gravitational/trace"
)

// CryptoRandomHex returns hex encoded random string generated with crypto-strong
// pseudo random generator of the given bytes
func CryptoRandomHex(len int) (string, error) {
	randomBytes := make([]byte, len)
	if _, err := rand.Reader.Read(randomBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(randomBytes), nil
}
