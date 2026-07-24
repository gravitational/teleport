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

// Package localagent provides a local gRPC server over a Unix socket,
// authenticated by a pinned self-signed server certificate.
package localagent

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils/cert"
)

const (
	// SocketFileName is the local agent Unix socket filename.
	SocketFileName = "agent.sock"

	// CertFileName is the local agent server certificate filename.
	CertFileName = "cert.pem"

	activeAgentDialTimeout = 500 * time.Millisecond
)

// Server is a local gRPC server over a Unix socket, authenticated by a pinned
// self-signed server certificate.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	dir        string
	logger     *slog.Logger
}

// ActiveConnectionCheck checks whether an existing local agent connection is
// active. It should return nil when the existing agent should be preserved.
type ActiveConnectionCheck func(ctx context.Context, conn *grpc.ClientConn) error

type serverConfig struct {
	logger                *slog.Logger
	activeConnectionCheck ActiveConnectionCheck
}

// ServerOption configures a local agent server.
type ServerOption func(*serverConfig)

// WithLogger configures the logger for the local agent server.
func WithLogger(logger *slog.Logger) ServerOption {
	return func(cfg *serverConfig) {
		cfg.logger = logger
	}
}

// WithActiveConnectionCheck configures the RPC used to check whether an
// existing local agent socket belongs to an active server. If the check returns
// an error, the socket is treated as stale and replaced.
func WithActiveConnectionCheck(check ActiveConnectionCheck) ServerOption {
	return func(cfg *serverConfig) {
		cfg.activeConnectionCheck = check
	}
}

// NewServer returns a new local agent server based out of the given directory.
// The given directory is created with owner-only permissions if it does not
// exist.
func NewServer(dir string, opts ...ServerOption) (*Server, error) {
	var cfg serverConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, trace.Wrap(err)
	}

	l, err := newListener(dir, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverCert, err := generateServerCert(dir)
	if err != nil {
		l.Close()
		return nil, trace.Wrap(err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)),
		grpc.ChainUnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)

	return &Server{
		grpcServer: grpcServer,
		listener:   l,
		dir:        dir,
		logger:     cfg.logger,
	}, nil
}

// NewClient opens a new local agent client connection based out of the given
// directory.
func NewClient(dir string) (*grpc.ClientConn, error) {
	socketPath := filepath.Join(dir, SocketFileName)
	certPath := filepath.Join(dir, CertFileName)

	creds, err := credentials.NewClientTLSFromFile(certPath, "localhost")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := os.Stat(socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := grpc.NewClient("passthrough:",
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithChainStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		// The gRPC library fails to resolve Unix sockets on Windows, so skip
		// address resolution and connect to the socket with a custom dialer.
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cc, nil
}

// RegisterService satisfies [grpc.ServiceRegistrar].
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl any) {
	s.grpcServer.RegisterService(desc, impl)
}

// Serve starts the local agent server.
func (s *Server) Serve(ctx context.Context) error {
	context.AfterFunc(ctx, func() { s.Stop(context.Background()) })
	return trace.Wrap(s.grpcServer.Serve(s.listener))
}

// Stop gracefully stops the local agent server and removes the socket and
// certificate files.
func (s *Server) Stop(ctx context.Context) {
	s.grpcServer.GracefulStop()

	if err := os.Remove(filepath.Join(s.dir, SocketFileName)); err != nil {
		s.logger.DebugContext(ctx, "failed to remove local agent socket", "error", err)
	}

	if err := os.Remove(filepath.Join(s.dir, CertFileName)); err != nil {
		s.logger.DebugContext(ctx, "failed to remove local agent certificate", "error", err)
	}
}

func newListener(dir string, cfg serverConfig) (net.Listener, error) {
	socketPath := filepath.Join(dir, SocketFileName)

	l, err := net.Listen("unix", socketPath)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, errAddrInUse) {
		return nil, trace.Wrap(err)
	}

	// Check if the socket is in-use by another active instance.
	if cfg.activeConnectionCheck != nil {
		if active, err := existingAgentActive(dir, cfg.activeConnectionCheck); err != nil {
			return nil, trace.Wrap(err)
		} else if active {
			return nil, trace.AlreadyExists("another local agent instance is already running")
		}
	}

	// Remove the socket and retry.
	if err := os.Remove(socketPath); err != nil {
		return nil, trace.Wrap(err)
	}
	if l, err = net.Listen("unix", socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	return l, nil
}

func existingAgentActive(dir string, checkFn ActiveConnectionCheck) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), activeAgentDialTimeout)
	defer cancel()

	cc, err := NewClient(dir)
	if err != nil {
		return false, nil
	}
	defer cc.Close()

	if err := checkFn(ctx, cc); err != nil {
		return false, nil
	}

	return true, nil
}

func generateServerCert(dir string) (tls.Certificate, error) {
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil, nil, time.Now)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "failed to generate the certificate")
	}

	certPath := filepath.Join(dir, CertFileName)
	f, err := os.OpenFile(certPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
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
