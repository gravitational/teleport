/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"strings"

	"github.com/gravitational/trace"
)

// CalculateSPKI the hash value of the SPKI header in a certificate.
func CalculateSPKI(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CheckSPKI the passed in pin against the calculated value from a certificate.
func CheckSPKI(pin string, cert *x509.Certificate) error {
	// Check that the format of the pin is valid.
	parts := strings.Split(pin, ":")
	if len(parts) != 2 {
		return trace.BadParameter("invalid format for certificate pin, expected algorithm:pin")
	}
	if parts[0] != "sha256" {
		return trace.BadParameter("sha256 only supported hashing algorithm for certificate pin")
	}

	// Check that that pin itself matches that value calculated from the passed
	// in certificate.
	if subtle.ConstantTimeCompare([]byte(CalculateSPKI(cert)), []byte(pin)) != 1 {
		return trace.BadParameter(errorMessage)
	}

	return nil
}

var errorMessage string = "provided certificate pin does not match cluster pin. " +
	"This could have occurred if the Certificate Authority (CA) for the cluster " +
	"was rotated, invalidating the old pin. Run \"tctl status\" to compare the pin " +
	"used to join the cluster to the actual pin for the cluster."
