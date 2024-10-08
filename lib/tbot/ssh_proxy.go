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

package tbot

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
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
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxySSHConfig contains configuration parameters required
// to initialize the local ssh proxy.
type ProxySSHConfig struct {
	DestinationPath           string
	Insecure                  bool
	FIPS                      bool
	TSHConfigPath             string
	ProxyServer               string
	Cluster                   string
	User                      string
	Host                      string
	Port                      string
	EnableResumption          bool
	TLSRoutingEnabled         bool
	ConnectionUpgradeRequired bool
	Log                       *slog.Logger
}

// ProxySSH creates a local ssh proxy, dialing a node and transferring data through
// stdin and stdout, to be used as an OpenSSH/PuTTY proxy command.
func ProxySSH(ctx context.Context, proxyConfig ProxySSHConfig) error {
	tshConfig := &libclient.TSHConfig{}
	if proxyConfig.TSHConfigPath != "" {
		var err error
		tshConfig, err = libclient.LoadTSHConfig(proxyConfig.TSHConfigPath)
		if err != nil {
			return trace.Wrap(err, "loading proxy templates")
		}
	}

	proxy := proxyConfig.ProxyServer
	cluster := proxyConfig.Cluster
	targetHost := proxyConfig.Host
	expanded, matched := tshConfig.ProxyTemplates.Apply(
		net.JoinHostPort(proxyConfig.Host, proxyConfig.Port),
	)
	if matched {
		proxyConfig.Log.DebugContext(
			ctx,
			"proxy templated matched",
			"populated_template", expanded,
		)
		if expanded.Cluster != "" {
			cluster = expanded.Cluster
		}
		if expanded.Proxy != "" {
			proxy = expanded.Proxy
		}
		if expanded.Host != "" {
			targetHost = expanded.Host
		}
	}

	proxyHost, _, err := net.SplitHostPort(proxy)
	if err != nil {
		return trace.Wrap(err)
	}

	facade, keyring, err := parseIdentity(
		proxyConfig.DestinationPath,
		proxyHost,
		cluster,
		proxyConfig.Insecure,
		proxyConfig.FIPS,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	sshConfig, err := facade.SSHClientConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	if proxyConfig.User != "" {
		sshConfig.User = proxyConfig.User
	}

	if cluster == "" {
		cluster = facade.Get().ClusterName
	}

	pclt, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      proxy,
		TLSRoutingEnabled: proxyConfig.TLSRoutingEnabled,
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
		UnaryInterceptors:       []grpc.UnaryClientInterceptor{interceptors.GRPCClientUnaryErrorInterceptor},
		StreamInterceptors:      []grpc.StreamClientInterceptor{interceptors.GRPCClientStreamErrorInterceptor},
		SSHConfig:               sshConfig,
		InsecureSkipVerify:      proxyConfig.Insecure,
		ALPNConnUpgradeRequired: proxyConfig.ConnectionUpgradeRequired,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var target string
	if expanded == nil || (len(expanded.Search) == 0 && expanded.Query == "") {
		targetHost = cleanTargetHost(targetHost, proxyHost, cluster)
		target = net.JoinHostPort(targetHost, proxyConfig.Port)
	} else {
		authConfig, err := pclt.ClientConfig(ctx, cluster)
		if err != nil {
			return trace.Wrap(err)
		}

		// Override the credentials with the facade, which is tailored for
		// connections to auth. The proxy client will try to use the TLS
		// config from above that was explicitly tailored for connecting
		// to the proxy, and if reused, will result in handshake failures.
		authConfig.Credentials = []client.Credentials{facade}

		node, err := resolveTargetHost(ctx, authConfig, expanded.Search, expanded.Query)
		if err != nil {
			return trace.Wrap(err)
		}

		proxyConfig.Log.DebugContext(ctx, "found matching SSH host", "host_uuid", node.GetName(), "host_name", node.GetHostname())

		target = net.JoinHostPort(node.GetName(), "0")
	}

	conn, _, err := pclt.DialHost(ctx, target, cluster, keyring)
	if err != nil {
		return trace.Wrap(err)
	}

	if proxyConfig.EnableResumption {
		conn, err = resumption.WrapSSHClientConn(ctx, conn, func(ctx context.Context, hostID string) (net.Conn, error) {
			// if the connection is being resumed, it means that
			// we didn't need the agent in the first place
			var noAgent agent.ExtendedAgent
			conn, _, err := pclt.DialHost(ctx, net.JoinHostPort(hostID, "0"), cluster, noAgent)
			return conn, err
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer conn.Close()

	err = trace.Wrap(utils.ProxyConn(ctx, utils.CombinedStdio{}, conn))
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	return trace.Wrap(err)
}

// resolveTargetHost determines the ssh instance to be connected to based on either
// the provided search or query. The auth client is intentionally single use and
// closed to reduce resource consumption after the host has been resolved. Any future
// changes that require an additional request to auth should reuse the client instead
// of creating one per request.
func resolveTargetHost(ctx context.Context, cfg client.Config, search, query string) (types.Server, error) {
	apiClient, err := client.New(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer apiClient.Close()

	return resolveTargetHostWithClient(ctx, apiClient, search, query)
}

// resolveTargetHostWithClient resolves the target host using the provided
// client and search and query parameters.
func resolveTargetHostWithClient(
	ctx context.Context, clt client.ListUnifiedResourcesClient, search, query string,
) (types.Server, error) {
	resources, _, err := client.GetUnifiedResourcePage(ctx, clt, &proto.ListUnifiedResourcesRequest{
		// We only want a single node, but, we set limit=2 so we can throw a
		// helpful error when multiple match. In the happy path, where a single
		// node matches, this does not degrade performance because even if
		// limit=1 the UnifiedResource cache will still iterate to the end to
		// determine if there is a NextKey to return.
		Limit:               2,
		Kinds:               []string{types.KindNode},
		SearchKeywords:      libclient.ParseSearchKeywords(search, ','),
		PredicateExpression: query,
		SortBy:              types.SortBy{Field: types.ResourceKind},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(resources) == 0 {
		return nil, trace.NotFound("no matching SSH hosts found for search terms or query expression")
	}
	if len(resources) > 1 {
		names := make([]string, len(resources))
		for i, res := range resources {
			names[i] = res.GetName()
		}
		return nil, trace.BadParameter("found multiple matching SSH hosts %v", names)
	}
	node := resources[0].ResourceWithLabels.(*types.ServerV2)
	if node == nil {
		return nil, trace.BadParameter("expected node resource, got %T", resources[0].ResourceWithLabels)
	}
	return node, nil
}

func parseIdentity(destPath, proxy, cluster string, insecure, fips bool) (*identity.Facade, agent.ExtendedAgent, error) {
	identityPath := filepath.Join(destPath, config.IdentityFilePath)
	keyRing, err := identityfile.KeyRingFromIdentityFile(identityPath, proxy, cluster)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	i, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: keyRing.SSHPrivateKey.PrivateKeyPEM(),
		PublicKeyBytes:  keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
	}, &proto.Certs{
		SSH:        keyRing.Cert,
		TLS:        keyRing.TLSCert,
		TLSCACerts: keyRing.TLSCAs(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	agentKey, err := keyRing.AsAgentKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	if !ok {
		return nil, nil, trace.BadParameter("unexpected keyring type %T, expected agent.ExtendedAgent (this is a bug)", keyring)
	}
	if err := keyring.Add(agentKey); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return identity.NewFacade(fips, insecure, i), keyring, nil
}

func cleanTargetHost(targetHost, proxyHost, siteName string) string {
	targetHost = strings.TrimSuffix(targetHost, "."+proxyHost)
	targetHost = strings.TrimSuffix(targetHost, "."+siteName)
	return targetHost
}
