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
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	hardwarekeyagentv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/hardwarekeyagent/v1"
)

const (
	dirName  = ".Teleport-PIV"
	sockName = "agent.sock"
)

// RunServer runs a new [hardwarekeyagentv1.HardwareKeyAgentServiceServer] using the service.
func (s *Service) RunServer(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	grpcServer := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	hardwarekeyagentv1.RegisterHardwareKeyAgentServiceServer(grpcServer, s)

	keyAgentDir := filepath.Join(os.TempDir(), dirName)
	if err := os.Mkdir(keyAgentDir, 0o600); err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(keyAgentDir)

	keyAgentPath := filepath.Join(os.TempDir(), dirName, sockName)
	l, err := net.Listen("unix", keyAgentPath)
	if err != nil {
		return trace.Wrap(err)
	}
	context.AfterFunc(ctx, func() { l.Close() })

	// TODO: overwrite unix socket if it exists?

	fmt.Fprintln(os.Stderr, "Listening for hardware key agent requests")
	return trace.Wrap(grpcServer.Serve(l))
}
