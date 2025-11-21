// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cryptosuitestest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gravitational/teleport/api/constants"
	internalrsa "github.com/gravitational/teleport/lib/cryptosuites/internal/rsa"
)

// PrecomputeRSAKeys may be called from TestMain to set this package into a
// mode where it will precompute a fixed number of RSA keys and reuse them to
// save on CPU usage.
func PrecomputeRSAKeys(ctx context.Context) {
	internalrsa.StartPrecomputeOnce.Do(func() {
		go precomputeTestKeys(ctx)
	})
}

func precomputeTestKeys(ctx context.Context) {
	generatedTestKeys := generateTestKeys(ctx)
	keysToReuse := make([]*rsa.PrivateKey, 0, testKeysNumber)
	for range testKeysNumber {
		select {
		case k := <-generatedTestKeys:
			internalrsa.PrecomputedKeys <- k
			keysToReuse = append(keysToReuse, k)
		case <-ctx.Done():
			return
		}
	}

	for {
		for _, k := range keysToReuse {
			select {
			case internalrsa.PrecomputedKeys <- k:
			case <-ctx.Done():
				return
			}
		}
	}
}

// testKeysNumber is the number of RSA keys generated in tests.
const testKeysNumber = 25

func generateTestKeys(ctx context.Context) <-chan *rsa.PrivateKey {
	generatedTestKeys := make(chan *rsa.PrivateKey, testKeysNumber)
	for range testKeysNumber {
		// Generate each key in a separate goroutine to take advantage of
		// multiple cores if possible.
		go func() {
			private, err := generateRSAPrivateKey()
			if err != nil {
				// Use only in tests. Safe to panic.
				panic(err)
			}

			select {
			case generatedTestKeys <- private:
			case <-ctx.Done():
				return
			}

		}()
	}
	return generatedTestKeys
}

func generateRSAPrivateKey() (*rsa.PrivateKey, error) {
	//nolint:forbidigo // This is the one function allowed to generate RSA keys.
	return rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
}
