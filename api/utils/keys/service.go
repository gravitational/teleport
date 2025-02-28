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

package keys

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gravitational/trace"

	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
)

// Service implementation [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type Service struct {
	hardwarekeyagentv1.UnimplementedHardwareKeyAgentServiceServer

	c ServiceConfig
}

// ServiceConfig is configuration for a hardware key agent Service.
type ServiceConfig struct {
	// HardwareKeyPrompt is a hardware key prompt to use during signature requests, when necessary.
	HardwareKeyPrompt HardwareKeyPrompt
}

func NewService(config ServiceConfig) *Service {
	return &Service{c: config}
}

// Sign the given digest with the specified hardware private key.
func (s *Service) Sign(_ context.Context, req *hardwarekeyagentv1.SignRequest) (*hardwarekeyagentv1.Signature, error) {
	slotKey, err := pivSlotProtoToUint(req.KeyRef.PivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := GetYubiKeyPrivateKey(&YubiKeyPrivateKeyRef{
		SerialNumber: req.KeyRef.SerialNumber,
		SlotKey:      slotKey,
	}, s.c.HardwareKeyPrompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var signerOpts crypto.SignerOpts
	switch req.HashName {
	case hardwarekeyagentv1.HashName_HASH_NAME_SHA256:
		signerOpts = crypto.SHA256
	case hardwarekeyagentv1.HashName_HASH_NAME_SHA512:
		signerOpts = crypto.SHA512
	default:
		return nil, trace.BadParameter("unsupported hash func %q", req.HashName.String())
	}

	if req.GetSaltLength() != nil {
		signerOpts = &rsa.PSSOptions{
			Hash:       signerOpts.HashFunc(),
			SaltLength: int(req.GetSaltLength().(*hardwarekeyagentv1.SignRequest_Length).Length),
		}
	}

	signature, err := key.Sign(rand.Reader, req.Digest, signerOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &hardwarekeyagentv1.Signature{
		Signature: signature,
	}, nil
}

// GetAttestation gets the attestation statement for the specified hardware private key.
func (s *Service) GetAttestation(ctx context.Context, req *hardwarekeyagentv1.GetAttestationRequest) (*hardwarekeyagentv1.GetAttestationResponse, error) {
	slotKey, err := pivSlotProtoToUint(req.KeyRef.PivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := GetYubiKeyPrivateKey(&YubiKeyPrivateKeyRef{
		SerialNumber: req.KeyRef.SerialNumber,
		SlotKey:      slotKey,
	}, s.c.HardwareKeyPrompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &hardwarekeyagentv1.GetAttestationResponse{
		AttestationStatement: key.GetAttestationStatement().ToProto(),
	}, nil
}

func pivSlotProtoToUint(pivSlot hardwarekeyagentv1.PIVSlot) (uint32, error) {
	switch pivSlot {
	case hardwarekeyagentv1.PIVSlot_PIV_SLOT_9A:
		return 0x9a, nil
	case hardwarekeyagentv1.PIVSlot_PIV_SLOT_9C:
		return 0x9c, nil
	case hardwarekeyagentv1.PIVSlot_PIV_SLOT_9D:
		return 0x9d, nil
	case hardwarekeyagentv1.PIVSlot_PIV_SLOT_9E:
		return 0x9e, nil
	default:
		return 0, trace.BadParameter("unknown piv slot for proto enum %d", pivSlot)
	}
}

func PIVSlotUintToProto(pivSlot uint32) (hardwarekeyagentv1.PIVSlot, error) {
	switch pivSlot {
	case 0x9a:
		return hardwarekeyagentv1.PIVSlot_PIV_SLOT_9A, nil
	case 0x9c:
		return hardwarekeyagentv1.PIVSlot_PIV_SLOT_9C, nil
	case 0x9d:
		return hardwarekeyagentv1.PIVSlot_PIV_SLOT_9D, nil
	case 0x9e:
		return hardwarekeyagentv1.PIVSlot_PIV_SLOT_9E, nil
	default:
		return 0, trace.BadParameter("unknown piv slot for proto enum %d", pivSlot)
	}
}
