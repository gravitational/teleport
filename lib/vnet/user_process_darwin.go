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
	"os"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// runPlatformUserProcess launches a daemon in the background that will handle
// all networking and OS configuration. The user process exposes a gRPC
// interface that the daemon uses to query application names and get user
// certificates for apps. It returns a [ProcessManager] which controls the
// lifecycle of both the user and daemon processes.
func runPlatformUserProcess(ctx context.Context, cfg *UserProcessConfig) (*ProcessManager, *vnetv1.NetworkStackInfo, error) {
	ipcCreds, err := newIPCCredentials()
	if err != nil {
		return nil, nil, trace.Wrap(err, "creating credentials for IPC")
	}
	serverTLSConfig, err := ipcCreds.server.serverTLSConfig()
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating gRPC server TLS config")
	}

	credDir, err := os.MkdirTemp("", "vnet_service_certs")
	if err != nil {
		return nil, nil, trace.Wrap(err, "creating temp dir for service certs")
	}
	// Write credentials with 0200 so that only root can read them and no user
	// processes should be able to connect to the service.
	if err := ipcCreds.client.write(credDir, 0200); err != nil {
		return nil, nil, trace.Wrap(err, "writing service IPC credentials")
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil, trace.Wrap(err, "listening on tcp socket")
	}
	// grpcServer.Serve takes ownership of (and closes) the listener.
	grpcServer := grpc.NewServer(
		grpc.Creds(grpccredentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	clock := clockwork.NewRealClock()
	appProvider := newLocalAppProvider(cfg.ClientApplication, clock)
	svc := newClientApplicationService(appProvider)
	vnetv1.RegisterClientApplicationServiceServer(grpcServer, svc)

	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("admin process", func() error {
		defer func() {
			// Delete service credentials after the service terminates.
			if ipcCreds.client.remove(credDir); err != nil {
				log.ErrorContext(ctx, "Failed to remove service credential files", "error", err)
			}
			if err := os.RemoveAll(credDir); err != nil {
				log.ErrorContext(ctx, "Failed to remove service credential directory", "error", err)
			}
		}()
		daemonConfig := daemon.Config{
			ServiceCredentialPath:        credDir,
			ClientApplicationServiceAddr: listener.Addr().String(),
		}
		return trace.Wrap(execAdminProcess(processCtx, daemonConfig))
	})
	pm.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service",
			"addr", listener.Addr().String())
		return trace.Wrap(grpcServer.Serve(listener),
			"serving VNet user process gRPC service")
	})
	pm.AddCriticalBackgroundTask("gRPC server closer", func() error {
		// grpcServer.Serve does not stop on its own when processCtx is done, so
		// this task waits for processCtx and then explicitly stops grpcServer.
		<-processCtx.Done()
		grpcServer.GracefulStop()
		return nil
	})

	select {
	case nsi := <-svc.networkStackInfo:
		return pm, nsi, nil
	case <-processCtx.Done():
		return nil, nil, trace.Wrap(pm.Wait(), "process manager exited before network stack info was received")
	}
}
