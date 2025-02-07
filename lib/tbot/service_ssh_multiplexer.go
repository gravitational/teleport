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
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

const (
	sshMuxSocketName = "v1.sock"
	agentSocketName  = "agent.sock"
)

var (
	muxReqsStartedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tbot_ssh_multiplexer_requests_started_total",
			Help: "Number of requests completed by the multiplexer",
		},
	)
	muxReqsHandledCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tbot_ssh_multiplexer_requests_handled_total",
			Help: "Number of requests completed by the multiplexer",
		}, []string{"status"},
	)
	muxReqsInflightGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tbot_ssh_multiplexer_requests_in_flight",
			Help: "Number of SSH connections currently being handled by the multiplexer",
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

	agentMu sync.Mutex
	agent   agent.ExtendedAgent
}

// writeIfChanged reads the artifact first to determine if it has changed
// before writing to it. This avoids truncating the file during another
// processes read.
//
// TODO(noah): Replace this with proper atomic writing.
// https://github.com/gravitational/teleport/issues/25462
func writeIfChanged(ctx context.Context, dest bot.Destination, log *slog.Logger, path string, data []byte) error {
	existingData, err := dest.Read(ctx, path)
	if err != nil {
		log.DebugContext(
			ctx,
			"Error occurred reading artifact for change-check, will write",
			"path", path,
			"error", err,
		)
		return dest.Write(ctx, path, data)
	}

	if bytes.Equal(existingData, data) {
		log.DebugContext(
			ctx,
			"Artifact unchanged, not writing",
			"path", path,
		)
		return nil
	}
	log.DebugContext(
		ctx,
		"Artifact changed, will write",
		"path", path,
	)

	return dest.Write(ctx, path, data)
}

func (s *SSHMultiplexerService) writeArtifacts(
	ctx context.Context,
	proxyHost string,
	authClient *authclient.Client,
) error {
	dest := s.cfg.Destination.(*config.DestinationDirectory)

	clusterNames, err := getClusterNames(ctx, authClient, s.identity.Get().ClusterName)
	if err != nil {
		return trace.Wrap(err, "fetching cluster names")
	}

	// Generate known hosts
	knownHosts, _, err := ssh.GenerateKnownHosts(
		ctx,
		s.botAuthClient,
		clusterNames,
		proxyHost,
	)
	if err != nil {
		return trace.Wrap(err, "generating known hosts")
	}
	if err := writeIfChanged(
		ctx, dest, s.log, ssh.KnownHostsName, []byte(knownHosts),
	); err != nil {
		return trace.Wrap(err, "writing %s", ssh.KnownHostsName)
	}

	// Generate SSH config
	proxyCommand := s.cfg.ProxyCommand
	if len(proxyCommand) == 0 {
		executablePath, err := os.Executable()
		if err != nil {
			return trace.Wrap(err, "determining executable path")
		}
		proxyCommand = []string{
			executablePath,
			"ssh-multiplexer-proxy-command",
		}
	}

	absPath, err := filepath.Abs(dest.Path)
	if err != nil {
		return trace.Wrap(err, "determining absolute path for destination")
	}

	var sshConfigBuilder strings.Builder
	err = openssh.WriteMuxedSSHConfig(&sshConfigBuilder, &openssh.MuxedSSHConfigParameters{
		AppName:         openssh.TbotApp,
		ClusterNames:    clusterNames,
		KnownHostsPath:  filepath.Join(absPath, ssh.KnownHostsName),
		ProxyCommand:    proxyCommand,
		MuxSocketPath:   filepath.Join(absPath, sshMuxSocketName),
		AgentSocketPath: filepath.Join(absPath, agentSocketName),
	})
	if err != nil {
		return trace.Wrap(err, "generating SSH config")
	}
	sshConfBytes := []byte(sshConfigBuilder.String())
	if err := writeIfChanged(
		ctx, dest, s.log, ssh.ConfigName, sshConfBytes,
	); err != nil {
		return trace.Wrap(err, "writing %s", ssh.ConfigName)
	}

	return nil
}

func (s *SSHMultiplexerService) setup(ctx context.Context) (
	_ *authclient.Client,
	_ *cyclingHostDialClient,
	proxyHost string,
	_ *libclient.TSHConfig,
	_ error,
) {
	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(
		muxReqsStartedCounter,
		muxReqsHandledCounter,
		muxReqsInflightGauge,
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
		for _, t := range tshConfig.ProxyTemplates {
			s.log.DebugContext(
				ctx,
				"Loaded proxy template",
				"template", t.Template,
				"proxy", t.Proxy,
				"host", t.Host,
				"cluster", t.Cluster,
				"query", t.Query,
				"search", t.Search,
			)
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
	proxyAddr, err := proxyPing.proxyWebAddr()
	if err != nil {
		return nil, nil, "", nil, trace.Wrap(err, "determining proxy web addr")
	}
	proxyHost, _, err = net.SplitHostPort(proxyAddr)
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
	proxyClient := newCyclingHostDialClient(100, proxyclient.ClientConfig{
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
			clientMetrics.UnaryClientInterceptor(),
			interceptors.GRPCClientUnaryErrorInterceptor,
		},
		StreamInterceptors: []grpc.StreamClientInterceptor{
			clientMetrics.StreamClientInterceptor(),
			interceptors.GRPCClientStreamErrorInterceptor,
		},
		SSHConfig:               sshConfig,
		InsecureSkipVerify:      s.botCfg.Insecure,
		ALPNConnUpgradeRequired: connUpgradeRequired,
	})

	authClient, err := clientForFacade(
		ctx, s.log, s.botCfg, s.identity, s.resolver,
	)
	if err != nil {
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

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateLifetime.TTL,
		nil,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating identity")
	}

	newAgent := agent.NewKeyring()
	err = newAgent.Add(agent.AddedKey{
		PrivateKey:   id.PrivateKey,
		Certificate:  id.SSHCert,
		LifetimeSecs: 0,
	})
	if err != nil {
		return nil, trace.Wrap(err, "adding identity to agent")
	}
	// There's a bug with Paramiko and older versions of OpenSSH that requires
	// that the bare key also be included in the agent or the key with the
	// certificate will not be used.
	// See the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
	err = newAgent.Add(agent.AddedKey{
		PrivateKey:   id.PrivateKey,
		Certificate:  nil,
		LifetimeSecs: 0,
	})
	if err != nil {
		return nil, trace.Wrap(err, "adding bare key to agent")
	}

	s.agentMu.Lock()
	s.agent = newAgent.(agent.ExtendedAgent)
	s.agentMu.Unlock()

	return id, nil
}

func (s *SSHMultiplexerService) identityRenewalLoop(
	ctx context.Context, proxyHost string, authClient *authclient.Client,
) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()
	err := runOnInterval(ctx, runOnIntervalConfig{
		name: "identity-renewal",
		f: func(ctx context.Context) error {
			id, err := s.generateIdentity(ctx)
			if err != nil {
				return trace.Wrap(err, "generating identity")
			}
			s.identity.Set(id)
			return s.writeArtifacts(ctx, proxyHost, authClient)
		},
		interval:   s.botCfg.CertificateLifetime.RenewalInterval,
		retryLimit: renewalRetryLimit,
		log:        s.log,
		reloadCh:   reloadCh,
	})
	return trace.Wrap(err)
}

func (s *SSHMultiplexerService) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/Run",
	)
	defer func() { tracing.EndSpan(span, err) }()

	authClient, hostDialer, proxyHost, tshConfig, err := s.setup(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer authClient.Close()

	dest := s.cfg.Destination.(*config.DestinationDirectory)
	absPath, err := filepath.Abs(dest.Path)
	if err != nil {
		return trace.Wrap(err, "determining absolute path for destination")
	}
	muxListenerAddr := url.URL{Scheme: "unix", Path: filepath.Join(absPath, sshMuxSocketName)}
	muxListener, err := createListener(
		ctx,
		s.log,
		muxListenerAddr.String(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer muxListener.Close()

	agentListenerAddr := url.URL{Scheme: "unix", Path: filepath.Join(absPath, agentSocketName)}
	agentListener, err := createListener(
		ctx,
		s.log,
		agentListenerAddr.String(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer agentListener.Close()

	eg, egCtx := errgroup.WithContext(ctx)
	// Handle main mux listener
	eg.Go(func() error {
		defer context.AfterFunc(egCtx, func() { _ = muxListener.Close() })()
		for {
			downstream, err := muxListener.Accept()
			if err != nil {
				if utils.IsUseOfClosedNetworkError(err) {
					return nil
				}

				s.log.WarnContext(
					egCtx,
					"Error encountered accepting mux connection, sleeping and continuing",
					"error",
					err,
				)
				select {
				case <-egCtx.Done():
					return nil
				case <-time.After(50 * time.Millisecond):
				}

				continue
			}

			go func() {
				muxReqsStartedCounter.Inc()
				muxReqsInflightGauge.Inc()
				defer muxReqsInflightGauge.Dec()

				err := s.handleConn(
					egCtx, tshConfig, authClient, hostDialer, proxyHost, downstream,
				)

				var status string
				switch {
				case err != nil && !errors.Is(err, context.Canceled):
					status = "ERROR"
					s.log.WarnContext(egCtx, "Mux handler exited with error", "error", err)
				default:
					status = "OK"
				}
				muxReqsHandledCounter.WithLabelValues(status).Inc()
			}()
		}
	})
	// Handle agent listener
	eg.Go(func() error {
		defer context.AfterFunc(egCtx, func() { _ = agentListener.Close() })()
		for {
			conn, err := agentListener.Accept()
			if err != nil {
				if utils.IsUseOfClosedNetworkError(err) {
					return nil
				}

				s.log.WarnContext(
					egCtx,
					"Error encountered accepting agent connection, sleeping and continuing",
					"error",
					err,
				)
				select {
				case <-egCtx.Done():
					return nil
				case <-time.After(50 * time.Millisecond):
				}

				continue
			}

			go func() {
				defer context.AfterFunc(egCtx, func() { _ = conn.Close() })()
				s.agentMu.Lock()
				currentAgent := s.agent
				s.agentMu.Unlock()

				s.log.DebugContext(egCtx, "Serving agent connection")
				//nolint:staticcheck // SA4023. ServeAgent always returns a non-nil error. This is fine.
				err := agent.ServeAgent(currentAgent, conn)
				if err != nil && !utils.IsOKNetworkError(err) {
					s.log.WarnContext(
						egCtx,
						"Error encountered serving agent connection",
						"error",
						err,
					)
				}
			}()

		}
	})
	// Handle identity renewal
	eg.Go(func() error {
		return s.identityRenewalLoop(egCtx, proxyHost, authClient)
	})

	return eg.Wait()
}

func (s *SSHMultiplexerService) handleConn(
	ctx context.Context,
	tshConfig *libclient.TSHConfig,
	authClient *authclient.Client,
	hostDialer *cyclingHostDialClient,
	proxyHost string,
	downstream net.Conn,
) (err error) {
	ctx, span := tracer.Start(
		ctx,
		"SSHMultiplexerService/handleConn",
		oteltrace.WithNewRoot(),
	)
	defer func() { tracing.EndSpan(span, err) }()
	defer func() {
		if err := downstream.Close(); err != nil && !utils.IsOKNetworkError(err) {
			s.log.DebugContext(ctx, "Error closing downstream connection", "error", err)
		}
	}()

	var stderr *os.File
	defer func() {
		if stderr == nil {
			return
		}
		defer stderr.Close()
		if err != nil {
			fmt.Fprintln(stderr, err)
		}
	}()

	var req string
	// here we try receiving a file descriptor to use for error output, which
	// should be mapped to OpenSSH's own stderr (or /dev/null)
	if un, _ := downstream.(*net.UnixConn); un != nil {
		b := make([]byte, 1)
		fds := make([]*os.File, 1)

		n, fdn, err := uds.ReadWithFDs(un, b, fds)
		if err != nil {
			return trace.Wrap(err, "reading request")
		}
		if fdn > 0 {
			s.log.DebugContext(ctx, "Received stderr file descriptor from client for error reporting")
			stderr = fds[0]
		}
		// this approach works because we know that req must be at least one
		// byte at the end (as it must end with a NUL)
		req = string(b[:n])
	}

	// The first thing downstream will send is the multiplexing request which is
	// in the "[host]:[port]|[cluster_name]\x00" format.
	// The "|[cluster_name]" section is optional and if omitted, the cluster
	// associated with the bot will be used.
	//
	// We choose this format because | is not an acceptable character in
	// hostnames or ports through OpenSSH.
	// https://github.com/openssh/openssh-portable/commit/7ef3787c84b6b524501211b11a26c742f829af1a
	buf := bufio.NewReader(downstream)
	if !strings.HasSuffix(req, "\x00") {
		r, err := buf.ReadString('\x00')
		if err != nil {
			return trace.Wrap(err, "reading request")
		}
		req += r
	}
	req = req[:len(req)-1] // Drop the NUL.

	// Split by | to pull out the optionally specified cluster name.
	// TODO(noah): When we need to add another parameter in future, we should
	// roll this API to v2 and use a more extensible format.
	splitReq := strings.Split(req, "|")
	if len(splitReq) > 2 {
		return trace.BadParameter(
			"malformed request, expected at most 2 fields, got %d: %q",
			len(splitReq), req,
		)
	}

	host, port, err := utils.SplitHostPort(splitReq[0])
	if err != nil {
		return trace.Wrap(err, "malformed request %q", req)
	}

	clusterName := s.identity.Get().ClusterName
	if len(splitReq) > 1 {
		clusterName = splitReq[1]
	}

	log := s.log.With(
		slog.Group("req",
			"host", host,
			"port", port,
			"cluster_name", clusterName,
		),
	)
	log.InfoContext(ctx, "Received multiplexing request")

	expanded, matched := tshConfig.ProxyTemplates.Apply(
		net.JoinHostPort(host, port),
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
		node, err := resolveTargetHostWithClient(ctx, authClient.APIClient, expanded.Search, expanded.Query)
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

	// We need the agent to support proxy recording mode.
	s.agentMu.Lock()
	currentAgent := s.agent
	s.agentMu.Unlock()

	upstream, _, err := hostDialer.DialHost(ctx, target, clusterName, currentAgent)
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
				// we didn't need the agent in the first place as proxy
				// recording mode does not require it.
				var noAgent agent.ExtendedAgent
				conn, _, err := hostDialer.DialHost(
					ctx, net.JoinHostPort(hostID, "0"), clusterName, noAgent,
				)
				return conn, err
			})
		if err != nil {
			return trace.Wrap(err, "wrapping conn for session resumption")
		}
	}
	defer func() {
		if err := upstream.Close(); err != nil && !utils.IsOKNetworkError(err) {
			s.log.DebugContext(ctx, "Error closing upstream connection", "error", err)
		}
	}()

	defer context.AfterFunc(ctx, func() {
		if err := trace.NewAggregate(
			upstream.Close(),
			downstream.Close(),
		); err != nil {
			s.log.DebugContext(ctx, "Error closing connections", "error", err)
		}
	})()

	log.InfoContext(ctx, "Proxying connection for multiplexing request")
	startedProxying := time.Now()

	// once the connection is actually started we should stop writing to the
	// client's stderr
	if stderr != nil {
		_ = stderr.Close()
		stderr = nil
	}

	errCh := make(chan error, 2)
	go func() {
		defer upstream.Close()
		defer downstream.Close()
		// Drain the buffer we used to read in the request in case it read in more
		// than just the initial request.
		drained, _ := buf.Peek(buf.Buffered())
		if _, err := upstream.Write(drained); err != nil {
			errCh <- trace.Wrap(err, "draining request buffer upstream")
			return
		}
		_, err := io.Copy(upstream, downstream)
		if utils.IsOKNetworkError(err) {
			err = nil
		}
		errCh <- trace.Wrap(err, "downstream->upstream")
	}()

	go func() {
		defer upstream.Close()
		defer downstream.Close()
		_, err := io.Copy(downstream, upstream)
		if utils.IsOKNetworkError(err) {
			err = nil
		}
		errCh <- trace.Wrap(err, "upstream->downstream")
	}()

	err = trace.NewAggregate(<-errCh, <-errCh)
	log.InfoContext(
		ctx,
		"Finished proxying connection multiplexing request",
		"proxied_duration", time.Since(startedProxying),
	)
	return err
}

func (s *SSHMultiplexerService) String() string {
	return config.SSHMultiplexerServiceType
}

type hostDialer interface {
	DialHost(ctx context.Context, target string, cluster string, keyring agent.ExtendedAgent) (net.Conn, proxyclient.ClusterDetails, error)
	Close() error
}

// cyclingHostDialClient handles cycling through proxy clients every configured
// number of connections. This prevents a single client being overwhelmed.
type cyclingHostDialClient struct {
	max          int32
	hostDialerFn func(ctx context.Context) (hostDialer, error)

	mu         sync.Mutex
	started    int32
	currentClt *refCountProxyClient
}

type refCountProxyClient struct {
	clt      hostDialer
	refCount atomic.Int32
}

func (r *refCountProxyClient) release() {
	if r == nil {
		return
	}
	if r.refCount.Add(-1) <= 0 {
		go r.clt.Close()
	}
}

type refCountConn struct {
	net.Conn
	parent atomic.Pointer[refCountProxyClient]
}

func (r *refCountConn) Close() error {
	// Swap operation ensures that this conn only releases the ref to its
	// underlying client once, even if Close is called multiple times.
	defer r.parent.Swap(nil).release()
	return trace.Wrap(r.Conn.Close())
}

func newCyclingHostDialClient(max int32, config proxyclient.ClientConfig) *cyclingHostDialClient {
	return &cyclingHostDialClient{
		max: max,
		hostDialerFn: func(ctx context.Context) (hostDialer, error) {
			return proxyclient.NewClient(ctx, config)
		},
	}
}

func (s *cyclingHostDialClient) DialHost(ctx context.Context, target string, cluster string, keyring agent.ExtendedAgent) (net.Conn, proxyclient.ClusterDetails, error) {
	s.mu.Lock()
	if s.currentClt == nil {
		clt, err := s.hostDialerFn(ctx)
		if err != nil {
			s.mu.Unlock()
			return nil, proxyclient.ClusterDetails{}, trace.Wrap(err)
		}
		s.currentClt = &refCountProxyClient{clt: clt}
		// cyclingHostDialClient holds a reference while the refCountProxyClient
		// is "live"
		s.currentClt.refCount.Add(1)
		s.started = 0
	}

	currentClt := s.currentClt
	s.started++
	if s.started >= s.max {
		// the reference owned by cyclingHostDialClient is transferred to currentClt
		s.currentClt = nil
	} else {
		currentClt.refCount.Add(1)
	}
	s.mu.Unlock()

	innerConn, details, err := currentClt.clt.DialHost(ctx, target, cluster, keyring)
	if err != nil {
		currentClt.release()
		return nil, details, trace.Wrap(err)
	}

	wrappedConn := &refCountConn{
		Conn: innerConn,
	}
	wrappedConn.parent.Store(currentClt)
	return wrappedConn, details, nil
}
