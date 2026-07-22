// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package challenge_test

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

func TestSignAndVerify(t *testing.T) {
	for _, algo := range []cryptosuites.Algorithm{
		cryptosuites.RSA2048,
		cryptosuites.ECDSAP256,
		cryptosuites.Ed25519,
	} {
		t.Run(algo.String(), func(t *testing.T) {
			t.Parallel()

			chal, err := challenge.New()
			require.NoError(t, err, "Creating new challenge")

			signer, err := cryptosuites.GenerateKeyWithAlgorithm(algo)
			require.NoError(t, err)
			sig, err := challenge.Sign(chal, signer)
			require.NoError(t, err, "Signing failed")

			t.Run("verify valid signature", func(t *testing.T) {
				require.NoError(t, challenge.Verify(chal, sig, signer.Public()))
			})

			t.Run("verifying an invalid signature is an error", func(t *testing.T) {
				sig = []byte("invalid sig")
				require.Error(t, challenge.Verify(chal, sig, signer.Public()))
			})

			t.Run("signing an empty challenge is an error", func(t *testing.T) {
				_, err = challenge.Sign([]byte{}, signer)
				require.ErrorAs(t, err, new(*trace.BadParameterError))
			})

			t.Run("verifying an empty challenge is an error", func(t *testing.T) {
				err = challenge.Verify([]byte{}, sig, signer.Public())
				require.ErrorAs(t, err, new(*trace.BadParameterError))
			})

			t.Run("verifying an empty signature is an error", func(t *testing.T) {
				err = challenge.Verify(chal, []byte{}, signer.Public())
				require.ErrorAs(t, err, new(*trace.BadParameterError))
			})
		})
	}
}
