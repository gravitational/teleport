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

// RandomizedDuration returns duration which is within given deviation from a given
// median.
//
// For example RandomizedDuration(time.Second * 10, 0.5) will return
// a random duration between 6 and 15 seconds.
func RandomizedDuration(median time.Duration, deviation float64) time.Duration {
	min := int64(float64(median) * (1 - deviation))
	max := int64(float64(median) * (1 + deviation))

	ceiling := big.NewInt(max - min)
	randomDeviation, err := rand.Int(rand.Reader, ceiling)
	if err != nil {
		return median
	}
	return time.Duration(min + randomDeviation.Int64())
}
