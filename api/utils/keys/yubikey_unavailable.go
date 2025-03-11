//go:build !piv && !pivtest

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
	"io"

	"github.com/gravitational/trace"
)

func NewYubiKeyPIVService(ctx context.Context, _ HardwareKeyPrompt) HardwareKeyService {
	return &unavailableYubiKeyPIVService{}
}

type unavailableYubiKeyPIVService struct{}

func (s *unavailableYubiKeyPIVService) NewPrivateKey(ctx context.Context, customSlot PIVSlot, requiredPolicy PrivateKeyPolicy) (*HardwarePrivateKeyRef, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *unavailableYubiKeyPIVService) Sign(ctx context.Context, ref HardwarePrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

func (s *unavailableYubiKeyPIVService) SetPrompt(prompt HardwareKeyPrompt) {}

func (_ PIVSlot) validate() error {
	return trace.Wrap(errPIVUnavailable)
}
