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
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

func TestSignAndVerify(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048 /* bits */)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name   string
		signer crypto.Signer
		pubKey crypto.PublicKey
	}{
		{
			name:   "ecdsa key",
			signer: ecKey,
			pubKey: ecKey.Public(),
		},
		{
			name:   "rsa key",
			signer: rsaKey,
			pubKey: rsaKey.Public(),
		},
		{
			name:   "ed25519 key",
			signer: edPriv,
			pubKey: edPub,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chal, err := challenge.New()
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			sig, err := challenge.Sign(chal, test.signer)
			if err != nil {
				t.Fatalf("Sign failed: %v", err)
			}

			// Verify correct challenge signature.
			if err := challenge.Verify(chal, sig, test.pubKey); err != nil {
				t.Errorf("Verify returned err=%v, want nil", err)
			}

			// Verify bad challenge signature.
			sig = []byte("invalid sig")
			if err := challenge.Verify(chal, sig, test.pubKey); err == nil {
				t.Error("Verify returned nil err, want non-nil")
			}
		})
	}
}
