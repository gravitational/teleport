// Teleport
// Copyright (C) 2023 Gravitational, Inc.
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

package rsa

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentKeyGen)

	// PrecomputedKeys is a queue of cached keys ready for usage.
	PrecomputedKeys = make(chan *rsa.PrivateKey, 25)

	// StartPrecomputeOnce is used to start the background task that precomputes key pairs.
	StartPrecomputeOnce sync.Once
)

// GenerateKey returns a newly generated RSA private key.
func GenerateKey() (*rsa.PrivateKey, error) {
	return getOrGenerateRSAPrivateKey()
}

func getOrGenerateRSAPrivateKey() (*rsa.PrivateKey, error) {
	select {
	case k := <-PrecomputedKeys:
		return k, nil
	default:
		rsaKeyPair, err := generateRSAPrivateKey()
		if err != nil {
			return nil, err
		}
		return rsaKeyPair, nil
	}
}

func generateRSAPrivateKey() (*rsa.PrivateKey, error) {
	//nolint:forbidigo // This is the one function allowed to generate RSA keys.
	return rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
}

func precomputeKeys() {
	const backoff = time.Second * 30
	for {
		rsaPrivateKey, err := generateRSAPrivateKey()
		if err != nil {
			log.ErrorContext(context.Background(), "Failed to precompute key pair, retrying (this might be a bug).",
				slog.Any("error", err), slog.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		PrecomputedKeys <- rsaPrivateKey
	}
}

func precomputeTestKeys() {
	generatedTestKeys := generateTestKeys()
	keysToReuse := make([]*rsa.PrivateKey, 0, testKeysNumber)
	for range testKeysNumber {
		k := <-generatedTestKeys
		PrecomputedKeys <- k
		keysToReuse = append(keysToReuse, k)
	}
	for {
		for _, k := range keysToReuse {
			PrecomputedKeys <- k
		}
	}
}

// testKeysNumber is the number of RSA keys generated in tests.
const testKeysNumber = 25

func generateTestKeys() <-chan *rsa.PrivateKey {
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
			generatedTestKeys <- private
		}()
	}
	return generatedTestKeys
}

// PrecomputeKeys sets this package into a mode where a small backlog of keys are
// computed in advance. This should only be enabled if large spikes in key computation
// are expected (e.g. in auth/proxy services). Safe to double-call.
func PrecomputeKeys() {
	StartPrecomputeOnce.Do(func() {
		go precomputeKeys()
	})
}

// PrecomputeTestKeys generates a fixed number of RSA keys and reuses them to
// reduce CPU usage. This method should only be in tests. Safe to call multiple
// times. This function takes *testing.M so it can only be used from TestMain in
// tests.
// Deprecated: prefer using cyptosuitestest.PrecomputeRSAKeys instead
// TODO(tross): Delete once references in teleport.e are gone.
func PrecomputeTestKeys(_ *testing.M) {
	StartPrecomputeOnce.Do(func() {
		go precomputeTestKeys()
	})
}
