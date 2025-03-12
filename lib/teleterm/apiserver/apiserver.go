/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	vnetapi "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/apiserver/handler"
	"github.com/gravitational/teleport/lib/teleterm/vnet"
	"github.com/gravitational/teleport/lib/utils"
)

// New creates an instance of API Server
func New(cfg Config) (*APIServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create Terminal and VNet services.

	serviceHandler, err := handler.New(
		handler.Config{
			DaemonService: cfg.Daemon,
			Storage:       cfg.Storage,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vnetService, err := vnet.New(vnet.Config{
		DaemonService:      cfg.Daemon,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ClusterIDCache:     cfg.ClusterIDCache,
		InstallationID:     cfg.InstallationID,
		Clock:              cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the listener, set up the server.

	ls, err := newListener(cfg.HostAddr, cfg.ListeningC)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcServer := grpc.NewServer(cfg.TshdServerCreds,
		grpc.ChainUnaryInterceptor(withErrorHandling(cfg.Logger)),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)

	api.RegisterTerminalServiceServer(grpcServer, serviceHandler)
	vnetapi.RegisterVnetServiceServer(grpcServer, vnetService)

	return &APIServer{
		Config:      cfg,
		ls:          ls,
		grpcServer:  grpcServer,
		vnetService: vnetService,
	}, nil
}

// Serve starts accepting incoming connections
func (s *APIServer) Serve() error {
	return s.grpcServer.Serve(s.ls)
}

// Stop stops the server and closes all listeners
func (s *APIServer) Stop() {
	// Gracefully stopping the gRPC server takes a second or two. Closing the VNet service is almost
	// immediate. Closing the VNet service before the gRPC server gives some time for the VNet admin
	// process to notice that the client is gone and shut down as well.
	if err := s.vnetService.Close(); err != nil {
		slog.ErrorContext(context.Background(), "Error while closing VNet service", "error", err)
	}

	s.grpcServer.GracefulStop()
}

func newListener(hostAddr string, listeningC chan<- utils.NetAddr) (net.Listener, error) {
	uri, err := utils.ParseAddr(hostAddr)

	if err != nil {
		return nil, trace.BadParameter("invalid host address: %s", hostAddr)
	}

	lis, err := net.Listen(uri.Network(), uri.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr := utils.FromAddr(lis.Addr())
	sendBoundNetworkPortToStdout(addr)
	if listeningC != nil {
		listeningC <- addr
	}

	slog.InfoContext(context.Background(), "tsh daemon listener created", "listen_addr", addr.FullAddress())

	return lis, nil
}

func sendBoundNetworkPortToStdout(addr utils.NetAddr) {
	// Connect needs this message to know which port has been assigned to the server.
	fmt.Printf("{CONNECT_GRPC_PORT: %v}\n", addr.Port(1))
}

// Server is a combination of the underlying grpc.Server and its RuntimeOpts.
type APIServer struct {
	Config
	// ls is the server listener
	ls          net.Listener
	grpcServer  *grpc.Server
	vnetService *vnet.Service
}
