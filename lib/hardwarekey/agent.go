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

package hardwarekey

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/utils/cert"
)

const (
	dirName      = ".Teleport-PIV"
	sockName     = "agent.sock"
	certFileName = "cert.pem"
)

// DefaultAgentDir is the default dir for the hardware key agent's socket and certificate files.
func DefaultAgentDir() string {
	return filepath.Join(os.TempDir(), dirName)
}

// NewAgentClient opens a new hardware key agent client connected to the
// server based out of the given directory.
//
// [DefaultAgentDir] should be used for [keyAgentDir] outside of tests.
func NewAgentClient(ctx context.Context, keyAgentDir string) (hardwarekeyagentv1.HardwareKeyAgentServiceClient, error) {
	socketPath := filepath.Join(keyAgentDir, sockName)
	certPath := filepath.Join(keyAgentDir, certFileName)

	creds, err := credentials.NewClientTLSFromFile(certPath, "localhost")
	if err != nil {
		return nil, err
	}

	return hardwarekeyagent.NewClient(socketPath, creds)
}

// Server implementation [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	dir        string
}

// NewAgentServer returns a new hardware key agent server based out of the given directory.
// The given directory will be created when the server is served and destroyed with the server is stopped.
//
// [DefaultAgentDir] should be used for [keyAgentDir] outside of tests.
func NewAgentServer(ctx context.Context, s hardwarekey.Service, keyAgentDir string, knownKeyFn hardwarekeyagent.KnownHardwareKeyFn) (*Server, error) {
	if knownKeyFn == nil {
		return nil, trace.BadParameter("knownKeyFn must be provided")
	}

	if err := os.MkdirAll(keyAgentDir, 0o700); err != nil {
		return nil, trace.Wrap(err)
	}

	l, err := newAgentListener(ctx, keyAgentDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := generateServerCert(keyAgentDir)
	if err != nil {
		l.Close()
		return nil, trace.Wrap(err)
	}

	grpcServer, err := hardwarekeyagent.NewServer(s, credentials.NewServerTLSFromCert(&cert), knownKeyFn)
	if err != nil {
		l.Close()
		return nil, trace.Wrap(err)
	}

	return &Server{
		grpcServer: grpcServer,
		listener:   l,
		dir:        keyAgentDir,
	}, nil
}

func newAgentListener(ctx context.Context, keyAgentDir string) (net.Listener, error) {
	socketPath := filepath.Join(keyAgentDir, sockName)
	l, err := net.Listen("unix", socketPath)
	if err == nil {
		return l, nil
	} else if !errors.Is(err, errAddrInUse) {
		return nil, trace.Wrap(err)
	}

	// A hardware key agent already exists in the given path. Before replacing it,
	// try to connect to it and see if it is active.
	client, err := NewAgentClient(ctx, keyAgentDir)
	if err == nil {
		pong, err := client.Ping(ctx, &hardwarekeyagentv1.PingRequest{})
		if err == nil {
			return nil, trace.AlreadyExists("another agent instance is already running; PID: %d", pong.Pid)
		}
	}

	// If it isn't running, remove the socket and try again.
	if err := os.Remove(socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	if l, err = net.Listen("unix", socketPath); err != nil {
		return nil, trace.Wrap(err)
	}

	return l, nil
}

func generateServerCert(keyAgentDir string) (tls.Certificate, error) {
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil /*ipAddresses*/)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "failed to generate the certificate")
	}

	certPath := filepath.Join(keyAgentDir, certFileName)
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

// Serve the hardware key agent server.
func (s *Server) Serve(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Listening for hardware key agent requests")
	context.AfterFunc(ctx, s.Stop)
	return trace.Wrap(s.grpcServer.Serve(s.listener))
}

// Stop the hardware key agent server.
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
	if err := os.RemoveAll(s.dir); err != nil {
		slog.DebugContext(context.TODO(), "failed to clear hardware key agent directory")
	}
}
