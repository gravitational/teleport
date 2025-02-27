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
	"crypto/rsa"
	"crypto/x509"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
)

// NewHardwareKeyAgentKey returns a new hardware key agent key.
func NewHardwareKeyAgentKey(keyRef *YubiKeyPrivateKeyRef, keyPEM []byte) (*PrivateKey, error) {
	agentPath := filepath.Join(os.TempDir(), dirName, sockName)
	if _, err := os.Stat(agentPath); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := grpc.NewClient("unix://"+agentPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := PIVSlotUintToProto(keyRef.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	protoKeyRef := &hardwarekeyagentv1.KeyRef{
		SerialNumber: keyRef.SerialNumber,
		PivSlot:      pivSlot,
	}

	agentClient := hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(cc)
	att, err := agentClient.GetAttestation(context.TODO(), &hardwarekeyagentv1.GetAttestationRequest{
		KeyRef: protoKeyRef,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slotCert, err := x509.ParseCertificate(att.AttestationStatement.GetYubikeyAttestationStatement().SlotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Ping to determine if the agent is alive, returning an error if it is not

	agentSigner := &hardwareKeyAgentKey{
		agent:     agentClient,
		keyRef:    protoKeyRef,
		publicKey: slotCert.PublicKey,
		// TODO: get policy from attestation on server side
		privateKeyPolicy: PrivateKeyPolicyHardwareKeyTouchAndPIN,
		att:              att.AttestationStatement,
	}

	return NewPrivateKey(agentSigner, keyPEM)
}

type hardwareKeyAgentKey struct {
	agent            hardwarekeyagentv1.HardwareKeyAgentServiceClient
	keyRef           *hardwarekeyagentv1.KeyRef
	publicKey        crypto.PublicKey
	att              *attestationv1.AttestationStatement
	privateKeyPolicy PrivateKeyPolicy
}

func (s *hardwareKeyAgentKey) Public() crypto.PublicKey {
	return s.publicKey
}

func (s *hardwareKeyAgentKey) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	req := &hardwarekeyagentv1.SignRequest{
		KeyRef: s.keyRef,
		Digest: digest,
	}

	if opts == nil {
		return nil, trace.BadParameter("hash func should be provided")
	}

	switch opts.HashFunc() {
	case crypto.SHA256:
		req.HashName = hardwarekeyagentv1.HashName_HASH_NAME_SHA256
	case crypto.SHA512:
		req.HashName = hardwarekeyagentv1.HashName_HASH_NAME_SHA512
	default:
		return nil, trace.BadParameter("unsupported hash func %q", opts.HashFunc().String())
	}

	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		switch pssOpts.SaltLength {
		case rsa.PSSSaltLengthEqualsHash:
			req.SaltLength = &hardwarekeyagentv1.SignRequest_Auto{
				Auto: hardwarekeyagentv1.SaltLengthAuto_SALT_LENGTH_AUTO_HASH_LENGTH,
			}
		case rsa.PSSSaltLengthAuto:
			req.SaltLength = &hardwarekeyagentv1.SignRequest_Auto{
				Auto: hardwarekeyagentv1.SaltLengthAuto_SALT_LENGTH_AUTO_MAX,
			}
		default:
			req.SaltLength = &hardwarekeyagentv1.SignRequest_Length{
				Length: uint32(pssOpts.SaltLength),
			}
		}
	}

	signature, err := s.agent.Sign(context.TODO(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return signature.Signature, nil
}

// GetAttestationStatement returns an AttestationStatement for this private key.
func (s *hardwareKeyAgentKey) GetAttestationStatement() *AttestationStatement {
	return AttestationStatementFromProto(s.att)
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
func (s *hardwareKeyAgentKey) GetPrivateKeyPolicy() PrivateKeyPolicy {
	return s.privateKeyPolicy
}
