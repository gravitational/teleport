// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// hardwarekeyagent provides an agent implementation of [hardwarekey.Service],
// used to share service state across process boundaries.
package hardwarekeyagent

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/lib/utils/cert"
)

const (
	dirName      = ".Teleport-PIV"
	sockName     = "agent.sock"
	certFileName = "cert.pem"
)

// Server implementation [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type Server struct {
	hardwarekeyagentv1.UnimplementedHardwareKeyAgentServiceServer
	s hardwarekey.Service
}

// NewServer returns a new hardware key agent server.
func NewServer(s hardwarekey.Service) *Server {
	return &Server{s: s}
}

// RunServer runs a new [hardwarekeyagentv1.HardwareKeyAgentServiceServer] using the service.
func (s *Server) RunServer(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	l, err := newAgentListener(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	context.AfterFunc(ctx, func() { l.Close() })

	cert, err := generateServerCert()
	if err != nil {
		return trace.Wrap(err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	hardwarekeyagentv1.RegisterHardwareKeyAgentServiceServer(grpcServer, s)

	fmt.Fprintln(os.Stderr, "Listening for hardware key agent requests")
	return trace.Wrap(grpcServer.Serve(l))
}

func newAgentListener(ctx context.Context) (net.Listener, error) {
	keyAgentDir := filepath.Join(os.TempDir(), dirName)
	if err := os.MkdirAll(keyAgentDir, 0o700); err != nil {
		return nil, trace.Wrap(err)
	}

	keyAgentPath := filepath.Join(keyAgentDir, sockName)
	l, err := net.Listen("unix", keyAgentPath)
	if err == nil {
		return l, nil
	} else if !errors.Is(err, syscall.EADDRINUSE) {
		return nil, trace.Wrap(err)
	}

	// A hardware key agent already exists in the given path. Before replacing it,
	// try to connect to it and see if it is active.
	client, err := newClient()
	if err == nil {
		pong, err := client.Ping(ctx, &hardwarekeyagentv1.PingRequest{})
		if err == nil {
			return nil, trace.AlreadyExists("another agent instance is already running; PID: %d", pong.Pid)
		}
	}

	// If it isn't running, remove the agent dir and try again.
	if err := os.RemoveAll(keyAgentDir); err != nil {
		return nil, trace.Wrap(err)
	}

	return newAgentListener(ctx)
}

func generateServerCert() (tls.Certificate, error) {
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil /*ipAddresses*/)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "failed to generate the certificate")
	}

	certPath := filepath.Join(os.TempDir(), dirName, certFileName)
	f, err := os.OpenFile(certPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if _, err = f.Write(creds.Cert); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if err = f.Close(); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return keys.X509KeyPair(creds.Cert, creds.PrivateKey)
}

// Sign the given digest with the specified hardware private key.
func (s *Server) Sign(ctx context.Context, req *hardwarekeyagentv1.SignRequest) (*hardwarekeyagentv1.Signature, error) {
	slotKey, err := pivSlotKeyFromProto(req.KeyRef.SlotKey)
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
	}

	keyInfo := hardwarekey.ContextualKeyInfo{
		ProxyHost:   req.KeyInfo.ProxyHost,
		Username:    req.KeyInfo.Username,
		ClusterName: req.KeyInfo.ClusterName,
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
		return nil, trace.Wrap(err)
	}

	return &hardwarekeyagentv1.Signature{
		Signature: signature,
	}, nil
}

// Ping the server and get its PID.
func (s *Server) Ping(ctx context.Context, req *hardwarekeyagentv1.PingRequest) (*hardwarekeyagentv1.PingResponse, error) {
	return &hardwarekeyagentv1.PingResponse{
		Pid: uint32(os.Getpid()),
	}, nil
}

func pivSlotKeyFromProto(pivSlot hardwarekeyagentv1.PIVSlotKey) (hardwarekey.PIVSlotKey, error) {
	switch pivSlot {
	case hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9A:
		return 0x9a, nil
	case hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9C:
		return 0x9c, nil
	case hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9D:
		return 0x9d, nil
	case hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9E:
		return 0x9e, nil
	default:
		return 0, trace.BadParameter("unknown piv slot key for proto enum %d", pivSlot)
	}
}

func pivSlotKeyToProto(slotKey hardwarekey.PIVSlotKey) (hardwarekeyagentv1.PIVSlotKey, error) {
	switch slotKey {
	case 0x9a:
		return hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9A, nil
	case 0x9c:
		return hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9C, nil
	case 0x9d:
		return hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9D, nil
	case 0x9e:
		return hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9E, nil
	default:
		return 0, trace.BadParameter("unknown proto enum for piv slot key %d", slotKey)
	}
}
