package utils

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"time"

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

// RandomDuration returns a duration in a range [0, max)
func RandomDuration(max time.Duration) time.Duration {
	randomVal, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return max / 2
	}
	return time.Duration(randomVal.Int64())
}
