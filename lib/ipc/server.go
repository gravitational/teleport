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

package ipc

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
	// SocketFileName is the filename of the Unix socket used for IPC.
	SocketFileName = "agent.sock"

	// CertFileName is the filename of the self-signed TLS certificate presented
	// by the IPC server.
	CertFileName = "cert.pem"
)

// Server is a gRPC server used for inter-process communication between
// processes on the same host. It listens on a Unix socket and uses TLS with
// a self-signed server certificate to encrypt client/server communication.
//
// Server implements the grpc.ServiceRegistrar interface, so it can be passed
// directly to service registration methods.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	dir        string
	logger     *slog.Logger
}

// ActiveSocketCheck checks whether the server that created an existing IPC
// socket is still alive. If the check returns an error, the socket is considered
// stale and will be replaced.
type ActiveSocketCheck func(ctx context.Context, conn *grpc.ClientConn) error

type serverConfig struct {
	logger            *slog.Logger
	activeSocketCheck ActiveSocketCheck
}

// ServerOption provides optional configuration for the Server.
type ServerOption func(*serverConfig)

// WithLogger configures the logger for the Server.
func WithLogger(logger *slog.Logger) ServerOption {
	return func(cfg *serverConfig) {
		cfg.logger = logger
	}
}

// WithActiveSocketCheck configures the RPC used to check whether the server
// that created an existing IPC socket is still alive.
func WithActiveSocketCheck(check ActiveSocketCheck) ServerOption {
	return func(cfg *serverConfig) {
		cfg.activeSocketCheck = check
	}
}

// NewServer creates a new IPC server. It will listen on a unix socket file in
// the given directory. The given context is used to support cancellation when
// checking whether an existing socket is active or stale, it does not determine
// the lifetime of the server itself (the context passed to Serve does this).
func NewServer(ctx context.Context, dir string, opts ...ServerOption) (*Server, error) {
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

	l, err := newListener(ctx, dir, cfg)
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

// NewClient connects to the IPC server listening on the unix socket file in the
// given directory.
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

// RegisterService implements grpc.ServiceRegistrar.
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl any) {
	s.grpcServer.RegisterService(desc, impl)
}

// Serve runs the server until the given context is canceled or reaches its
// deadline.
func (s *Server) Serve(ctx context.Context) error {
	context.AfterFunc(ctx, func() { s.Stop(context.Background()) })
	return trace.Wrap(s.grpcServer.Serve(s.listener))
}

// Stop gracefully stops the server and removes the socket and certificate files.
func (s *Server) Stop(ctx context.Context) {
	s.grpcServer.GracefulStop()

	if err := os.Remove(filepath.Join(s.dir, SocketFileName)); err != nil {
		s.logger.DebugContext(ctx, "failed to remove IPC socket", "error", err)
	}

	if err := os.Remove(filepath.Join(s.dir, CertFileName)); err != nil {
		s.logger.DebugContext(ctx, "failed to remove IPC certificate", "error", err)
	}
}

func newListener(ctx context.Context, dir string, cfg serverConfig) (net.Listener, error) {
	socketPath := filepath.Join(dir, SocketFileName)

	l, err := net.Listen("unix", socketPath)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, errAddrInUse) {
		return nil, trace.Wrap(err)
	}

	// No way to determine whether the socket is active.
	if cfg.activeSocketCheck == nil {
		return nil, trace.AlreadyExists("another instance is already running")
	}

	// Check if the socket is in-use by another active instance.
	if active, err := existingServerActive(ctx, dir, cfg.activeSocketCheck); err != nil {
		return nil, trace.Wrap(err)
	} else if active {
		return nil, trace.AlreadyExists("another instance is already running")
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

func existingServerActive(ctx context.Context, dir string, checkFn ActiveSocketCheck) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
	defer f.Close()
	if _, err = f.Write(creds.Cert); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if err = f.Close(); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return keys.X509KeyPair(creds.Cert, creds.PrivateKey)
}
