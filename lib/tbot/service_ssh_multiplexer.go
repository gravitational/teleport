/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"path"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"

	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	connectionsHandledCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tbot_ssh_multiplexer_connections_total",
			Help: "Number of SSH connections proxied",
		}, []string{"status"},
	)
	inflightConnectionsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tbot_ssh_multiplexer_connections_in_flight",
			Help: "Number of SSH connections currently being proxied",
		},
	)
)

// SSHMultiplexerService is a long-lived local SSH proxy. It listens on a local Unix
// socket and has a special client with support for FDPassing with OpenSSH.
// It places an emphasis on high performance.
type SSHMultiplexerService struct {
	cfg              *config.SSHMultiplexerService
	botCfg           *config.BotConfig
	svcIdentity      *config.UnstableClientCredentialOutput
	log              *slog.Logger
	proxyPingCache   *proxyPingCache
	alpnUpgradeCache *alpnProxyConnUpgradeRequiredCache
	resolver         reversetunnelclient.Resolver

	// Fields below here are initialized by the service itself on startup.
	authClient  *authclient.Client
	proxyClient *proxyclient.Client
	tshConfig   *libclient.TSHConfig
	proxyHost   string
	clusterName string
}

func (s *SSHMultiplexerService) setup(ctx context.Context) (func(), error) {
	nopClose := func() {}

	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(
		connectionsHandledCounter,
		inflightConnectionsGauge,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load in any proxy templates if a path to them has been provided.
	tshConfig := &libclient.TSHConfig{}
	if s.cfg.ProxyTemplatesPath != "" {
		s.log.Info("Loading proxy templates", "path", s.cfg.ProxyTemplatesPath)
		var err error
		tshConfig, err = libclient.LoadTSHConfig(s.cfg.ProxyTemplatesPath)
		if err != nil {
			return nil, trace.Wrap(err, "loading proxy templates")
		}
	}
	s.tshConfig = tshConfig

	// Wait for the output service to provide us with an impersonated client
	// credential.
	select {
	case <-ctx.Done():
		return nopClose, ctx.Err()
	case <-time.After(10 * time.Second):
		return nopClose, trace.BadParameter("timeout waiting for identity to be ready")
	case <-s.svcIdentity.Ready():
	}
	facade, err := s.svcIdentity.Facade()
	if err != nil {
		return nopClose, trace.Wrap(err)
	}
	sshConfig, err := facade.SSHClientConfig()
	if err != nil {
		return nopClose, trace.Wrap(err)
	}

	// Ping the proxy and determine if we need to upgrade the connection.
	proxyPing, err := s.proxyPingCache.ping(ctx)
	if err != nil {
		return nopClose, trace.Wrap(err)
	}
	proxyAddr := proxyPing.Proxy.SSH.PublicAddr
	proxyHost, _, err := net.SplitHostPort(proxyPing.Proxy.SSH.PublicAddr)
	if err != nil {
		return nopClose, trace.Wrap(err)
	}
	s.proxyHost = proxyHost
	s.clusterName = proxyPing.ClusterName
	connUpgradeRequired := false
	if proxyPing.Proxy.TLSRoutingEnabled {
		connUpgradeRequired, err = s.alpnUpgradeCache.isUpgradeRequired(
			ctx, proxyAddr, s.botCfg.Insecure,
		)
		if err != nil {
			return nopClose, trace.Wrap(err, "determining if ALPN upgrade is required")
		}
	}

	// Create Proxy and Auth clients
	proxyClient, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      proxyAddr,
		TLSRoutingEnabled: proxyPing.Proxy.TLSRoutingEnabled,
		TLSConfigFunc: func(cluster string) (*tls.Config, error) {
			cfg, err := facade.TLSConfig()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// The facade TLS config is tailored toward connections to the Auth service.
			// Override the server name to be the proxy and blank out the next protos to
			// avoid hitting the proxy web listener.
			cfg.ServerName = proxyHost
			cfg.NextProtos = nil
			return cfg, nil
		},
		UnaryInterceptors: []grpc.UnaryClientInterceptor{
			interceptors.GRPCClientUnaryErrorInterceptor,
		},
		StreamInterceptors: []grpc.StreamClientInterceptor{
			interceptors.GRPCClientStreamErrorInterceptor,
		},
		SSHConfig:               sshConfig,
		InsecureSkipVerify:      s.botCfg.Insecure,
		ALPNConnUpgradeRequired: connUpgradeRequired,

		// Here we use a special dial context that will create a new connection
		// after the cycleCount has been reached. This prevents too many SSH
		// connections from sharing the same upstream connection.
		DialContext: newDialCycling(100),
	})
	if err != nil {
		return nopClose, trace.Wrap(err)
	}
	s.proxyClient = proxyClient

	authClient, err := clientForFacade(
		ctx, s.log, s.botCfg, facade, s.resolver,
	)
	if err != nil {
		_ = proxyClient.Close()
		return nopClose, trace.Wrap(err)
	}
	s.authClient = authClient

	return func() {
		_ = authClient.Close()
		_ = proxyClient.Close()
	}, nil
}

func (s *SSHMultiplexerService) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/Run",
	)
	defer func() { tracing.EndSpan(span, err) }()

	closer, err := s.setup(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closer()

	dest := s.cfg.Destination.(*config.DestinationDirectory)
	l, err := createListener(
		ctx,
		s.log,
		fmt.Sprintf("unix://%s", path.Join(dest.Path, "tbot_ssh_multiplexer.v1.sock")))
	if err != nil {
		return trace.Wrap(err)
	}

	defer context.AfterFunc(ctx, func() { _ = l.Close() })()
	for {
		downstream, err := l.Accept()
		if err != nil {
			if utils.IsUseOfClosedNetworkError(err) {
				return nil
			}

			s.log.WarnContext(ctx, "Error encountered accepting connection, sleeping and continuing", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(50 * time.Millisecond):
			}

			continue
		}

		go func() {
			inflightConnectionsGauge.Inc()
			err := s.handleConn(ctx, downstream)
			inflightConnectionsGauge.Dec()
			status := "OK"
			if err != nil {
				status = "ERROR"
				s.log.WarnContext(ctx, "Handler exited", "error", err)
			}
			connectionsHandledCounter.WithLabelValues(status).Inc()
		}()
	}
}

func (s *SSHMultiplexerService) handleConn(
	ctx context.Context,
	downstream net.Conn,
) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/handleConn",
		oteltrace.WithNewRoot(),
	)
	defer func() { tracing.EndSpan(span, err) }()
	defer downstream.Close()

	// The first thing downstream will send is the hostport of the target,
	// followed by a newline.
	buf := bufio.NewReader(downstream)
	hostPort, err := buf.ReadString('\n')
	if err != nil {
		return trace.Wrap(err, "reading hostport from downstream")
	}
	hostPort = hostPort[:len(hostPort)-1] // Strip off \n

	s.log.Info("Handling new connection", "host_port", hostPort)
	defer s.log.Info("Finished handling connection", "host_port", hostPort)

	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return trace.Wrap(err, "splitting host and port")
	}

	clusterName := s.clusterName
	expanded, matched := s.tshConfig.ProxyTemplates.Apply(hostPort)
	if matched {
		s.log.DebugContext(
			ctx,
			"Proxy templated matched",
			"populated_template", expanded,
		)
		if expanded.Cluster != "" {
			clusterName = expanded.Cluster
		}

		if expanded.Host != "" {
			host = expanded.Host
		}
	}

	var target string
	if expanded == nil || (len(expanded.Search) == 0 && expanded.Query == "") {
		host = cleanTargetHost(host, s.proxyHost, clusterName)
		target = net.JoinHostPort(host, port)
	} else {
		node, err := resolveTargetHostWithClient(ctx, s.authClient, expanded.Search, expanded.Query)
		if err != nil {
			return trace.Wrap(err, "resolving target host")
		}

		s.log.DebugContext(
			ctx,
			"Found matching SSH host",
			"host_uuid", node.GetName(),
			"host_name", node.GetHostname(),
		)

		target = net.JoinHostPort(node.GetName(), "0")
	}

	upstream, _, err := s.proxyClient.DialHost(ctx, target, clusterName, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if s.cfg.SessionResumptionEnabled() {
		s.log.DebugContext(ctx, "Enabling session resumption")
		upstream, err = resumption.WrapSSHClientConn(
			ctx,
			upstream,
			func(ctx context.Context, hostID string) (net.Conn, error) {
				s.log.DebugContext(ctx, "Resuming connection")
				// if the connection is being resumed, it means that
				// we didn't need the agent in the first place
				var noAgent agent.ExtendedAgent
				conn, _, err := s.proxyClient.DialHost(
					ctx, net.JoinHostPort(hostID, "0"), clusterName, noAgent,
				)
				return conn, err
			})
		if err != nil {
			return trace.Wrap(err, "wrapping conn for session resumption")
		}
	}
	// We don't need to defer close upstream here as this is handled by
	// ProxyConn

	return trace.Wrap(utils.ProxyConn(ctx, downstream, upstream), "proxying connection")
}

func (s *SSHMultiplexerService) String() string {
	return config.SSHMultiplexerServiceType
}
