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
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// runPlatformUserProcess launches a daemon in the background that will handle
// all networking and OS configuration. The user process exposes a gRPC
// interface that the daemon uses to query application names and get user
// certificates for apps. If successful it sets p.processManager and
// p.networkStackInfo.
func (p *UserProcess) runPlatformUserProcess(processCtx context.Context) error {
	ipcCreds, err := newIPCCredentials()
	if err != nil {
		return trace.Wrap(err, "creating credentials for IPC")
	}
	serverTLSConfig, err := ipcCreds.server.serverTLSConfig()
	if err != nil {
		return trace.Wrap(err, "generating gRPC server TLS config")
	}

	credDir, err := os.MkdirTemp("", "vnet_service_certs")
	if err != nil {
		return trace.Wrap(err, "creating temp dir for service certs")
	}
	// Write credentials with 0200 so that only root can read them and no user
	// processes should be able to connect to the service.
	if err := ipcCreds.client.write(credDir, 0200); err != nil {
		return trace.Wrap(err, "writing service IPC credentials")
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return trace.Wrap(err, "listening on tcp socket")
	}
	// grpcServer.Serve takes ownership of (and closes) the listener.
	grpcServer := grpc.NewServer(
		grpc.Creds(grpccredentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	vnetv1.RegisterClientApplicationServiceServer(grpcServer, p.clientApplicationService)

	p.processManager.AddCriticalBackgroundTask("admin process", func() error {
		defer func() {
			// Delete service credentials after the service terminates.
			if ipcCreds.client.remove(credDir); err != nil {
				log.ErrorContext(processCtx, "Failed to remove service credential files", "error", err)
			}
			if err := os.RemoveAll(credDir); err != nil {
				log.ErrorContext(processCtx, "Failed to remove service credential directory", "error", err)
			}
		}()
		daemonConfig := daemon.Config{
			ServiceCredentialPath:        credDir,
			ClientApplicationServiceAddr: listener.Addr().String(),
		}
		return trace.Wrap(execAdminProcess(processCtx, daemonConfig))
	})
	p.processManager.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service",
			"addr", listener.Addr().String())
		return trace.Wrap(grpcServer.Serve(listener),
			"serving VNet user process gRPC service")
	})
	p.processManager.AddCriticalBackgroundTask("gRPC server closer", func() error {
		// grpcServer.Serve does not stop on its own when processCtx is done, so
		// this task waits for processCtx and then explicitly stops grpcServer.
		<-processCtx.Done()
		grpcServer.GracefulStop()
		return nil
	})

	select {
	case nsi := <-p.clientApplicationService.networkStackInfo:
		p.networkStackInfo = nsi
		return nil
	case <-processCtx.Done():
		return trace.Wrap(p.processManager.Wait(), "process manager exited before network stack info was received")
	}
}
