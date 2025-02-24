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
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			signer, err := cryptosuites.GenerateKeyWithAlgorithm(algo)
			require.NoError(t, err)
			sig, err := challenge.Sign(chal, signer)
			if err != nil {
				t.Fatalf("Sign failed: %v", err)
			}

			// Verify correct challenge signature.
			if err := challenge.Verify(chal, sig, signer.Public()); err != nil {
				t.Errorf("Verify returned err=%v, want nil", err)
			}

			// Verify bad challenge signature.
			sig = []byte("invalid sig")
			if err := challenge.Verify(chal, sig, signer.Public()); err == nil {
				t.Error("Verify returned nil err, want non-nil")
			}
		})
	}
}
