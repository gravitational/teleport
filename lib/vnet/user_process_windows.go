// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// runPlatformUserProcess launches a Windows service in the background that will
// handle all networking and OS configuration. The user process exposes a gRPC
// interface that the admin process uses to query application names and get user
// certificates for apps. It returns a [ProcessManager] which controls the
// lifecycle of both the user and admin processes.
func runPlatformUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, err error) {
	// Make sure to close the process manager if returning a non-nil error.
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, trace.Wrap(err, "listening on tcp socket")
	}
	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("tcp socket closer", func() error {
		<-processCtx.Done()
		return trace.Wrap(listener.Close())
	})
	pm.AddCriticalBackgroundTask("admin process", func() error {
		return trace.Wrap(runService(processCtx, &windowsAdminProcessConfig{
			clientApplicationServiceAddr: listener.Addr().String(),
		}))
	})
	pm.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service", "addr", listener.Addr().String())
		// TODO(nklaassen): add mTLS credentials for client application service.
		grpcServer := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()),
			grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
			grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		)
		clock := clockwork.NewRealClock()
		appProvider := newLocalAppProvider(config.ClientApplication, clock)
		svc := newClientApplicationService(appProvider)
		vnetv1.RegisterClientApplicationServiceServer(grpcServer, svc)
		if err := grpcServer.Serve(listener); err != nil {
			return trace.Wrap(err, "serving VNet user process gRPC service")
		}
		return nil
	})
	return pm, nil
}
