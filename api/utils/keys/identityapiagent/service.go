// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package identityapiagent

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"

	identityapiv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identityapi/v1"
	"github.com/gravitational/teleport/api/utils/keys/identityapi"
)

// Service is a gRPC-backed implementation of [identityapi.Service].
type Service struct {
	client identityapiv1.IdentityAPIServiceClient
}

// NewService creates a new identity-api signer service from the given client.
func NewService(client identityapiv1.IdentityAPIServiceClient) *Service {
	return &Service{client: client}
}

// NewServiceFromIdentityFile creates an identity-api signer service using the
// socket and pinned certificate colocated with the given identity file.
func NewServiceFromIdentityFile(identityPath string) (*Service, error) {
	socketPath, certPath, err := PathsFromIdentityFile(identityPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := credentials.NewClientTLSFromFile(certPath, "localhost")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := NewClient(socketPath, creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewService(client), nil
}

// Sign performs a cryptographic signature using the provided key reference.
func (s *Service) Sign(ctx context.Context, ref *identityapi.PrivateKeyRef, _ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	publicKeyDER, err := x509.MarshalPKIXPublicKey(ref.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var hash identityapiv1.Hash
	switch opts.HashFunc() {
	case 0:
		hash = identityapiv1.Hash_HASH_NONE
	case crypto.SHA256:
		hash = identityapiv1.Hash_HASH_SHA256
	case crypto.SHA512:
		hash = identityapiv1.Hash_HASH_SHA512
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
		switch saltLength {
		case rsa.PSSSaltLengthEqualsHash:
			saltLength = pssOpts.Hash.Size()
		case rsa.PSSSaltLengthAuto:
			saltLength = (rsaPub.N.BitLen()-1+7)/8 - 2 - pssOpts.Hash.Size()
		}

		if saltLength < 0 {
			return nil, rsa.ErrMessageTooLong
		}
	}

	resp, err := s.client.Sign(ctx, &identityapiv1.SignRequest{
		Digest:       digest,
		Hash:         hash,
		SaltLength:   uint32(saltLength),
		PublicKeyDer: publicKeyDER,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Signature, nil
}
