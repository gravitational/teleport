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

// Package hardwarekeyagent provides a hardware key agent implementation of [hardwarekey.Service].
package hardwarekeyagent

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"log/slog"
	"net"
	"os"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/constants"
	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// NewClient creates a new hardware key agent client.
func NewClient(socketPath string, creds credentials.TransportCredentials) (hardwarekeyagentv1.HardwareKeyAgentServiceClient, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := grpc.NewClient("passthrough:",
		grpc.WithTransportCredentials(creds),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		// The [grpc] library fails to resolve unix sockets on Windows, so
		// we provide "passthrough:" to skip grpc's address resolution and
		// a custom [net] dialer to connect to the socket.
		grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(cc), nil
}

// NewServer returns a new hardware key agent server.
func NewServer(s hardwarekey.Service, creds credentials.TransportCredentials, knownKeyFn KnownHardwareKeyFn) (*grpc.Server, error) {
	if knownKeyFn == nil {
		return nil, trace.BadParameter("knownKeyFn must be provided")
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	hardwarekeyagentv1.RegisterHardwareKeyAgentServiceServer(grpcServer, &agentService{s: s, knownKeyFn: knownKeyFn})
	return grpcServer, nil
}

// KnownHardwareKeyFn is a function to determine if the hardware private key, described by the given
// key ref and key info, is known by this process. This is usually based on whether a matching key
// is found in the process's client key store.
type KnownHardwareKeyFn func(ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo) (bool, error)

// agentService implements [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type agentService struct {
	hardwarekeyagentv1.UnimplementedHardwareKeyAgentServiceServer
	s hardwarekey.Service

	// knownKeyFn is a function to determine if the hardware private key, described by the given
	// key ref and key info, is known by this process. This is usually based on whether a matching key
	// is found in the process's client key store.
	//
	// Unknown keys will treated with additional restrictions in [agentService.Sign] requests to
	// ensure the PIV slot is intended for Teleport client usage, e.g. the agent will require that
	// the PIV slot has a self-signed metadata certificate used to identify PIV keys generated
	// specifically for Teleport use.
	knownKeyFn KnownHardwareKeyFn
}

// Sign the given digest with the specified hardware private key.
func (s *agentService) Sign(ctx context.Context, req *hardwarekeyagentv1.SignRequest) (*hardwarekeyagentv1.Signature, error) {
	slotKey, err := hardwarekey.PIVSlotKeyFromProto(req.KeyRef.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pub, err := x509.ParsePKIXPublicKey(req.KeyRef.PublicKeyDer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRef := &hardwarekey.PrivateKeyRef{
		SerialNumber: req.KeyRef.SerialNumber,
		SlotKey:      slotKey,
		PublicKey:    pub,
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: req.KeyInfo.TouchRequired,
			PINRequired:   req.KeyInfo.PinRequired,
		},
		PINCacheTTL: req.KeyInfo.PinCacheTtl.AsDuration(),
	}

	// Double check that the client didn't provide some bogus pin cache TTL.
	if keyRef.PINCacheTTL > constants.MaxPIVPINCacheTTL {
		return nil, trace.BadParameter("pin_cache_ttl cannot be larger than %s", constants.MaxPIVPINCacheTTL)
	}

	keyInfo := hardwarekey.ContextualKeyInfo{
		ProxyHost:   req.KeyInfo.ProxyHost,
		Username:    req.KeyInfo.Username,
		ClusterName: req.KeyInfo.ClusterName,
	}

	knownKey, err := s.knownKeyFn(keyRef, keyInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyInfo.AgentKeyInfo = hardwarekey.AgentKeyInfo{
		UnknownAgentKey: !knownKey,
		Command:         req.Command,
	}

	var signerOpts crypto.SignerOpts
	switch req.Hash {
	case hardwarekeyagentv1.Hash_HASH_NONE:
		signerOpts = crypto.Hash(0)
	case hardwarekeyagentv1.Hash_HASH_SHA256:
		signerOpts = crypto.SHA256
	case hardwarekeyagentv1.Hash_HASH_SHA512:
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

	signature, err := s.s.Sign(ctx, keyRef, keyInfo, rand.Reader, req.Digest, signerOpts)
	if err != nil {
		slog.DebugContext(ctx, "hardware key agent signature failed", "error", err)
		return nil, trace.Wrap(err)
	}

	return &hardwarekeyagentv1.Signature{
		Signature: signature,
	}, nil
}

// Ping the server and get its PID.
func (s *agentService) Ping(ctx context.Context, req *hardwarekeyagentv1.PingRequest) (*hardwarekeyagentv1.PingResponse, error) {
	return &hardwarekeyagentv1.PingResponse{
		Pid: uint32(os.Getpid()),
	}, nil
}
