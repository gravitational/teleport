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
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/localagent"
)

const (
	// SocketFileName is the name of the socket on which the Hardware Key Agent
	// service will listen.
	SocketFileName = localagent.SocketFileName

	// CertFileName is the name of the file containing the Hardware Key Agent
	// server certificate.
	CertFileName = localagent.CertFileName

	dirName          = ".Teleport-PIV"
	keyAgentDirEnv   = "TELEPORT_KEY_AGENT_DIR"
	loginAgentDirEnv = "TELEPORT_LOGIN_AGENT_DIR"
)

// AgentDirFromEnv returns the directory for the hardware key agent's socket and
// certificate files from $TELEPORT_KEY_AGENT_DIR, $TELEPORT_LOGIN_AGENT_DIR, or
// defaultDir if neither environment variable was set.
func AgentDirFromEnv(defaultDir string) string {
	if dir := os.Getenv(keyAgentDirEnv); dir != "" {
		return dir
	}
	if dir := os.Getenv(loginAgentDirEnv); dir != "" {
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
func NewAgentClient(ctx context.Context, keyAgentDir string) (hardwarekeyagentv1.HardwareKeyAgentServiceClient, error) {
	cc, err := localagent.NewClient(keyAgentDir)
	if err != nil {
		return nil, err
	}
	return hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(cc), nil
}

// Server implementation [hardwarekeyagentv1.HardwareKeyAgentServiceServer].
type Server struct {
	server *localagent.Server
}

// NewAgentServer returns a new hardware key agent server based out of the given
// directory. The given directory will be created when the server is served and
// destroyed with the server is stopped.
//
// [DefaultAgentDir] should be used for [keyAgentDir] outside of tests.
func NewAgentServer(ctx context.Context, s hardwarekey.Service, keyAgentDir string, knownKeyFn hardwarekeyagent.KnownHardwareKeyFn) (*Server, error) {
	checkActiveConn := func(ctx context.Context, conn *grpc.ClientConn) error {
		_, err := hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(conn).
			Ping(ctx, &hardwarekeyagentv1.PingRequest{})
		return trace.Wrap(err)
	}

	localAgent, err := localagent.NewServer(keyAgentDir,
		localagent.WithActiveConnectionCheck(checkActiveConn),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := NewAgentServerWithLocalAgent(ctx, localAgent, s, knownKeyFn)
	if err != nil {
		localAgent.Stop(ctx)
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// NewAgentServerWithLocalAgent returns a hardware key agent server using an
// existing local agent server.
func NewAgentServerWithLocalAgent(
	ctx context.Context,
	localAgent *localagent.Server,
	s hardwarekey.Service,
	knownKeyFn hardwarekeyagent.KnownHardwareKeyFn,
) (*Server, error) {
	if err := hardwarekeyagent.RegisterServer(localAgent, s, knownKeyFn); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Server{server: localAgent}, nil
}

// Serve the hardware key agent server.
func (s *Server) Serve(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Listening for hardware key agent requests")
	return trace.Wrap(s.server.Serve(ctx))
}

// Stop the hardware key agent server.
func (s *Server) Stop() {
	s.server.Stop(context.TODO())
}
