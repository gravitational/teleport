/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
func CheckSPKI(pins []string, certs []*x509.Certificate) error {
	// check pins
	for _, pin := range pins {
		// Check that the format of the pin is valid.
		parts := strings.Split(pin, ":")
		if len(parts) != 2 {
			return trace.BadParameter("invalid format for certificate pin, expected algorithm:pin")
		}
		if parts[0] != "sha256" {
			return trace.BadParameter("sha256 only supported hashing algorithm for certificate pin")
		}
	}
	// Timing of this check depends only on the number of pins and certs, not
	// their contents.
outer:
	for _, cert := range certs {
		for _, pin := range pins {
			// Check that that pin itself matches that value calculated from the passed
			// in certificate.
			if subtle.ConstantTimeCompare([]byte(CalculateSPKI(cert)), []byte(pin)) == 1 {
				continue outer
			}
		}
		return trace.BadParameter("%s", errorMessage)
	}

	return nil
}

var errorMessage = "cluster pin does not match any provided certificate authority pin. " +
	"This could have occurred if the Certificate Authority (CA) for the cluster " +
	"was rotated, invalidating the old pin. This could also occur if a new HSM was " +
	"added. Run \"tctl status\" to compare the pin used to join the cluster to the " +
	"actual pin(s) for the cluster."
