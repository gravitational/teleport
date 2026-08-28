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

package darwin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
)

// ECDSAPublicKeyFromRaw reads an ECDSA public key from a raw Apple public key,
// as returned by SecKeyCopyExternalRepresentation.
func ECDSAPublicKeyFromRaw(pubKeyRaw []byte) (*ecdsa.PublicKey, error) {
	// Verify key length to avoid a potential panic below.
	// 3 is the smallest number that clears it, but in practice 65 is the more
	// common length.
	// Apple's docs make no guarantees, hence no assumptions are made here.
	switch l := len(pubKeyRaw); {
	case l < 3:
		return nil, fmt.Errorf("public key representation too small (%v bytes)", l)
	case l%2 != 1: // 0x4+keyLen+keyLen is always odd, see explanation below.
		return nil, fmt.Errorf("public key representation has unexpected length (%v bytes)", l)
	case pubKeyRaw[0] != 0x04: // See explanation below.
		return nil, fmt.Errorf("public key representation starts with unexpected byte (%#x vs 0x4)", pubKeyRaw[0])
	}

	// "For an elliptic curve public key, the format follows the ANSI X9.63
	// standard using a byte string of 04 || X || Y. (...) All of these
	// representations use constant size integers, including leading zeros as
	// needed."
	// https://developer.apple.com/documentation/security/1643698-seckeycopyexternalrepresentation?language=objc
	pubKeyRaw = pubKeyRaw[1:] // skip 0x4
	l := len(pubKeyRaw) / 2
	x := pubKeyRaw[:l]
	y := pubKeyRaw[l:]

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     (&big.Int{}).SetBytes(x),
		Y:     (&big.Int{}).SetBytes(y),
	}, nil
}
