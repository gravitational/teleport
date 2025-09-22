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

package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/auth/authclient"
	multiplexergrpc "github.com/gravitational/teleport/lib/multiplexer/grpc"
	"github.com/gravitational/teleport/lib/relaytunnel"
	"github.com/gravitational/teleport/lib/srv"
)

func (process *TeleportProcess) initRelay() {
	process.RegisterWithAuthServer(apitypes.RoleRelay, RelayIdentityEvent)
	process.RegisterCriticalFunc("relay.run", process.runRelayService)
}

func (process *TeleportProcess) runRelayService() error {
	log := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentRelay, process.id))
	sublogger := func(subcomponent string) *slog.Logger {
		return process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentRelay, process.id, subcomponent))
	}

	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentRelay); err != nil {
			log.WarnContext(process.ExitContext(), "Failed closing imported file descriptors.", "error", err)
		}
	}()

	conn, err := process.WaitForConnector(RelayIdentityEvent, log)
	if conn == nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	accessPoint, err := process.newLocalCacheForRelay(conn.Client, []string{teleport.ComponentRelay})
	if err != nil {
		return err
	}

	tunnelServer, err := relaytunnel.NewServer(relaytunnel.ServerConfig{
		GetCertificate: func(ctx context.Context) (*tls.Certificate, error) {
			return conn.serverGetCertificate()
		},
		GetPool: func(ctx context.Context) (*x509.CertPool, error) {
			pool, _, err := authclient.ClientCertPool(ctx, accessPoint, conn.clusterName, apitypes.HostCA)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return pool, nil
		},
		Ciphersuites: process.Config.CipherSuites,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tunnelCreds := tunnelServer.GRPCServerCredentials()
	if process.Config.Relay.TunnelPROXYProtocol {
		tunnelCreds = multiplexergrpc.PPV2ServerCredentials{TransportCredentials: tunnelCreds}
	}
	tunnelGRPCServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			interceptors.GRPCServerUnaryErrorInterceptor,
		),
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
		),
		grpc.Creds(tunnelCreds),
	)
	defer tunnelGRPCServer.Stop()

	relaytunnelv1alpha.RegisterDiscoveryServiceServer(tunnelGRPCServer, &relaytunnel.StaticDiscoverServiceServer{
		RelayGroup:            process.Config.Relay.RelayGroup,
		TargetConnectionCount: process.Config.Relay.TargetConnectionCount,
	})

	tunnelListener, err := process.importOrCreateListener(ListenerRelayTunnel, process.Config.Relay.TunnelListenAddr.String())
	if err != nil {
		return trace.Wrap(err)
	}
	defer tunnelListener.Close()
	go tunnelGRPCServer.Serve(tunnelListener)

	nonce := uuid.NewString()
	var relayServer atomic.Pointer[presencev1.RelayServer]
	relayServer.Store(&presencev1.RelayServer{
		Kind:    apitypes.KindRelayServer,
		SubKind: "",
		Version: apitypes.V1,
		Metadata: &headerv1.Metadata{
			Name: conn.HostUUID(),
		},
		Spec: &presencev1.RelayServer_Spec{
			Hostname:   process.Config.Hostname,
			RelayGroup: process.Config.Relay.RelayGroup,
			Nonce:      nonce,
		},
	})

	hb, err := srv.NewRelayServerHeartbeat(srv.HeartbeatV2Config[*presencev1.RelayServer]{
		InventoryHandle: process.inventoryHandle,
		GetResource: func(context.Context) (*presencev1.RelayServer, error) {
			return relayServer.Load(), nil
		},

		// there's no fallback announce mode, the relay service only works with
		// clusters recent enough to support relay heartbeats through the ICS
		Announcer: nil,

		OnHeartbeat: process.OnHeartbeat(teleport.ComponentRelay),
	}, sublogger("heartbeat"))
	if err != nil {
		return trace.Wrap(err)
	}
	go hb.Run()
	defer hb.Close()

	if err := process.closeImportedDescriptors(teleport.ComponentRelay); err != nil {
		log.WarnContext(process.ExitContext(), "Failed closing imported file descriptors", "error", err)
	}

	process.BroadcastEvent(Event{Name: RelayReady})
	log.InfoContext(process.ExitContext(), "The relay service has successfully started", "nonce", nonce)

	_, _ = process.WaitForEvent(process.ExitContext(), TeleportTerminatingEvent)

	log.InfoContext(process.ExitContext(), "Process is beginning shutdown, advertising terminating status and waiting for shutdown")

	{
		r := proto.CloneOf(relayServer.Load())
		r.GetSpec().Terminating = true
		relayServer.Store(r)
	}

	exitEvent, _ := process.WaitForEvent(process.ExitContext(), TeleportExitEvent)
	ctx, _ := exitEvent.Payload.(context.Context)
	if ctx == nil {
		// if we're here it's because we got an ungraceful exit event or
		// WaitForEvent errored out because of the ungraceful shutdown; either
		// way, process.ExitContext() is a done context and all operations
		// should get canceled immediately
		ctx = process.ExitContext()
		log.InfoContext(ctx, "Stopping the relay service ungracefully")
	} else {
		log.InfoContext(ctx, "Stopping the relay service")
	}

	log.DebugContext(ctx, "Stopping servers")
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer context.AfterFunc(egCtx, tunnelGRPCServer.Stop)()
		tunnelGRPCServer.GracefulStop()
		return nil
	})
	warnOnErr(egCtx, eg.Wait(), log)

	warnOnErr(ctx, hb.Close(), log)
	warnOnErr(ctx, conn.Close(), log)

	log.InfoContext(ctx, "The relay service has stopped")

	return nil
}
