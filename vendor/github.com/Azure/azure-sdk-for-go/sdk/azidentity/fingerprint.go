// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"bytes"
	"crypto/sha1"
	"encoding/pem"
	"fmt"
)

// fingerprint type wraps a byte slice that contains the corresponding SHA-1 fingerprint for the client's certificate
type fingerprint []byte

// String represents the fingerprint digest as a series of
// colon-delimited hexadecimal octets.
func (f fingerprint) String() string {
	var buf bytes.Buffer
	for i, b := range f {
		if i > 0 {
			fmt.Fprintf(&buf, ":")
		}
		fmt.Fprintf(&buf, "%02x", b)
	}
	return buf.String()
}

// newFingerprint calculates the fingerprint of the certificate based on it's Subject Public Key Info with the SHA-1
// signing algorithm.
func newFingerprint(block *pem.Block) (fingerprint, error) {
	h := sha1.New()
	_, err := h.Write(block.Bytes)
	if err != nil {
		return nil, err
	}
	return fingerprint(h.Sum(nil)), nil
}
