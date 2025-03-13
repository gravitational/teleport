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

package piv

import (
	"context"
	"crypto"
	"errors"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

var errPIVUnavailable = errors.New("PIV is unavailable in current build")

func NewYubiKeyPIVService(ctx context.Context, _ hardwarekey.PrivateKeyRef) *unavailableYubiKeyPIVService {
	return &unavailableYubiKeyPIVService{}
}

type unavailableYubiKeyPIVService struct{}

func (s *unavailableYubiKeyPIVService) NewPrivateKey(_ context.Context, _ hardwarekey.PIVSlot, _ hardwarekey.PromptPolicy) (*hardwarekey.PrivateKeyRef, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *unavailableYubiKeyPIVService) Sign(_ context.Context, _ hardwarekey.PrivateKeyRef, _ io.Reader, _ []byte, _ crypto.SignerOpts) ([]byte, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

func (s *unavailableYubiKeyPIVService) SetPrompt(_ hardwarekey.Prompt) {}
