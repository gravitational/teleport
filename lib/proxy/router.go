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

package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"sync"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// proxiedSessions counts successful connections to nodes
	proxiedSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricProxySSHSessions,
			Help: "Number of active sessions through this proxy",
		},
	)

	// failedConnectingToNode counts failed attempts to connect to nodes
	failedConnectingToNode = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricFailedConnectToNodeAttempts,
			Help: "Number of failed SSH connection attempts to a node. Use with `teleport_connect_to_node_attempts_total` to get the failure rate.",
		},
	)

	// connectingToNode counts connection attempts to nodes
	connectingToNode = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricConnectToNodeAttempts,
			Help:      "Number of SSH connection attempts to a node. Use with `failed_connect_to_node_attempts_total` to get the failure rate.",
		},
	)
)

func init() {
	metrics.RegisterPrometheusCollectors(proxiedSessions, failedConnectingToNode, connectingToNode)
}

// ProxiedMetricConn wraps [net.Conn] opened by
// the [Router] so that the proxiedSessions counter
// can be decremented when it is closed.
type ProxiedMetricConn struct {
	// once ensures that proxiedSessions is only decremented
	// a single time per [net.Conn]
	once sync.Once
	net.Conn
}

// NewProxiedMetricConn increments proxiedSessions and creates
// a ProxiedMetricConn that defers to the provided [net.Conn].
func NewProxiedMetricConn(conn net.Conn) *ProxiedMetricConn {
	proxiedSessions.Inc()
	return &ProxiedMetricConn{Conn: conn}
}

func (c *ProxiedMetricConn) Close() error {
	c.once.Do(proxiedSessions.Dec)
	return trace.Wrap(c.Conn.Close())
}

type serverResolverFn = func(ctx context.Context, host, port string, site site) (types.Server, error)

// SiteGetter provides access to connected local or remote sites
type SiteGetter interface {
	// GetSite returns the site matching the provided clusterName
	GetSite(clusterName string) (reversetunnelclient.RemoteSite, error)
}

// LocalAccessPoint provides access to remote cluster resources
type LocalAccessPoint interface {
	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
	// GetAuthPreference returns the local cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// RouterConfig contains all the dependencies required
// by the Router
type RouterConfig struct {
	// ClusterName indicates which cluster the router is for
	ClusterName string
	// LocalAccessPoint is the proxy cache
	LocalAccessPoint LocalAccessPoint
	// SiteGetter allows looking up sites
	SiteGetter SiteGetter
	// TracerProvider allows tracers to be created
	TracerProvider oteltrace.TracerProvider

	// serverResolver is used to resolve hosts, used by tests
	serverResolver serverResolverFn
}

// CheckAndSetDefaults ensures the required items were populated
func (c *RouterConfig) CheckAndSetDefaults() error {
	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName must be provided")
	}

	if c.LocalAccessPoint == nil {
		return trace.BadParameter("LocalAccessPoint must be provided")
	}

	if c.SiteGetter == nil {
		return trace.BadParameter("SiteGetter must be provided")
	}

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.DefaultProvider()
	}

	if c.serverResolver == nil {
		c.serverResolver = getServer
	}

	return nil
}

// Router is used by the proxy to establish connections to both
// nodes and other clusters.
type Router struct {
	clusterName      string
	localAccessPoint LocalAccessPoint
	localSite        reversetunnelclient.RemoteSite
	siteGetter       SiteGetter
	tracer           oteltrace.Tracer
	serverResolver   serverResolverFn
}

// NewRouter creates and returns a Router that is populated
// from the provided RouterConfig.
func NewRouter(cfg RouterConfig) (*Router, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	localSite, err := cfg.SiteGetter.GetSite(cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Router{
		clusterName:      cfg.ClusterName,
		localAccessPoint: cfg.LocalAccessPoint,
		localSite:        localSite,
		siteGetter:       cfg.SiteGetter,
		tracer:           cfg.TracerProvider.Tracer("Router"),
		serverResolver:   cfg.serverResolver,
	}, nil
}

// DialHost dials the node that matches the provided host, port and cluster. If no matching node
// is found an error is returned. If more than one matching node is found and the cluster networking
// configuration is not set to route to the most recent an error is returned.
func (r *Router) DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, clusterName string, accessChecker services.AccessChecker, agentGetter teleagent.Getter, signer agentless.SignerCreator) (_ net.Conn, err error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/DialHost",
		oteltrace.WithAttributes(
			attribute.String("host", host),
			attribute.String("port", port),
			attribute.String("cluster", clusterName),
		),
	)
	connectingToNode.Inc()
	defer func() {
		if err != nil {
			failedConnectingToNode.Inc()
		}
		tracing.EndSpan(span, err)
	}()

	site := r.localSite
	if clusterName != r.clusterName {
		remoteSite, err := r.getRemoteCluster(ctx, clusterName, accessChecker)
		if err != nil {
			return nil, trace.Wrap(err, "looking up remote cluster %q", clusterName)
		}
		site = remoteSite
	}

	span.AddEvent("looking up server")
	target, err := r.serverResolver(ctx, host, port, remoteSite{site})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("retrieved target server")

	principals := []string{host}

	var (
		isAgentlessNode bool
		serverID        string
		serverAddr      string
		proxyIDs        []string
		sshSigner       ssh.Signer
	)

	if target != nil {
		proxyIDs = target.GetProxyIDs()
		serverID = fmt.Sprintf("%v.%v", target.GetName(), clusterName)

		// add hostUUID.cluster to the principals
		principals = append(principals, serverID)

		// add ip if it exists to the principals
		serverAddr = target.GetAddr()

		switch {
		case serverAddr != "":
			h, _, err := net.SplitHostPort(serverAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			principals = append(principals, h)
		case serverAddr == "" && target.GetUseTunnel():
			serverAddr = reversetunnelclient.LocalNode
		}
		// If the node is a registered openssh node don't set agentGetter
		// so a SSH user agent will not be created when connecting to the remote node.
		if target.IsOpenSSHNode() {
			agentGetter = nil
			isAgentlessNode = true

			if target.GetSubKind() == types.SubKindOpenSSHNode {
				// If the node is of SubKindOpenSSHNode, create the signer.
				client, err := r.GetSiteClient(ctx, clusterName)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				sshSigner, err = signer(ctx, r.localAccessPoint, client)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				principals = append(principals, fmt.Sprintf("%s.%s", host, clusterName))
			}
		}
	} else {
		return nil, trace.ConnectionProblem(errors.New("connection problem"), "direct dialing to nodes not found in inventory is not supported")
	}

	conn, err := site.Dial(reversetunnelclient.DialParams{
		From:                  clientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: serverAddr},
		OriginalClientDstAddr: clientDstAddr,
		GetUserAgent:          agentGetter,
		IsAgentlessNode:       isAgentlessNode,
		AgentlessSigner:       sshSigner,
		Address:               host,
		Principals:            principals,
		ServerID:              serverID,
		ProxyIDs:              proxyIDs,
		ConnType:              types.NodeTunnel,
		TargetServer:          target,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// SSH connection MUST start with "SSH-2.0" bytes according to https://datatracker.ietf.org/doc/html/rfc4253#section-4.2
	conn = newCheckedPrefixWriter(conn, []byte("SSH-2.0"))
	return NewProxiedMetricConn(conn), trace.Wrap(err)
}

// checkedPrefixWriter checks that first data written into it has the specified prefix.
type checkedPrefixWriter struct {
	net.Conn

	requiredPrefix  []byte
	requiredPointer int
}

func newCheckedPrefixWriter(conn net.Conn, requiredPrefix []byte) *checkedPrefixWriter {
	return &checkedPrefixWriter{
		Conn:           conn,
		requiredPrefix: requiredPrefix,
	}
}

// Write writes data into connection, checking if it has required prefix. Not safe for concurrent calls.
func (c *checkedPrefixWriter) Write(p []byte) (int, error) {
	// If pointer reached end of required prefix the check is done
	if len(c.requiredPrefix) == c.requiredPointer {
		n, err := c.Conn.Write(p)
		return n, trace.Wrap(err)
	}

	// Decide which is smaller, provided data or remaining portion of the required prefix
	small, big := c.requiredPrefix[c.requiredPointer:], p
	if len(small) > len(big) {
		big, small = small, big
	}

	if !bytes.HasPrefix(big, small) {
		return 0, trace.AccessDenied("required prefix %q was not found", c.requiredPrefix)
	}
	n, err := c.Conn.Write(p)
	// Advance pointer by confirmed portion of the prefix.
	c.requiredPointer += min(n, len(small))
	return n, trace.Wrap(err)
}

// getRemoteCluster looks up the provided clusterName to determine if a remote site exists with
// that name and determines if the user has access to it.
func (r *Router) getRemoteCluster(ctx context.Context, clusterName string, checker services.AccessChecker) (reversetunnelclient.RemoteSite, error) {
	_, span := r.tracer.Start(
		ctx,
		"router/getRemoteCluster",
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer span.End()

	site, err := r.siteGetter.GetSite(clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	rc, err := r.localAccessPoint.GetRemoteCluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	if err := checker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	return site, nil
}

// site is the minimum interface needed to match servers
// for a reversetunnelclient.RemoteSite. It makes testing easier.
type site interface {
	GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetGitServers(context.Context, func(readonly.Server) bool) ([]types.Server, error)
}

// remoteSite is a site implementation that wraps
// a reversetunnelclient.RemoteSite
type remoteSite struct {
	site reversetunnelclient.RemoteSite
}

// GetNodes uses the wrapped sites NodeWatcher to filter nodes
func (r remoteSite) GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	watcher, err := r.site.NodeWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := watcher.CurrentResourcesWithFilter(ctx, fn)
	return servers, trace.Wrap(err)
}

// GetGitServers uses the wrapped sites GitServerWatcher to filter git servers.
func (r remoteSite) GetGitServers(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	watcher, err := r.site.GitServerWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return watcher.CurrentResourcesWithFilter(ctx, fn)
}

// GetClusterNetworkingConfig uses the wrapped sites cache to retrieve the ClusterNetworkingConfig
func (r remoteSite) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	ap, err := r.site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := ap.GetClusterNetworkingConfig(ctx)
	return cfg, trace.Wrap(err)
}

// getServer attempts to locate a node matching the provided host and port in
// the provided site.
func getServer(ctx context.Context, host, port string, site site) (types.Server, error) {
	if org, ok := types.GetGitHubOrgFromNodeAddr(host); ok {
		return getGitHubServer(ctx, org, site)
	}
	return getServerWithResolver(ctx, host, port, site, nil /* use default resolver */)
}

var disableUnqualifiedLookups = os.Getenv("TELEPORT_UNSTABLE_DISABLE_UNQUALIFIED_LOOKUPS") == "yes"

// getServerWithResolver attempts to locate a node matching the provided host and port in
// the provided site. The resolver argument is used in certain tests to mock DNS resolution
// and can generally be left nil.
func getServerWithResolver(ctx context.Context, host, port string, site site, resolver apiutils.HostResolver) (types.Server, error) {
	if site == nil {
		return nil, trace.BadParameter("invalid remote site provided")
	}

	strategy := types.RoutingStrategy_UNAMBIGUOUS_MATCH
	var caseInsensitiveRouting bool
	if cfg, err := site.GetClusterNetworkingConfig(ctx); err == nil {
		strategy = cfg.GetRoutingStrategy()
		caseInsensitiveRouting = cfg.GetCaseInsensitiveRouting()
	}

	routeMatcher, err := apiutils.NewSSHRouteMatcherFromConfig(apiutils.SSHRouteMatcherConfig{
		Host:                      host,
		Port:                      port,
		CaseInsensitive:           caseInsensitiveRouting,
		Resolver:                  resolver,
		DisableUnqualifiedLookups: disableUnqualifiedLookups,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var maxScore int
	scores := make(map[string]int)
	matches, err := site.GetNodes(ctx, func(server readonly.Server) bool {
		score := routeMatcher.RouteToServerScore(server)
		if score < 1 {
			return false
		}

		scores[server.GetName()] = score
		maxScore = max(maxScore, score)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if routeMatcher.MatchesServerIDs() && len(matches) > 1 {
		// if a dial request for an id-like target creates multiple matches,
		// give precedence to the exact match if one exists. If not, handle
		// multiple matchers per-usual below.
		for _, m := range matches {
			if m.GetName() == host {
				matches = []types.Server{m}
				break
			}
		}
	}

	if len(matches) > 1 {
		// in the event of multiple matches, some matches may be of higher quality than others
		// (e.g. matching an ip/hostname directly versus matching a resolved ip). if we have a
		// mix of match qualities, filter out the lower quality matches to reduce ambiguity.
		filtered := matches[:0]
		for _, m := range matches {
			if scores[m.GetName()] < maxScore {
				continue
			}

			filtered = append(filtered, m)
		}
		matches = filtered
	}

	var server types.Server
	switch {
	case strategy == types.RoutingStrategy_MOST_RECENT:
		for _, m := range matches {
			if server == nil || m.Expiry().After(server.Expiry()) {
				server = m
			}
		}
	case len(matches) > 1:
		// TODO(tross) DELETE IN V20.0.0
		// NodeIsAmbiguous is included in the error message for backwards compatibility
		// with older nodes that expect to see that string in the error message.
		return nil, trace.Wrap(teleport.ErrNodeIsAmbiguous, teleport.NodeIsAmbiguous)
	case len(matches) == 1:
		server = matches[0]
	}

	if routeMatcher.MatchesServerIDs() && server == nil {
		idType := "UUID"
		if aws.IsEC2NodeID(host) {
			idType = "EC2"
		}

		return nil, trace.NotFound("unable to locate node matching %s-like target %s", idType, host)
	}

	return server, nil
}

// DialSite establishes a connection to the auth server in the provided
// cluster. If the clusterName is an empty string then a connection to
// the local auth server will be established.
func (r *Router) DialSite(ctx context.Context, clusterName string, clientSrcAddr, clientDstAddr net.Addr) (_ net.Conn, err error) {
	_, span := r.tracer.Start(
		ctx,
		"router/DialSite",
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer func() { tracing.EndSpan(span, err) }()

	// default to local cluster if one wasn't provided
	if clusterName == "" {
		clusterName = r.clusterName
	}

	// dial the local auth server
	if clusterName == r.clusterName {
		conn, err := r.localSite.DialAuthServer(reversetunnelclient.DialParams{From: clientSrcAddr, OriginalClientDstAddr: clientDstAddr})
		return conn, trace.Wrap(err)
	}

	// lookup the site and dial its auth server
	site, err := r.siteGetter.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := site.DialAuthServer(reversetunnelclient.DialParams{From: clientSrcAddr, OriginalClientDstAddr: clientDstAddr})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewProxiedMetricConn(conn), trace.Wrap(err)
}

// GetSiteClient returns an auth client for the provided cluster.
func (r *Router) GetSiteClient(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	if clusterName == r.clusterName {
		return r.localSite.GetClient()
	}

	site, err := r.siteGetter.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return site.GetClient()
}

func getGitHubServer(ctx context.Context, gitHubOrg string, site site) (types.Server, error) {
	servers, err := site.GetGitServers(ctx, func(s readonly.Server) bool {
		github := s.GetGitHub()
		return github != nil && github.Organization == gitHubOrg
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch len(servers) {
	case 0:
		return nil, trace.NotFound("unable to locate Git server for GitHub organization %s", gitHubOrg)
	case 1:
		return servers[0], nil
	default:
		// It's unusual but possible to have multiple servers per organization
		// (e.g. possibly a second Git server for a manual CA rotation). Pick a
		// random one.
		return servers[rand.N(len(servers))], nil
	}
}
