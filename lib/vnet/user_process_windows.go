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
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// UserProcessConfig provides the necessary configuration to run VNet.
type UserProcessConfig struct {
	// LocalAppProvider is a required field providing an interface
	// implementation for [LocalAppProvider].
	LocalAppProvider LocalAppProvider
	// ClusterConfigCache is an optional field providing [ClusterConfigCache]. If empty, a new cache
	// will be created.
	ClusterConfigCache *ClusterConfigCache
	// HomePath is the tsh home used for Teleport clients created by VNet. Resolved using the same
	// rules as HomeDir in tsh.
	HomePath string
}

func (c *UserProcessConfig) CheckAndSetDefaults() error {
	if c.LocalAppProvider == nil {
		return trace.BadParameter("missing LocalAppProvider")
	}
	if c.HomePath == "" {
		c.HomePath = profile.FullProfilePath(os.Getenv(types.HomeEnvVar))
	}
	return nil
}

// RunUserProcess launches a Windows service in the background that in turn
// calls [RunAdminProcess]. The user process exposes a gRPC service that the
// admin process uses to query application names and get user certificates for
// apps.
//
// RunUserProcess returns a [ProcessManager] which controls the lifecycle of
// both the user and admin processes.
//
// The caller is expected to call Close on the process manager to clean up any
// resources and terminate the admin process, which will in turn stop the
// networking stack and deconfigure the host OS.
//
// ctx is used to wait for setup steps that happen before RunUserProcess hands out the
// control to the process manager. If ctx gets canceled during RunUserProcess, the process
// manager gets closed along with its background tasks.
func RunUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, err error) {
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, trace.Wrap(err, "listening on tcp socket")
	}
	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("socket closer", func() error {
		<-processCtx.Done()
		return trace.Wrap(listener.Close())
	})
	pm.AddCriticalBackgroundTask("admin process", func() error {
		adminConfig := AdminProcessConfig{
			UserProcessServiceAddr: listener.Addr().String(),
		}
		return trace.Wrap(execAdminProcess(processCtx, adminConfig))
	})
	pm.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service", "addr", listener.Addr().String())
		grpcServer := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()),
			grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
			grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		)
		svc, err := newUserProcessService()
		if err != nil {
			return trace.Wrap(err)
		}
		vnetv1.RegisterVnetUserProcessServiceServer(grpcServer, svc)
		if err := grpcServer.Serve(listener); err != nil {
			return trace.Wrap(err, "serving VNet user process gRPC service")
		}
		return nil
	})
	return pm, nil
}

type userProcessService struct {
	vnetv1.UnsafeVnetUserProcessServiceServer
}

func newUserProcessService() (*userProcessService, error) {
	return &userProcessService{}, nil
}

func (s *userProcessService) Ping(ctx context.Context, req *vnetv1.PingRequest) (*vnetv1.PingResponse, error) {
	if req.Version != api.Version {
		return nil, trace.BadParameter("version mismatch, user process version is %s, admin process version is %s",
			api.Version, req.Version)
	}
	return &vnetv1.PingResponse{
		Version: api.Version,
	}, nil
}

func (s *userProcessService) AuthenticateProcess(ctx context.Context, req *vnetv1.AuthenticateProcessRequest) (*vnetv1.AuthenticateProcessResponse, error) {
	log.DebugContext(ctx, "Received AuthenticateProcess request from admin process")
	pipePathPtr, err := syscall.UTF16PtrFromString(req.GetPipePath())
	if err != nil {
		return nil, trace.Wrap(err, "converting string to UTF16")
	}
	handle, err := windows.CreateFile(
		pipePathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return nil, trace.Wrap(err, "opening named pipe")
	}
	defer windows.CloseHandle(handle)
	log.DebugContext(ctx, "Connected to named pipe")
	return &vnetv1.AuthenticateProcessResponse{}, nil
}
