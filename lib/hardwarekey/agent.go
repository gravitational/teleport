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
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/ipc"
)

const (
	// SocketFileName is the name of the socket on which the Hardware Key Agent
	// service will listen.
	SocketFileName = ipc.SocketFileName

	// CertFileName is the name of the file containing the Hardware Key Agent
	// server certificate.
	CertFileName = ipc.CertFileName

	dirName     = ".Teleport-PIV"
	agentDirEnv = "TELEPORT_KEY_AGENT_DIR"
)

// AgentDirFromEnv returns the directory for the hardware key agent's socket and
// certificate files from $TELEPORT_KEY_AGENT_DIR, or defaultDir if the environment
// variable was not set.
func AgentDirFromEnv(defaultDir string) string {
	if dir := os.Getenv(agentDirEnv); dir != "" {
		return dir
	}
	return defaultDir
}

// DefaultAgentDir is the default dir for the hardware key agent's socket and certificate files.
func DefaultAgentDir() string {
	return filepath.Join(os.TempDir(), dirName)
}

// NewAgentClient opens a new hardware key agent client connected to the
// server based out of the given directory.
//
// [DefaultAgentDir] should be used for [keyAgentDir] outside of tests.
func NewAgentClient(keyAgentDir string) (hardwarekeyagentv1.HardwareKeyAgentServiceClient, error) {
	client, err := ipc.NewClient(keyAgentDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(client), nil
}

// Server implementation [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type Server struct {
	ipcServer *ipc.Server
	dir       string
}

// NewAgentServer returns a new hardware key agent server based out of the given
// directory. The given directory will be created when the server is served.
//
// [DefaultAgentDir] should be used for [keyAgentDir] outside of tests.
func NewAgentServer(ctx context.Context, s hardwarekey.Service, keyAgentDir string, knownKeyFn hardwarekeyagent.KnownHardwareKeyFn) (*Server, error) {
	if knownKeyFn == nil {
		return nil, trace.BadParameter("knownKeyFn must be provided")
	}

	server, err := ipc.NewServer(ctx, keyAgentDir,
		ipc.WithActiveSocketCheck(func(ctx context.Context, conn *grpc.ClientConn) error {
			_, err := hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(conn).
				Ping(ctx, &hardwarekeyagentv1.PingRequest{})
			return trace.Wrap(err)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hardwarekeyagent.RegisterServer(server, s, knownKeyFn)

	return &Server{
		ipcServer: server,
		dir:       keyAgentDir,
	}, nil
}

// Serve the hardware key agent server.
func (s *Server) Serve(ctx context.Context) error {
	return trace.Wrap(s.ipcServer.Serve(ctx))
}

// Stop the hardware key agent server.
func (s *Server) Stop() {
	s.ipcServer.Stop(context.Background())
}
