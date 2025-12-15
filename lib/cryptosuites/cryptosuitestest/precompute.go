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
		go precomputeTestKeys(ctx, constants.RSAKeySize)
		go precomputeTestKeys(ctx, 4096)
	})
}

func precomputeTestKeys(ctx context.Context, bitSize int) {
	source := internalrsa.PrecomputedKeys
	count := testKeysNumber
	if bitSize == 4096 {
		source = internalrsa.PrecomputedKeys4096
		count = testKeysNumber4096
	}
	generatedTestKeys := generateTestKeys(ctx, bitSize)
	keysToReuse := make([]*rsa.PrivateKey, 0, count)
	for range count {
		select {
		case k := <-generatedTestKeys:
			source <- k
			keysToReuse = append(keysToReuse, k)
		case <-ctx.Done():
			return
		}
	}

	for {
		for _, k := range keysToReuse {
			select {
			case source <- k:
			case <-ctx.Done():
				return
			}
		}
	}
}

// testKeysNumber is the number of RSA keys generated in tests.
const testKeysNumber = 25

// testKeysNumber is the number of RSA 4096 keys generated in tests.
const testKeysNumber4096 = 10

func generateTestKeys(ctx context.Context, bitSize int) <-chan *rsa.PrivateKey {
	count := testKeysNumber
	if bitSize == 4096 {
		count = testKeysNumber4096
	}
	generatedTestKeys := make(chan *rsa.PrivateKey, count)
	for range count {
		// Generate each key in a separate goroutine to take advantage of
		// multiple cores if possible.
		go func() {
			private, err := generateRSAPrivateKey(bitSize)
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

func generateRSAPrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	//nolint:forbidigo // This is the one function allowed to generate RSA keys.
	return rsa.GenerateKey(rand.Reader, bitSize)
}
