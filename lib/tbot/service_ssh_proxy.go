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
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"

	apiclient "github.com/gravitational/teleport/api/client"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

// SSHProxyService
type SSHProxyService struct {
	cfg         *config.SSHProxyService
	svcIdentity *config.UnstableClientCredentialOutput
	log         *slog.Logger

	apiClient   *authclient.Client
	proxyClient *proxyclient.Client
	tshConfig   *libclient.TSHConfig
}

func (s *SSHProxyService) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return trace.BadParameter("timeout waiting for identity to be ready")
	case <-s.svcIdentity.Ready():
	}
	facade, err := s.svcIdentity.Facade()
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, _, err := net.SplitHostPort(proxy)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      proxy,
		TLSRoutingEnabled: s.cfg.TLSRoutingEnabled,
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
		InsecureSkipVerify:      s.cfg.Insecure,
		ALPNConnUpgradeRequired: cfg.ConnectionUpgradeRequired,

		DialContext: dialCycling,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	authConfig, err := proxyClient.ClientConfig(ctx, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	authConfig.Credentials = []apiclient.Credentials{facade}

	dest := s.cfg.Destination.(*config.DestinationDirectory)
	l, err := createListener(ctx, s.log, fmt.Sprintf("unix://%s", dest.Path))
	if err != nil {
		return trace.Wrap(err)
	}

	defer context.AfterFunc(ctx, func() { _ = l.Close() })()
	for {
		downstream, err := l.Accept()
		if err != nil {
			s.log.WarnContext(ctx, "Accept error, sleeping and continuing", "error", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		go func() {
			err := s.handleConn(ctx, downstream)
			if err != nil {
				s.log.WarnContext(ctx, "Handler exited", "error", err)
			}
		}()
	}
}

func (s *SSHProxyService) handleConn(
	ctx context.Context,
	downstream net.Conn,
) (err error) {
	ctx, span := tracer.Start(ctx, "SPIFFEWorkloadAPIService/handleConn")
	defer tracing.EndSpan(span, err)
	defer downstream.Close()

	buf := bufio.NewReader(downstream)
	hostPort, err := buf.ReadString('\n')
	if err != nil {
		return trace.Wrap(err)
	}
	hostPort = hostPort[:len(hostPort)-1]

	s.log.Info("handling new connection", "host_port", hostPort)
	defer s.log.Info("finished handling connection", "host_port", hostPort)

	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return trace.Wrap(err)
	}

	expanded, matched := s.tshConfig.ProxyTemplates.Apply(hostPort)
	if matched {
		s.log.DebugContext(
			ctx,
			"proxy templated matched",
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
		node, err := resolveTargetHostWithClient(ctx, apiClient, expanded.Search, expanded.Query)
		if err != nil {
			return trace.Wrap(err)
		}

		cfg.Log.DebugContext(ctx, "found matching SSH host", "host_uuid", node.GetName(), "host_name", node.GetHostname())

		target = net.JoinHostPort(node.GetName(), "0")
	}

	upstream, _, err := s.proxyClient.DialHost(ctx, target, clusterName, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if cfg.EnableResumption {
		nodeConn, err = resumption.WrapSSHClientConn(ctx, nodeConn, func(ctx context.Context, hostID string) (net.Conn, error) {
			// if the connection is being resumed, it means that
			// we didn't need the agent in the first place
			var noAgent agent.ExtendedAgent
			conn, _, err := proxyClient.DialHost(ctx, net.JoinHostPort(hostID, "0"), clusterName, noAgent)
			return conn, err
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer upstream.Close()

	defer context.AfterFunc(ctx, func() {
		_ = upstream.Close()
		_ = downstream.Close()
	})()

	errC := make(chan error, 2)
	go func() {
		defer upstream.Close()
		defer downstream.Close()
		_, err := io.CopyN(upstream, buf, int64(buf.Buffered()))
		if err != nil {
			errC <- err
			return
		}
		_, err = io.Copy(upstream, downstream)
		errC <- err
	}()
	go func() {
		defer upstream.Close()
		defer downstream.Close()
		_, err := io.Copy(downstream, upstream)
		errC <- err
	}()

	err = trace.NewAggregate(<-errC, <-errC)
	if utils.IsOKNetworkError(err) {
		err = nil
	}
	return err
}

func (s *SSHProxyService) String() string {
	return config.SSHProxyServiceType
}
