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

package hardwarekeyagent

import (
	"context"
	"crypto"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// Service is an agent-based implementation of [hardwarekey.Service].
type Service struct {
	// Used for signature requests to the service.
	agentClient *Client
	// Used for non signature methods and as a fallback for signatures if the
	// agent client fails to handle a sign request.
	fallbackService hardwarekey.Service
}

// NewService creates a new hardware key service from the given
// agent client and fallback service. The fallback service is used for
// non-signature methods of [hardwarekey.Service] which are not implemented
// by the agent. Generally this fallback service is only used during login.
func NewService(agentClient *Client, fallbackService hardwarekey.Service) *Service {
	return &Service{
		agentClient:     agentClient,
		fallbackService: fallbackService,
	}
}

// NewPrivateKey creates or retrieves a hardware private key for the given config.
func (s *Service) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.Signer, error) {
	return s.fallbackService.NewPrivateKey(ctx, config)
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *Service) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// First try to sign with the agent, then fallback to the direct service if needed.
	signature, err := s.agentClient.Sign(ctx, ref, keyInfo, rand, digest, opts)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to perform signature over hardware key agent, falling back to direct hardware key service", "agent_err", err)
		signature, err = s.fallbackService.Sign(ctx, ref, keyInfo, rand, digest, opts)
	}

	return signature, trace.Wrap(err)
}

// TODO(Joerger): DELETE IN v19.0.0
func (s *Service) GetFullKeyRef(serialNumber uint32, slotKey hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	return s.fallbackService.GetFullKeyRef(serialNumber, slotKey)
}
