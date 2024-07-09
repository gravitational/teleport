/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package token

import (
	"math"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// Verify ensures the token fits into our length requirements and
// its bits of entropy are sufficient.
func Verify(token []byte) error {
	return verify(token, defaults.MaxTokenLength)
}

// VerifyHashed ensures the token fits into our length requirements and
// its bits of entropy are sufficient. If the token is not going to be
// hashed by bcrypt before it will be used, use [Verify] instead.
func VerifyHashed(token []byte) error {
	return verify(token, defaults.MaxHashedTokenLength)
}

func verify(token []byte, maxLen int) error {
	if len(token) < defaults.MinTokenLength {
		return trace.BadParameter("token is too short, min length is %d", defaults.MinTokenLength)
	}
	if len(token) > maxLen {
		return trace.BadParameter("token is too long, max length is %d", maxLen)
	}

	entropyBits := TokenStrength(token)
	if entropyBits < defaults.MinTokenStrength {
		return trace.BadParameter("token is not strong enough; try with a longer and/or more random token")
	}

	return nil
}

// TokenStrength returns an approximate value of the token's strength.
// The strength is derived from the shannon entropy of the token
// multiplied by the token's length.
func TokenStrength(input []byte) float64 {
	freq := make(map[byte]int)
	for _, b := range input {
		freq[b]++
	}

	// compute shannon entropy
	// https://mathworld.wolfram.com/Entropy.html
	var shannon float64
	for _, count := range freq {
		pval := float64(count) / float64(len(input))
		pinv := float64(len(input)) / float64(count)
		shannon += pval * math.Log2(pinv)
	}

	// multiply shannon entropy with length to reward longer tokens
	return shannon * float64(len(input))
}
