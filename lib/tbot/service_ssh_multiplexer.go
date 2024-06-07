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
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	sshMuxSocketName = "tbot_ssh_multiplexer.v1.sock"
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
	alpnUpgradeCache *alpnProxyConnUpgradeRequiredCache
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient     *authclient.Client
	botCfg            *config.BotConfig
	cfg               *config.SSHMultiplexerService
	getBotIdentity    getBotIdentityFn
	log               *slog.Logger
	proxyPingCache    *proxyPingCache
	reloadBroadcaster *channelBroadcaster
	resolver          reversetunnelclient.Resolver

	// Fields below here are initialized by the service itself on startup.
	identity *identity.Facade
}

func (s *SSHMultiplexerService) writeArtifacts(ctx context.Context, proxyHost string, id *identity.Identity) error {
	dest := s.cfg.Destination.(*config.DestinationDirectory)

	// TODO(noah): identity.SaveIdentity outputs artifacts we don't necessarily
	// want. For now, I've just manually output them here but we may want to
	// revisit how this is implemented.
	if err := dest.Write(ctx, identity.SSHCertKey, id.CertBytes); err != nil {
		return trace.Wrap(err, "writing %s", identity.SSHCertKey)
	}
	if err := dest.Write(ctx, identity.PrivateKeyKey, id.PrivateKeyBytes); err != nil {
		return trace.Wrap(err, "writing %s", identity.PrivateKeyKey)
	}
	if err := dest.Write(ctx, identity.PublicKeyKey, id.PublicKeyBytes); err != nil {
		return trace.Wrap(err, "writing %s", identity.PublicKeyKey)
	}

	// Generate known hosts
	knownHosts, err := ssh.GenerateKnownHosts(
		ctx,
		s.botAuthClient,
		[]string{id.ClusterName},
		proxyHost,
	)
	if err != nil {
		return trace.Wrap(err, "generating known hosts")
	}
	if err := dest.Write(ctx, ssh.KnownHostsName, []byte(knownHosts)); err != nil {
		return trace.Wrap(err, "writing %s", ssh.KnownHostsName)
	}

	// Generate SSH config
	executablePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "determining executable path")
	}
	muxCommand := executablePath
	muxSubcommand := "ssh-multiplexer-proxy-command"
	if s.cfg.CustomMuxCommand != "" {
		muxCommand = s.cfg.CustomMuxCommand
		muxSubcommand = s.cfg.CustomMuxSubcommand
	}

	var sshConfigBuilder strings.Builder
	sshConf := openssh.NewSSHConfig(openssh.GetSystemSSHVersion, nil)
	err = sshConf.GetSSHConfig(&sshConfigBuilder, &openssh.SSHConfigParameters{
		AppName:             openssh.TbotApp,
		ClusterNames:        []string{id.ClusterName},
		KnownHostsPath:      path.Join(dest.Path, ssh.KnownHostsName),
		IdentityFilePath:    path.Join(dest.Path, identity.PrivateKeyKey),
		CertificateFilePath: path.Join(dest.Path, identity.SSHCertKey),
		ProxyHost:           proxyHost,

		TBotMux:           true,
		TBotMuxCommand:    muxCommand,
		TBotMuxSubcommand: muxSubcommand,
		TBotMuxData:       `{"host":"%h","port":"%p"}`,
		TBotMuxSocketPath: path.Join(dest.Path, sshMuxSocketName),
		ExecutablePath:    executablePath,
	})
	if err != nil {
		return trace.Wrap(err, "generating SSH config")

	}
	if err := dest.Write(ctx, ssh.ConfigName, []byte(sshConfigBuilder.String())); err != nil {
		return trace.Wrap(err, "writing %s", ssh.ConfigName)
	}

	return nil
}

func (s *SSHMultiplexerService) setup(ctx context.Context) (
	_ *authclient.Client,
	_ *proxyclient.Client,
	proxyHost string,
	_ *libclient.TSHConfig,
	_ error,
) {
	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(
		connectionsHandledCounter,
		inflightConnectionsGauge,
	); err != nil {
		return nil, nil, "", nil, trace.Wrap(err)
	}

	if err := s.cfg.Destination.Init(ctx, []string{}); err != nil {
		return nil, nil, "", nil, trace.Wrap(err, "initializing destination")
	}

	// Load in any proxy templates if a path to them has been provided.
	tshConfig := &libclient.TSHConfig{}
	if s.cfg.ProxyTemplatesPath != "" {
		s.log.InfoContext(ctx, "Loading proxy templates", "path", s.cfg.ProxyTemplatesPath)
		var err error
		tshConfig, err = libclient.LoadTSHConfig(s.cfg.ProxyTemplatesPath)
		if err != nil {
			return nil, nil, "", nil, trace.Wrap(err, "loading proxy templates")
		}
	}

	// Generate our initial identity and write the artifacts to the destination.
	id, err := s.generateIdentity(ctx)
	if err != nil {
		return nil, nil, "", nil, trace.Wrap(err, "generating initial identity")
	}
	s.identity = identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)

	sshConfig, err := s.identity.SSHClientConfig()
	if err != nil {
		return nil, nil, "", nil, trace.Wrap(err)
	}

	// Ping the proxy and determine if we need to upgrade the connection.
	proxyPing, err := s.proxyPingCache.ping(ctx)
	if err != nil {
		return nil, nil, "", nil, trace.Wrap(err)
	}
	proxyAddr := proxyPing.Proxy.SSH.PublicAddr
	proxyHost, _, err = net.SplitHostPort(proxyPing.Proxy.SSH.PublicAddr)
	if err != nil {
		return nil, nil, "", nil, trace.Wrap(err)
	}
	connUpgradeRequired := false
	if proxyPing.Proxy.TLSRoutingEnabled {
		connUpgradeRequired, err = s.alpnUpgradeCache.isUpgradeRequired(
			ctx, proxyAddr, s.botCfg.Insecure,
		)
		if err != nil {
			return nil, nil, "", nil, trace.Wrap(err, "determining if ALPN upgrade is required")
		}
	}

	// Create Proxy and Auth clients
	proxyClient, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      proxyAddr,
		TLSRoutingEnabled: proxyPing.Proxy.TLSRoutingEnabled,
		TLSConfigFunc: func(cluster string) (*tls.Config, error) {
			cfg, err := s.identity.TLSConfig()
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
		return nil, nil, "", nil, trace.Wrap(err)
	}

	authClient, err := clientForFacade(
		ctx, s.log, s.botCfg, s.identity, s.resolver,
	)
	if err != nil {
		_ = proxyClient.Close()
		return nil, nil, "", nil, trace.Wrap(err)
	}

	return authClient, proxyClient, proxyHost, tshConfig, nil
}

// generateIdentity generates our impersonated identity which we will write to
// the destination.
func (s *SSHMultiplexerService) generateIdentity(ctx context.Context) (*identity.Identity, error) {
	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return nil, trace.Wrap(err, "fetching default roles")
	}

	ident, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateTTL,
		nil,
	)
	return ident, err
}

func (s *SSHMultiplexerService) identityRenewalLoop(ctx context.Context, proxyHost string) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	ticker := time.NewTicker(s.botCfg.RenewalInterval)
	jitter := retryutils.NewJitter()
	defer ticker.Stop()
	for {
		var err error
		for attempt := 1; attempt <= renewalRetryLimit; attempt++ {
			s.log.InfoContext(
				ctx,
				"Attempting to renew identity",
				"attempt", attempt,
				"retry_limit", renewalRetryLimit,
			)
			var id *identity.Identity
			id, err = s.generateIdentity(ctx)
			if err == nil {
				s.identity.Set(id)
				err = s.writeArtifacts(ctx, proxyHost, id)
				if err == nil {
					break
				}
			}

			if attempt != renewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				s.log.WarnContext(
					ctx,
					"Identity renewal attempt failed. Waiting to retry",
					"attempt", attempt,
					"retry_limit", renewalRetryLimit,
					"backoff", backoffTime,
					"error", err,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoffTime):
				}
			}
		}
		if err != nil {
			s.log.WarnContext(
				ctx,
				"All retry attempts exhausted renewing identity. Waiting for next normal renewal cycle",
				"retry_limit", renewalRetryLimit,
				"interval", s.botCfg.RenewalInterval,
			)
		} else {
			s.log.InfoContext(
				ctx,
				"Renewed identity. Waiting for next identity renewal",
				"interval", s.botCfg.RenewalInterval,
			)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			continue
		case <-reloadCh:
			continue
		}
	}
}

func (s *SSHMultiplexerService) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/Run",
	)
	defer func() { tracing.EndSpan(span, err) }()

	authClient, proxyClient, proxyHost, tshConfig, err := s.setup(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer authClient.Close()
	defer proxyClient.Close()

	dest := s.cfg.Destination.(*config.DestinationDirectory)
	l, err := createListener(
		ctx,
		s.log,
		fmt.Sprintf("unix://%s", path.Join(dest.Path, sshMuxSocketName)))
	if err != nil {
		return trace.Wrap(err)
	}
	defer l.Close()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer context.AfterFunc(egCtx, func() { _ = l.Close() })()
		for {
			downstream, err := l.Accept()
			if err != nil {
				if utils.IsUseOfClosedNetworkError(err) {
					return nil
				}

				s.log.WarnContext(
					egCtx,
					"Error encountered accepting connection, sleeping and continuing",
					"error",
					err,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(50 * time.Millisecond):
				}

				continue
			}

			go func() {
				inflightConnectionsGauge.Inc()
				err := s.handleConn(
					egCtx, tshConfig, authClient, proxyClient, proxyHost, downstream,
				)
				inflightConnectionsGauge.Dec()
				status := "OK"
				if err != nil {
					status = "ERROR"
					s.log.WarnContext(egCtx, "Handler exited", "error", err)
				}
				connectionsHandledCounter.WithLabelValues(status).Inc()
			}()
		}
	})
	eg.Go(func() error {
		return s.identityRenewalLoop(egCtx, proxyHost)
	})

	return eg.Wait()
}

type multiplexingRequest struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func (m multiplexingRequest) Validate() error {
	switch {
	case m.Host == "":
		return trace.BadParameter("host: must be specified")
	case m.Port == "":
		return trace.BadParameter("port: must be specified")
	}
	return nil
}

func (s *SSHMultiplexerService) handleConn(
	ctx context.Context,
	tshConfig *libclient.TSHConfig,
	authClient *authclient.Client,
	proxyClient *proxyclient.Client,
	proxyHost string,
	downstream net.Conn,
) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/handleConn",
		oteltrace.WithNewRoot(),
	)
	defer func() { tracing.EndSpan(span, err) }()
	defer downstream.Close()

	// The first thing downstream will send is the multiplexingRequest JSON.
	buf := bufio.NewReader(downstream)
	reqBytes, err := buf.ReadBytes('\n')
	if err != nil {
		return trace.Wrap(err, "reading request from downstream")
	}
	req := multiplexingRequest{}
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return trace.Wrap(err, "unmarshaling request %q", reqBytes)
	}

	log := s.log.With(
		slog.Group("req",
			"host", req.Host,
			"port", req.Port,
		),
	)
	log.InfoContext(ctx, "Received multiplexing request")

	if err := req.Validate(); err != nil {
		return trace.Wrap(err, "validating multiplexing request")
	}
	host := req.Host
	port := req.Port

	clusterName := s.identity.Get().ClusterName
	expanded, matched := tshConfig.ProxyTemplates.Apply(
		net.JoinHostPort(req.Host, req.Port),
	)
	if matched {
		log.DebugContext(
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
		host = cleanTargetHost(host, proxyHost, clusterName)
		target = net.JoinHostPort(host, port)
	} else {
		node, err := resolveTargetHostWithClient(ctx, authClient, expanded.Search, expanded.Query)
		if err != nil {
			return trace.Wrap(err, "resolving target host")
		}

		log.DebugContext(
			ctx,
			"Found matching SSH host",
			"host_uuid", node.GetName(),
			"host_name", node.GetHostname(),
		)

		target = net.JoinHostPort(node.GetName(), "0")
	}

	upstream, _, err := proxyClient.DialHost(ctx, target, clusterName, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if s.cfg.SessionResumptionEnabled() {
		log.DebugContext(ctx, "Enabling session resumption")
		upstream, err = resumption.WrapSSHClientConn(
			ctx,
			upstream,
			func(ctx context.Context, hostID string) (net.Conn, error) {
				log.DebugContext(ctx, "Resuming connection")
				// if the connection is being resumed, it means that
				// we didn't need the agent in the first place
				var noAgent agent.ExtendedAgent
				conn, _, err := proxyClient.DialHost(
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

	log.InfoContext(ctx, "Proxying connection for multiplexing request")
	startedProxying := time.Now()
	err = utils.ProxyConn(ctx, downstream, upstream)
	log.InfoContext(
		ctx,
		"Finished proxying connection multiplexing request",
		"proxied_duration", time.Since(startedProxying),
	)
	if err != nil {
		return trace.Wrap(err, "proxying connection")
	}
	return nil
}

func (s *SSHMultiplexerService) String() string {
	return config.SSHMultiplexerServiceType
}

// cyclingHostDialClient
type cyclingHostDialClient struct {
	max    int
	config proxyclient.ClientConfig
	inner  *proxyclient.Client
}

func newCyclingHostDialClient(max int32, config *proxyclient.ClientConfig) *cyclingHostDialClient {
	return &cyclingHostDialClient{max: max, config: config}
}

func (s *cyclingHostDialClient) DialHost(ctx context.Context, target string, cluster string, keyring agent.ExtendedAgent) (net.Conn, proxyclient.ClusterDetails, error) {
	return s.inner.DialHost(ctx, target, cluster, keyring)
}
