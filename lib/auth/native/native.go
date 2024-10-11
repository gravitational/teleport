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

package native

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/keys"
)

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentKeyGen,
})

// precomputedKeys is a queue of cached keys ready for usage.
var precomputedKeys = make(chan *rsa.PrivateKey, 25)

// startPrecomputeOnce is used to start the background task that precomputes key pairs.
var startPrecomputeOnce sync.Once

// GenerateKeyPair generates a new RSA key pair.
func GenerateKeyPair() ([]byte, []byte, error) {
	priv, err := GeneratePrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return priv.PrivateKeyPEM(), priv.MarshalSSHPublicKey(), nil
}

// GenerateEICEKey generates a key that can be send to an Amazon EC2 instance using the ec2instanceconnect.SendSSHPublicKey method.
func GenerateEICEKey() (publicKey any, privateKey any, err error) {
	if IsBoringBinary() {
		privKey, err := GeneratePrivateKey()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return privKey.Public(), privKey, nil
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return pubKey, privKey, nil
}

// GeneratePrivateKey generates a new RSA private key.
func GeneratePrivateKey() (*keys.PrivateKey, error) {
	rsaKey, err := getOrGenerateRSAPrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We encode the private key in PKCS #1, ASN.1 DER form
	// instead of PKCS #8 to maintain compatibility with some
	// third party clients.
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:    keys.PKCS1PrivateKeyType,
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	return keys.NewPrivateKey(rsaKey, keyPEM)
}

func GenerateRSAPrivateKey() (*rsa.PrivateKey, error) {
	return getOrGenerateRSAPrivateKey()
}

func getOrGenerateRSAPrivateKey() (*rsa.PrivateKey, error) {
	select {
	case k := <-precomputedKeys:
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
	return rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
}

func precomputeKeys() {
	const backoff = time.Second * 30
	for {
		rsaPrivateKey, err := generateRSAPrivateKey()
		if err != nil {
			log.WithError(err).Errorf("Failed to precompute key pair, retrying in %s (this might be a bug).", backoff)
			time.Sleep(backoff)
		}

		precomputedKeys <- rsaPrivateKey
	}
}

func precomputeTestKeys() {
	generatedTestKeys := generateTestKeys()
	keysToReuse := make([]*rsa.PrivateKey, 0, testKeysNumber)
	for range testKeysNumber {
		k := <-generatedTestKeys
		precomputedKeys <- k
		keysToReuse = append(keysToReuse, k)
	}
	for {
		for _, k := range keysToReuse {
			precomputedKeys <- k
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
	startPrecomputeOnce.Do(func() {
		go precomputeKeys()
	})
}

// PrecomputeTestKeys generates RSA keys and reuse them to reduce CPU usage. This method should
// only be in tests. Safe to call multiple times.
// This function takes *testing.M, so is only can be used from TestMain in tests.
func PrecomputeTestKeys(_ *testing.M) {
	startPrecomputeOnce.Do(func() {
		go precomputeTestKeys()
	})
}
