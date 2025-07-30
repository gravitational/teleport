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
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// Service is an agent implementation of [hardwarekey.AgentService].
type Service struct {
	// Used for signature requests to the service.
	agentClient hardwarekeyagentv1.HardwareKeyAgentServiceClient
	// Used for non signature methods and as a fallback for signatures if the
	// agent client fails to handle a sign request.
	fallbackService hardwarekey.Service
}

// NewService creates a new hardware key agent service from the given
// agent client and fallback service.
//
// The fallback service is used for methods unsupported by the agent service,
// such as [Service.NewPrivateKey], and as a fallback for failed agent signatures.
func NewService(agentClient hardwarekeyagentv1.HardwareKeyAgentServiceClient, fallbackService hardwarekey.Service) *Service {
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
	signature, err := s.agentSign(ctx, ref, keyInfo, rand, digest, opts)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to perform signature over hardware key agent, falling back to fallback service", "agent_err", err)
		signature, err = s.fallbackService.Sign(ctx, ref, keyInfo, rand, digest, opts)
	}

	return signature, trace.Wrap(err)
}

func (s *Service) agentSign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	slotKey, err := hardwarekey.PIVSlotKeyToProto(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(ref.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var hash hardwarekeyagentv1.Hash
	switch opts.HashFunc() {
	case 0:
		hash = hardwarekeyagentv1.Hash_HASH_NONE
	case crypto.SHA256:
		hash = hardwarekeyagentv1.Hash_HASH_SHA256
	case crypto.SHA512:
		hash = hardwarekeyagentv1.Hash_HASH_SHA512
	default:
		return nil, trace.BadParameter("unsupported hash func %q", opts.HashFunc().String())
	}

	var saltLength int
	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		if pssOpts.Hash == 0 {
			return nil, trace.BadParameter("hash must be specified for PSS signature")
		}

		rsaPub, ok := ref.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, trace.BadParameter("cannot perform PSS signature for non-rsa key")
		}

		saltLength = pssOpts.SaltLength

		// If the salt length is [rsa.PSSSaltLengthEqualsHash] or [rsa.PSSSaltLengthAuto],
		// pre-calculate the salt length so we can send it over the gRPC message.
		switch saltLength {
		case rsa.PSSSaltLengthEqualsHash:
			saltLength = pssOpts.Hash.Size()
		case rsa.PSSSaltLengthAuto:
			// We use the same salt length calculation as the crypto/rsa package.
			// https://github.com/golang/go/blob/21483099632c11743d01ec6f38577f31de26b0d0/src/crypto/internal/fips140/rsa/pkcs1v22.go#L253
			saltLength = (rsaPub.N.BitLen()-1+7)/8 - 2 - pssOpts.Hash.Size()
		}

		if saltLength < 0 {
			return nil, rsa.ErrMessageTooLong
		}
	}

	// Trim leading path (/ or \ on windows) from command for user readability.
	command := os.Args[0]
	if i := strings.LastIndexAny(command, "/\\"); i != -1 {
		command = command[i+1:]
	}
	commandString := fmt.Sprintf("%v %v", command, strings.Join(os.Args[1:], " "))

	req := &hardwarekeyagentv1.SignRequest{
		Digest:     digest,
		Hash:       hash,
		SaltLength: uint32(saltLength),
		KeyRef: &hardwarekeyagentv1.KeyRef{
			SerialNumber: ref.SerialNumber,
			SlotKey:      slotKey,
			PublicKeyDer: publicKeyDER,
		},
		KeyInfo: &hardwarekeyagentv1.KeyInfo{
			TouchRequired: ref.Policy.TouchRequired,
			PinRequired:   ref.Policy.PINRequired,
			PinCacheTtl:   durationpb.New(ref.PINCacheTTL),
			ProxyHost:     keyInfo.ProxyHost,
			Username:      keyInfo.Username,
			ClusterName:   keyInfo.ClusterName,
		},
		Command: commandString,
	}

	resp, err := s.agentClient.Sign(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Signature, nil
}

// TODO(Joerger): DELETE IN v19.0.0
func (s *Service) GetFullKeyRef(serialNumber uint32, slotKey hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	return s.fallbackService.GetFullKeyRef(serialNumber, slotKey)
}
