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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentKeyGen)

	// PrecomputedKeys is a queue of cached keys ready for usage.
	PrecomputedKeys = make(chan *rsa.PrivateKey, 25)

	// PrecomputedKeys4096 is a queue of cached 4096-bit keys ready for usage.
	PrecomputedKeys4096 = make(chan *rsa.PrivateKey, 10)

	// StartPrecomputeOnce is used to start the background task that precomputes key pairs.
	StartPrecomputeOnce sync.Once
)

// GenerateKey returns a newly generated RSA private key.
func GenerateKey() (*rsa.PrivateKey, error) {
	return getOrGenerateRSAPrivateKey(constants.RSAKeySize)
}

// GenerateKey4096 generates a 4096-bit RSA private key meant for use in asymmetric encryption use cases such as
// encrypted session recordings.
func GenerateKey4096() (*rsa.PrivateKey, error) {
	return getOrGenerateRSAPrivateKey(4096)
}

func getOrGenerateRSAPrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	source := PrecomputedKeys
	if bitSize == 4096 {
		source = PrecomputedKeys4096
	}

	select {
	case k := <-source:
		return k, nil
	default:
		rsaKeyPair, err := generateRSAPrivateKey(bitSize)
		if err != nil {
			return nil, err
		}
		return rsaKeyPair, nil
	}
}

func generateRSAPrivateKey(bits int) (*rsa.PrivateKey, error) {
	//nolint:forbidigo // This is the one function allowed to generate RSA keys.
	return rsa.GenerateKey(rand.Reader, bits)
}

func precomputeKeys() {
	const backoff = time.Second * 30
	for {
		rsaPrivateKey, err := generateRSAPrivateKey(constants.RSAKeySize)
		if err != nil {
			log.ErrorContext(context.Background(), "Failed to precompute key pair, retrying (this might be a bug).",
				slog.Any("error", err), slog.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		PrecomputedKeys <- rsaPrivateKey
	}
}

// PrecomputeKeys sets this package into a mode where a small backlog of keys are
// computed in advance. This should only be enabled if large spikes in key computation
// are expected (e.g. in auth/proxy services). Safe to double-call.
func PrecomputeKeys() {
	StartPrecomputeOnce.Do(func() {
		go precomputeKeys()
	})
}
