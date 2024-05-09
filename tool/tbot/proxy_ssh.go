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
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
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
	if err != nil {
		return trace.Wrap(err)
	}

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

	tshCFG, err := loadConfig(filepath.Join(profile.FullProfilePath(""), "config/config.yaml"))
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
			SearchKeywords:      ParseSearchKeywords(expanded.Search, ','),
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

	conn, _, err := pclt.DialHost(ctx, target, facade.Get().ClusterName, nil)
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

func ParseSearchKeywords(spec string, customDelimiter rune) []string {
	delimiter := customDelimiter
	if delimiter == 0 {
		delimiter = rune(',')
	}

	var tokens []string
	openQuotes := false
	var tokenStart int
	specLen := len(spec)
	// tokenize the label search:
	for i, ch := range spec {
		endOfToken := false
		if i+utf8.RuneLen(ch) == specLen {
			i += utf8.RuneLen(ch)
			endOfToken = true
		}
		switch ch {
		case '"':
			openQuotes = !openQuotes
		case delimiter:
			if !openQuotes {
				endOfToken = true
			}
		}
		if endOfToken && i > tokenStart {
			tokens = append(tokens, strings.TrimSpace(strings.Trim(spec[tokenStart:i], `"`)))
			tokenStart = i + 1
		}
	}

	return tokens
}

func cleanTargetHost(targetHost, proxyHost, siteName string) string {
	targetHost = strings.TrimSuffix(targetHost, "."+proxyHost)
	targetHost = strings.TrimSuffix(targetHost, "."+siteName)
	return targetHost
}

// loadConfig load a single config file from given path. If the path does not exist, an empty config is returned instead.
func loadConfig(fullConfigPath string) (*TSHConfig, error) {
	bs, err := os.ReadFile(fullConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &TSHConfig{}, nil
		}
		return nil, trace.ConvertSystemError(err)
	}
	var cfg TSHConfig
	if err := yaml.Unmarshal(bs, &cfg); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// TSHConfig represents configuration loaded from the tsh config file.
type TSHConfig struct {
	// ProxyTemplates describe rules for parsing out proxy out of full hostnames.
	ProxyTemplates ProxyTemplates `yaml:"proxy_templates,omitempty"`
	// Aliases are custom commands extending baseline tsh functionality.
	Aliases map[string]string `yaml:"aliases,omitempty"`
}

// Check validates the tsh config.
func (config *TSHConfig) Check() error {
	for _, template := range config.ProxyTemplates {
		if err := template.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ProxyTemplates represents a list of individual proxy templates.
type ProxyTemplates []*ProxyTemplate

// ExpandedTemplate contains any matched date from a
// [ProxyTemplate] that has been expanded after being evaluated.
type ExpandedTemplate struct {
	Proxy   string
	Host    string
	Cluster string
	Query   string
	Search  string
}

func (e ExpandedTemplate) String() string {
	return fmt.Sprintf("Proxy=%s,Host=%s,Cluster=%s,Query=%s,Search=%s", e.Proxy, e.Host, e.Cluster, e.Query, e.Search)
}

// Apply attempts to match the provided full hostname against all the templates
// in the list. Returns extracted proxy and host upon encountering the first
// matching template.
func (t ProxyTemplates) Apply(fullHostname string) (expanded *ExpandedTemplate, matched bool) {
	for _, template := range t {
		expanded, matched := template.Apply(fullHostname)
		if matched {
			return expanded, true
		}
	}
	return nil, false
}

// ProxyTemplate describes a single rule for parsing out proxy address from
// the full hostname. Used by tsh proxy ssh.
type ProxyTemplate struct {
	// Template is a regular expression that full hostname is matched against.
	Template string `yaml:"template"`
	// Proxy is the proxy address. Can refer to regex groups from the template.
	Proxy string `yaml:"proxy"`
	// Cluster is an optional cluster name. Can refer to regex groups from the template.
	Cluster string `yaml:"cluster"`
	// Host is an optional hostname. Can refer to regex groups from the template.
	Host string `yaml:"host"`
	// Query is an optional predicate expression used to resolve the target host.
	// Can refer to regex groups from the template.
	Query string `yaml:"query"`
	// Search contains optional fuzzy matching terms used to resolve the target host.
	// Can refer to regex groups from the template.
	Search string `yaml:"search"`

	// re is the compiled template regexp.
	re *regexp.Regexp
}

// Check validates the proxy template.
func (t *ProxyTemplate) Check() (err error) {
	if strings.TrimSpace(t.Template) == "" {
		return trace.BadParameter("empty proxy template")
	}

	if strings.TrimSpace(t.Proxy) == "" &&
		strings.TrimSpace(t.Cluster) == "" &&
		strings.TrimSpace(t.Host) == "" &&
		strings.TrimSpace(t.Query) == "" &&
		strings.TrimSpace(t.Search) == "" {
		return trace.BadParameter("empty proxy, cluster, host, query, and search fields in proxy template, but at least one is required")
	}
	t.re, err = regexp.Compile(t.Template)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Apply applies the proxy template to the provided hostname and returns
// expanded proxy address and hostname.
func (t *ProxyTemplate) Apply(fullHostname string) (_ *ExpandedTemplate, matched bool) {
	match := t.re.FindAllStringSubmatchIndex(fullHostname, -1)
	if match == nil {
		return nil, false
	}

	var expanded ExpandedTemplate
	if t.Proxy != "" {
		var expandedProxy []byte
		for _, m := range match {
			expandedProxy = t.re.ExpandString(expandedProxy, t.Proxy, fullHostname, m)
		}
		expanded.Proxy = string(expandedProxy)
	}

	if t.Host != "" {
		var expandedHost []byte
		for _, m := range match {
			expandedHost = t.re.ExpandString(expandedHost, t.Host, fullHostname, m)
		}
		expanded.Host = string(expandedHost)
	}

	if t.Cluster != "" {
		var expandedCluster []byte
		for _, m := range match {
			expandedCluster = t.re.ExpandString(expandedCluster, t.Cluster, fullHostname, m)
		}
		expanded.Cluster = string(expandedCluster)
	}

	if t.Query != "" {
		var expandedQuery []byte
		for _, m := range match {
			expandedQuery = t.re.ExpandString(expandedQuery, t.Query, fullHostname, m)
		}
		expanded.Query = string(expandedQuery)
	}

	if t.Search != "" {
		var expandedSearch []byte
		for _, m := range match {
			expandedSearch = t.re.ExpandString(expandedSearch, t.Search, fullHostname, m)
		}
		expanded.Search = string(expandedSearch)
	}

	return &expanded, true
}
