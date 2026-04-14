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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net"
	"os"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	identityapiv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identityapi/v1"
)

// CurrentSignerFn returns the signer currently served by the identity-api
// endpoint. Implementations may rotate the signer over time.
type CurrentSignerFn func() (crypto.Signer, error)

// NewClient creates a new identity-api gRPC client.
func NewClient(socketPath string, creds credentials.TransportCredentials) (identityapiv1.IdentityAPIServiceClient, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := grpc.NewClient("passthrough:",
		grpc.WithTransportCredentials(creds),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return identityapiv1.NewIdentityAPIServiceClient(cc), nil
}

// NewServer returns a new identity-api gRPC server.
func NewServer(currentSigner CurrentSignerFn, creds credentials.TransportCredentials) (*grpc.Server, error) {
	if currentSigner == nil {
		return nil, trace.BadParameter("currentSigner must be provided")
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
	)
	identityapiv1.RegisterIdentityAPIServiceServer(grpcServer, &agentService{currentSigner: currentSigner})
	return grpcServer, nil
}

type agentService struct {
	identityapiv1.UnimplementedIdentityAPIServiceServer
	currentSigner CurrentSignerFn
}

// Sign the given digest with the current private key.
func (s *agentService) Sign(ctx context.Context, req *identityapiv1.SignRequest) (*identityapiv1.Signature, error) {
	signer, err := s.currentSigner()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.PublicKeyDer) != 0 {
		reqPublicKey, err := x509.ParsePKIXPublicKey(req.PublicKeyDer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !publicKeysEqual(signer.Public(), reqPublicKey) {
			return nil, trace.BadParameter("requested public key does not match the current signer")
		}
	}

	var signerOpts crypto.SignerOpts
	switch req.Hash {
	case identityapiv1.Hash_HASH_NONE:
		signerOpts = crypto.Hash(0)
	case identityapiv1.Hash_HASH_SHA256:
		signerOpts = crypto.SHA256
	case identityapiv1.Hash_HASH_SHA512:
		signerOpts = crypto.SHA512
	default:
		return nil, trace.BadParameter("unsupported hash %q", req.Hash.String())
	}

	if req.SaltLength > 0 {
		signerOpts = &rsa.PSSOptions{
			Hash:       signerOpts.HashFunc(),
			SaltLength: int(req.SaltLength),
		}
	}

	signature, err := signer.Sign(rand.Reader, req.Digest, signerOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &identityapiv1.Signature{Signature: signature}, nil
}

// Ping checks whether the server is alive.
func (s *agentService) Ping(ctx context.Context, req *identityapiv1.PingRequest) (*identityapiv1.PingResponse, error) {
	return &identityapiv1.PingResponse{Pid: uint32(os.Getpid())}, nil
}

func publicKeysEqual(a, b crypto.PublicKey) bool {
	comparableA, ok := a.(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return false
	}
	return comparableA.Equal(b)
}
