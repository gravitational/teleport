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

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/tshwrap"
	"github.com/gravitational/teleport/lib/utils"
)

func onProxySSHCommand(botConfig *config.BotConfig, cf *config.CLIConf) error {
	destination, err := tshwrap.GetDestinationDirectory(botConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	identityPath := filepath.Join(destination.Path, config.IdentityFilePath)

	ctx := context.Background()
	key, err := identityfile.KeyFromIdentityFile(identityPath, cf.ProxyServer, cf.Cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	i, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: key.PrivateKeyPEM(),
		PublicKeyBytes:  key.MarshalSSHPublicKey(),
	}, &proto.Certs{
		SSH:        key.Cert,
		TLS:        key.TLSCert,
		TLSCACerts: key.TLSCAs(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	facade := identity.NewFacade(false, false, i)

	sshConfig, err := facade.SSHClientConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster := cf.Cluster
	if cluster == "" {
		cluster = facade.Get().ClusterName
	}

	tshCFG, err := libclient.LoadAllConfigs("", "")
	if err != nil {
		return trace.Wrap(err)
	}

	_, hostPort, ok := strings.Cut(cf.UserHostPort, "@")
	if !ok {
		hostPort = cf.UserHostPort
	}

	proxy, _, err := net.SplitHostPort(cf.ProxyServer)
	if err != nil {
		return trace.Wrap(err)
	}

	expanded, matched := tshCFG.ProxyTemplates.Apply(hostPort)
	if matched {
		log.DebugContext(ctx, "proxy templated matched", "expanded", expanded)
		if expanded.Cluster != "" {
			cluster = expanded.Cluster
		}
		if expanded.Proxy != "" {
			proxy = expanded.Proxy
		}
	}

	pclt, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      cf.ProxyServer,
		TLSRoutingEnabled: true,
		TLSConfigFunc: func(cluster string) (*tls.Config, error) {
			cfg, err := facade.TLSConfig()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			cfg.ServerName = proxy
			cfg.NextProtos = nil
			return cfg, nil
		},
		UnaryInterceptors:  []grpc.UnaryClientInterceptor{interceptors.GRPCClientUnaryErrorInterceptor},
		StreamInterceptors: []grpc.StreamClientInterceptor{interceptors.GRPCClientStreamErrorInterceptor},
		SSHConfig:          sshConfig,
		InsecureSkipVerify: cf.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var target string
	switch {
	case expanded == nil:
		targetHost, targetPort, err := net.SplitHostPort(hostPort)
		if err != nil {
			targetHost = hostPort
			targetPort = "0"
		}
		targetHost = cleanTargetHost(targetHost, cf.ProxyServer, cluster)
		target = net.JoinHostPort(targetHost, targetPort)
	case expanded.Host != "":
		targetHost, targetPort, err := net.SplitHostPort(expanded.Host)
		if err != nil {
			targetHost = expanded.Host
			targetPort = "0"
		}
		targetHost = cleanTargetHost(targetHost, cf.ProxyServer, cluster)
		target = net.JoinHostPort(targetHost, targetPort)
	case len(expanded.Search) != 0 || expanded.Query != "":
		authClientCfg, err := pclt.ClientConfig(ctx, cluster)
		if err != nil {
			return trace.Wrap(err)
		}

		tlscfg, err := facade.TLSConfig()
		if err != nil {
			return trace.Wrap(err)
		}
		authClientCfg.Credentials = []client.Credentials{client.LoadTLS(tlscfg)}

		authClientCfg.DialInBackground = true
		apiClient, err := client.New(ctx, authClientCfg)
		if err != nil {
			return trace.Wrap(err)
		}

		nodes, err := client.GetAllResources[types.Server](ctx, apiClient, &proto.ListResourcesRequest{
			ResourceType:        types.KindNode,
			SearchKeywords:      libclient.ParseSearchKeywords(expanded.Search, ','),
			PredicateExpression: expanded.Query,
		})
		_ = apiClient.Close()
		if err != nil {
			return trace.Wrap(err)
		}

		if len(nodes) == 0 {
			return trace.NotFound("no matching SSH hosts found for search terms or query expression")
		}

		if len(nodes) > 1 {
			return trace.BadParameter("found multiple matching SSH hosts %v", nodes[:2])
		}

		log.DebugContext(ctx, "found matching SSH host", "host_uuid", nodes[0].GetName(), "host_name", nodes[0].GetHostname())

		// Dialing is happening by UUID but a port is still required by
		// the Proxy dial request. Zero is an indicator to the Proxy that
		// it may chose the appropriate port based on the target server.
		target = fmt.Sprintf("%s:0", nodes[0].GetName())
	default:
		return trace.BadParameter("no hostname, search terms or query expression provided")
	}

	agentKey, err := key.AsAgentKey()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(espadolini): figure out if and how we can just derive an agent from
	// [*identity.Facade] that's kept up to date as the facade is changed
	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	if !ok {
		return trace.BadParameter("unexpected keyring type %T, expected agent.ExtendedKeyring", keyring)
	}
	if err := keyring.Add(agentKey); err != nil {
		return trace.Wrap(err)
	}

	conn, _, err := pclt.DialHost(ctx, target, cluster, keyring)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	stdio := utils.CombineReadWriteCloser(io.NopCloser(os.Stdin), utils.NopWriteCloser(os.Stdout))
	err = trace.Wrap(utils.ProxyConn(ctx, stdio, conn))
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	return trace.Wrap(err)
}

func cleanTargetHost(targetHost, proxyHost, siteName string) string {
	targetHost = strings.TrimSuffix(targetHost, "."+proxyHost)
	targetHost = strings.TrimSuffix(targetHost, "."+siteName)
	return targetHost
}
