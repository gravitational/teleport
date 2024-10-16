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

package oracle

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestCreateJksWallet(t *testing.T) {
	algos := []cryptosuites.Algorithm{
		cryptosuites.RSA2048,
		cryptosuites.ECDSAP256,
		cryptosuites.Ed25519,
	}

	for _, algo := range algos {
		t.Run(algo.String(), func(t *testing.T) {
			signer, err := cryptosuites.GenerateKeyWithAlgorithm(algo)
			require.NoError(t, err)

			publicPEM, err := keys.MarshalPublicKey(signer.Public())
			require.NoError(t, err)

			wrapped, err := keys.NewSoftwarePrivateKey(signer)
			require.NoError(t, err)

			_, err = createJKSWallet(signer, publicPEM, publicPEM, "dummy")
			require.NoError(t, err)

			_, err = createJKSWallet(wrapped, publicPEM, publicPEM, "dummy")
			require.NoError(t, err)
		})
	}
}
