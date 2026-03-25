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
	"crypto/rand"
	"encoding/hex"
	"iter"
	"math/big"
	mathrand "math/rand/v2"
	"time"

	"github.com/gravitational/trace"
)

// CryptoRandomHex returns a hex-encoded random string generated
// with a crypto-strong pseudo-random generator. The length parameter
// controls how many random bytes are generated, and the returned
// hex string will be twice the length. An error is returned when
// fewer bytes were generated than length.
func CryptoRandomHex(length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
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

// ShuffleVisit yields the items of a slice in random order, while arranging
// them in the same order at the head of the slice. Exiting early from the
// iterator will result in a slice that's partially shuffled - specifically,
// pulling N items from the iterator will also arrange for the slice to contain
// the same items in the same order at indices 0 through N-1. The slice is
// updated as items are yielded from the iterator, so the first N items are
// fixed in position and inspectable during the iteration at step N.
func ShuffleVisit[S ~[]E, E any](s S) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		for i := range len(s) {
			j := mathrand.N(len(s))
			// swapping here (instead of swapping after the yield) ensures that
			// pulling items from the iterator also puts them in order at the
			// beginning of the slice, otherwise there would be a difference
			// between exhausting the iterator and exiting early; the items are
			// also accessible during the iteration
			s[0], s[j] = s[j], s[0]
			if !yield(i, s[0]) {
				return
			}
			s = s[1:]
		}
	}
}
