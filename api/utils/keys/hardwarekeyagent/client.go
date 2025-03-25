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
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
)

// AgentClient is a hardware key agent client implementation of [hardwarekey.Service].
type AgentClient struct {
	client        hardwarekeyagentv1.HardwareKeyAgentServiceClient
	directService hardwarekey.Service
}

// NewClient creates a new hardware key agent client. If the hardware key agent connection
// fails, this client will fallback to the given direct hardware key service.
func NewClient(ctx context.Context, directService hardwarekey.Service) (*AgentClient, error) {
	client, err := newClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AgentClient{
		client:        client,
		directService: directService,
	}, nil
}

func newClient() (hardwarekeyagentv1.HardwareKeyAgentServiceClient, error) {
	agentPath := filepath.Join(os.TempDir(), dirName, sockName)
	if _, err := os.Stat(agentPath); err != nil {
		return nil, trace.Wrap(err)
	}

	certPath := filepath.Join(os.TempDir(), dirName, certFileName)
	creds, err := credentials.NewClientTLSFromFile(certPath, "localhost")
	if err != nil {
		return nil, err
	}

	// The [grpc] library fails to resolve unix sockets on Windows, so
	// we provide "passthrough:" to skip grpc's address resolution and
	// a custom [net] dialer to connect to the socket.
	cc, err := grpc.NewClient("passthrough:", grpc.WithTransportCredentials(creds), grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
		return net.Dial("unix", agentPath)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(cc), nil
}

// NewPrivateKey creates or retrieves a hardware private key for the given config.
func (c *AgentClient) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.Signer, error) {
	return c.directService.NewPrivateKey(ctx, config)
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (c *AgentClient) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	signature, err := c.agentSign(ctx, ref, keyInfo, rand, digest, opts)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to perform signature over hardware key agent, falling back to direct hardware key service", "agent_err", err)
		signature, err = c.directService.Sign(ctx, ref, keyInfo, rand, digest, opts)
	}

	return signature, trace.Wrap(err)
}

func (c *AgentClient) agentSign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, _ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	slotKey, err := pivSlotKeyToProto(ref.SlotKey)
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
		saltLength = pssOpts.SaltLength

		// If the salt length is [rsa.PSSSaltLengthEqualsHash] or [rsa.PSSSaltLengthAuto],
		// pre-calculate the salt length so we can send it over the gRPC message.
		switch saltLength {
		case rsa.PSSSaltLengthEqualsHash:
			saltLength = opts.HashFunc().Size()
		case rsa.PSSSaltLengthAuto:
			rsaPub, ok := ref.PublicKey.(rsa.PublicKey)
			if !ok {
				return nil, trace.BadParameter("cannot perform PSS signature for non-rsa key")
			}

			// We use the same salt length calculation as the crypto/rsa package.
			// https://github.com/golang/go/blob/21483099632c11743d01ec6f38577f31de26b0d0/src/crypto/internal/fips140/rsa/pkcs1v22.go#L253
			saltLength = (rsaPub.N.BitLen()-1+7)/8 - 2 - opts.HashFunc().Size()
		}

		if saltLength < 0 {
			return nil, rsa.ErrMessageTooLong
		}

		if saltLength > math.MaxUint32 {
			return nil, trace.BadParameter("invalid salt length %d", saltLength)
		}
	}

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
			ProxyHost:     keyInfo.ProxyHost,
			Username:      keyInfo.Username,
			ClusterName:   keyInfo.ClusterName,
		},
		// TODO: Add command to sign request for prompt context.
	}

	resp, err := c.client.Sign(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Signature, nil
}

// TODO(Joerger): DELETE IN v19.0.0
func (c *AgentClient) GetFullKeyRef(serialNumber uint32, slotKey hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	return c.directService.GetFullKeyRef(serialNumber, slotKey)
}
