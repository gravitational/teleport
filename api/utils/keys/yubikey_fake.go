//go:build pivtest

/*
Copyright 2024 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"

	"github.com/gravitational/trace"
)

var errPIVUnavailable = errors.New("PIV is unavailable in current build")

// Return a fake YubiKey private key.
func getOrGenerateYubiKeyPrivateKey(_ context.Context, policy PrivateKeyPolicy, _ PIVSlot, _ HardwareKeyPrompt) (*PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPEM, err := MarshalPrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer := &fakeYubiKeyPrivateKey{
		Signer:           priv,
		privateKeyPolicy: policy,
	}

	return NewPrivateKey(signer, keyPEM)
}

func parseYubiKeyPrivateKeyData(_ []byte, _ HardwareKeyPrompt) (*PrivateKey, error) {
	// TODO(Joerger): add custom marshal/unmarshal logic for fakeYubiKeyPrivateKey (if necessary).
	return nil, trace.Wrap(errPIVUnavailable)
}

func (s PIVSlot) validate() error {
	return trace.Wrap(errPIVUnavailable)
}

type fakeYubiKeyPrivateKey struct {
	crypto.Signer
	privateKeyPolicy PrivateKeyPolicy
}

// GetAttestationStatement returns an AttestationStatement for this private key.
func (y *fakeYubiKeyPrivateKey) GetAttestationStatement() *AttestationStatement {
	// Since this is only used in tests, we will ignore the attestation statement in the end.
	// We just need it to be non-nil so that it goes through the test modules implementation
	// of AttestHardwareKey.
	return &AttestationStatement{}
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
func (y *fakeYubiKeyPrivateKey) GetPrivateKeyPolicy() PrivateKeyPolicy {
	return y.privateKeyPolicy
}

// IsHardware returns true if [k] is a hardware PIV key.
func (k *PrivateKey) IsHardware() bool {
	switch k.Signer.(type) {
	case *fakeYubiKeyPrivateKey:
		return true
	}
	return false
}
