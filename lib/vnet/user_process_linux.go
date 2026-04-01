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
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/uds"
)

const clientApplicationServiceSocketName = "vnet.sock"

// runPlatformUserProcess launches a daemon in the background that will handle
// all networking and OS configuration. The user process exposes a gRPC
// interface that the daemon uses to query application names and get user
// certificates for apps. If successful it sets p.processManager and
// p.networkStackInfo.
func (p *UserProcess) runPlatformUserProcess(processCtx context.Context) error {
	// Prefer XDG_RUNTIME_DIR for runtime sockets.
	// Paths must be absolute, ignore relative values.
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" || !filepath.IsAbs(runtimeDir) {
		runtimeDir = os.TempDir()
	}
	socketDir, err := os.MkdirTemp(runtimeDir, "vnet_service")
	if err != nil {
		return trace.Wrap(err, "creating temp dir for service socket")
	}

	listener, socketPath, err := listenUnixSocket(socketDir)
	if err != nil {
		if removeErr := os.RemoveAll(socketDir); removeErr != nil {
			log.ErrorContext(processCtx, "Failed to remove service socket directory", "error", removeErr)
		}
		return trace.Wrap(err, "listening on unix socket")
	}
	// grpcServer.Serve takes ownership of (and closes) the listener.
	grpcServer := grpc.NewServer(
		grpc.Creds(uds.NewTransportCredentials(insecure.NewCredentials())),
		grpc.ChainUnaryInterceptor(
			rootOnlyUnixSocketUnaryInterceptor,
			interceptors.GRPCServerUnaryErrorInterceptor,
		),
		grpc.ChainStreamInterceptor(
			rootOnlyUnixSocketStreamInterceptor,
			interceptors.GRPCServerStreamErrorInterceptor,
		),
	)
	vnetv1.RegisterClientApplicationServiceServer(grpcServer, p.clientApplicationService)

	p.processManager.AddCriticalBackgroundTask("admin process", func() error {
		defer func() {
			// Delete vnet socket after the service terminates.
			if err := os.RemoveAll(socketDir); err != nil {
				log.ErrorContext(processCtx, "Failed to remove service socket directory", "error", err)
			}
		}()
		return trace.Wrap(execAdminProcess(processCtx, LinuxAdminProcessConfig{
			ClientApplicationServiceSocketPath: socketPath,
		}))
	})
	p.processManager.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service",
			"socket", socketPath)
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

func listenUnixSocket(dir string) (net.Listener, string, error) {
	socketPath := filepath.Join(dir, clientApplicationServiceSocketName)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating unix socket listener")
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return nil, "", trace.Wrap(err, "chmod unix socket %s", socketPath)
	}
	return listener, socketPath, nil
}

func rootOnlyUnixSocketUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	if err := requireRootUnixSocketPeer(ctx); err != nil {
		return nil, trace.Wrap(err, "validating unix socket peer for unary call %s", info.FullMethod)
	}
	return handler(ctx, req)
}

func rootOnlyUnixSocketStreamInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if err := requireRootUnixSocketPeer(ss.Context()); err != nil {
		return trace.Wrap(err, "validating unix socket peer for stream call %s", info.FullMethod)
	}
	return handler(srv, ss)
}

func requireRootUnixSocketPeer(ctx context.Context) error {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "gRPC peer not found in context")
	}
	authInfo, ok := p.AuthInfo.(uds.AuthInfo)
	if !ok || authInfo.Creds == nil {
		return status.Error(codes.Unauthenticated, "missing unix socket peer credentials")
	}
	if authInfo.Creds.UID != 0 {
		return status.Errorf(codes.PermissionDenied, "unix socket peer uid %d is not root", authInfo.Creds.UID)
	}
	return nil
}
