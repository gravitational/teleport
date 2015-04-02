package secret

import (
	"encoding/base64"
	"fmt"
)

// NewKey returns a new key that can be used to encrypt and decrypt messages.
func NewKey() (*[SecretKeyLength]byte, error) {
	// get 32-bytes of random from /dev/urandom
	bytes, err := randomProvider.Bytes(SecretKeyLength)
	if err != nil {
		return nil, fmt.Errorf("unable to generate random: %v", err)
	}

	return keySliceToArray(bytes)
}

// EncodedStringToKey converts a base64-encoded string into key bytes.
func EncodedStringToKey(encodedKey string) (*[SecretKeyLength]byte, error) {
	// decode base64-encoded key
	keySlice, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	// convert to array and return
	return keySliceToArray(keySlice)
}

// KeyToEncodedString converts bytes into a base64-encoded string
func KeyToEncodedString(keybytes *[SecretKeyLength]byte) string {
	return base64.StdEncoding.EncodeToString(keybytes[:])
}
