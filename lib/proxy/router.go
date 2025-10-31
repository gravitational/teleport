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
	"log/slog"
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
	"github.com/gravitational/teleport/lib/desktop"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/sshagent"
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

type serverResolverFn = func(ctx context.Context, host, port string, cluster cluster) (types.Server, error)
type windowsDesktopServiceConnectorFn = func(ctx context.Context, config *desktop.ConnectionConfig) (conn net.Conn, version string, err error)

// ClusterGetter provides access to connected local or remote clusters.
type ClusterGetter interface {
	// Cluster returns the cluster matching the provided clusterName
	Cluster(ctx context.Context, clusterName string) (reversetunnelclient.Cluster, error)
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
	// ClusterGetter allows looking up clusters
	ClusterGetter ClusterGetter
	// TracerProvider allows tracers to be created
	TracerProvider oteltrace.TracerProvider
	// Log is an optional logger. A default logger will be created if not set.
	Logger *slog.Logger

	// serverResolver is used to resolve hosts, used by tests
	serverResolver serverResolverFn
	// serverResolver is used to connect to Windows desktop service, used by tests
	windowsDesktopServiceConnector windowsDesktopServiceConnectorFn
}

// CheckAndSetDefaults ensures the required items were populated
func (c *RouterConfig) CheckAndSetDefaults() error {
	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName must be provided")
	}

	if c.LocalAccessPoint == nil {
		return trace.BadParameter("LocalAccessPoint must be provided")
	}

	if c.ClusterGetter == nil {
		return trace.BadParameter("SiteGetter must be provided")
	}

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.DefaultProvider()
	}

	if c.serverResolver == nil {
		c.serverResolver = getServer
	}

	if c.windowsDesktopServiceConnector == nil {
		c.windowsDesktopServiceConnector = desktop.ConnectToWindowsService
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	return nil
}

// Router is used by the proxy to establish connections to
// nodes, desktops, and other clusters.
type Router struct {
	clusterName                    string
	localAccessPoint               LocalAccessPoint
	localCluster                   reversetunnelclient.Cluster
	clusterGetter                  ClusterGetter
	tracer                         oteltrace.Tracer
	log                            *slog.Logger
	serverResolver                 serverResolverFn
	windowsDesktopServiceConnector windowsDesktopServiceConnectorFn
}

// NewRouter creates and returns a Router that is populated
// from the provided RouterConfig.
func NewRouter(cfg RouterConfig) (*Router, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	localCluster, err := cfg.ClusterGetter.Cluster(context.Background(), cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Router{
		clusterName:                    cfg.ClusterName,
		localAccessPoint:               cfg.LocalAccessPoint,
		localCluster:                   localCluster,
		clusterGetter:                  cfg.ClusterGetter,
		tracer:                         cfg.TracerProvider.Tracer("Router"),
		log:                            cfg.Logger,
		serverResolver:                 cfg.serverResolver,
		windowsDesktopServiceConnector: cfg.windowsDesktopServiceConnector,
	}, nil
}

// DialHost dials the node that matches the provided host, port and cluster. If no matching node
// is found an error is returned. If more than one matching node is found and the cluster networking
// configuration is not set to route to the most recent an error is returned.
func (r *Router) DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, clusterName string, clusterAccessChecker func(types.RemoteCluster) error, agentGetter sshagent.ClientGetter, signer agentless.SignerCreator) (_ net.Conn, err error) {
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

	cluster := r.localCluster
	if clusterName != r.clusterName {
		remoteCluster, err := r.getRemoteCluster(ctx, clusterName, clusterAccessChecker)
		if err != nil {
			return nil, trace.Wrap(err, "looking up remote cluster %q", clusterName)
		}
		cluster = remoteCluster
	}

	span.AddEvent("looking up server")
	target, err := r.serverResolver(ctx, host, port, fakeCluster{cluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("retrieved target server")

	serverID := target.GetName() + "." + clusterName
	hostClusterPrincipal := host + "." + clusterName
	principals := []string{
		host,
		// Add in hostClusterPrincipal for when nodes are on leaf clusters.
		hostClusterPrincipal,
	}

	// add hostUUID.cluster to the principals if it's different from hostClusterPrincipal.
	if serverID != hostClusterPrincipal {
		principals = append(principals, serverID)
	}

	serverAddr := target.GetAddr()
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
	var sshSigner ssh.Signer
	if target.IsOpenSSHNode() {
		agentGetter = nil

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
		}
	}

	conn, err := cluster.Dial(reversetunnelclient.DialParams{
		From:                  clientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: serverAddr},
		OriginalClientDstAddr: clientDstAddr,
		GetUserAgent:          agentGetter,
		AgentlessSigner:       sshSigner,
		Address:               host,
		Principals:            apiutils.Deduplicate(principals),
		ServerID:              serverID,
		ProxyIDs:              target.GetProxyIDs(),
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

// DialWindowsDesktop dials the desktop that matches the provided desktop name and cluster.
// If no matching desktop is found, an error is returned.
func (r *Router) DialWindowsDesktop(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, desktopName, clusterName string, clusterAccessChecker func(types.RemoteCluster) error) (_ net.Conn, err error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/DialWindowsDesktop",
		oteltrace.WithAttributes(
			attribute.String("desktopName", desktopName),
			attribute.String("cluster", clusterName),
		),
	)
	defer func() { tracing.EndSpan(span, err) }()

	cluster := r.localCluster
	if clusterName != r.clusterName {
		remoteCluster, err := r.getRemoteCluster(ctx, clusterName, clusterAccessChecker)
		if err != nil {
			return nil, trace.Wrap(err, "looking up remote cluster %q", clusterName)
		}
		cluster = remoteCluster
	}

	accessPoint, err := cluster.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	span.AddEvent("looking up Windows desktop service connection")

	serviceConn, _, err := r.windowsDesktopServiceConnector(ctx, &desktop.ConnectionConfig{
		Log:            r.log,
		DesktopsGetter: accessPoint,
		Cluster:        cluster,
		ClientSrcAddr:  clientSrcAddr,
		ClientDstAddr:  clientDstAddr,
		ClusterName:    clusterName,
		DesktopName:    desktopName,
	})
	if err != nil {
		return nil, trace.Wrap(err, "cannot connect to Windows Desktop Service")
	}

	span.AddEvent("retrieved Windows desktop service connection")

	return serviceConn, trace.Wrap(err)
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

// getRemoteCluster looks up the provided clusterName to determine if a remote cluster exists with
// that name and determines if the user has access to it.
func (r *Router) getRemoteCluster(ctx context.Context, clusterName string, clusterAccessChecker func(types.RemoteCluster) error) (reversetunnelclient.Cluster, error) {
	_, span := r.tracer.Start(
		ctx,
		"router/getRemoteCluster",
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer span.End()

	cluster, err := r.clusterGetter.Cluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	rc, err := r.localAccessPoint.GetRemoteCluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	if err := clusterAccessChecker(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	return cluster, nil
}

// cluster is the minimum interface needed to match servers
// for a reversetunnelclient.Cluster. It makes testing easier.
type cluster interface {
	GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetGitServers(context.Context, func(readonly.Server) bool) ([]types.Server, error)
}

// fakeCluster is a cluster implementation that wraps
// a reversetunnelclient.Cluster
type fakeCluster struct {
	cluster reversetunnelclient.Cluster
}

// GetNodes uses the wrapped cluster's NodeWatcher to filter nodes
func (r fakeCluster) GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	watcher, err := r.cluster.NodeWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := watcher.CurrentResourcesWithFilter(ctx, fn)
	return servers, trace.Wrap(err)
}

// GetGitServers uses the wrapped cluster's GitServerWatcher to filter git servers.
func (r fakeCluster) GetGitServers(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	watcher, err := r.cluster.GitServerWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return watcher.CurrentResourcesWithFilter(ctx, fn)
}

// GetClusterNetworkingConfig uses the wrapped cluster's cache to retrieve the ClusterNetworkingConfig
func (r fakeCluster) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	ap, err := r.cluster.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := ap.GetClusterNetworkingConfig(ctx)
	return cfg, trace.Wrap(err)
}

// getServer attempts to locate a node matching the provided host and port in
// the provided cluster.
func getServer(ctx context.Context, host, port string, cluster cluster) (types.Server, error) {
	if org, ok := types.GetGitHubOrgFromNodeAddr(host); ok {
		return getGitHubServer(ctx, org, cluster)
	}
	return getServerWithResolver(ctx, host, port, cluster, nil /* use default resolver */)
}

var disableUnqualifiedLookups = os.Getenv("TELEPORT_UNSTABLE_DISABLE_UNQUALIFIED_LOOKUPS") == "yes"

// getServerWithResolver attempts to locate a node matching the provided host and port in
// the provided cluster. The resolver argument is used in certain tests to mock DNS resolution
// and can generally be left nil.
func getServerWithResolver(ctx context.Context, host, port string, cluster cluster, resolver apiutils.HostResolver) (types.Server, error) {
	if cluster == nil {
		return nil, trace.BadParameter("invalid remote cluster provided")
	}

	strategy := types.RoutingStrategy_UNAMBIGUOUS_MATCH
	var caseInsensitiveRouting bool
	if cfg, err := cluster.GetClusterNetworkingConfig(ctx); err == nil {
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
	matches, err := cluster.GetNodes(ctx, func(server readonly.Server) bool {
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

	switch {
	case len(matches) == 1:
		return matches[0], nil

	case len(matches) > 1:
		// TODO(tross) DELETE IN V20.0.0
		// NodeIsAmbiguous is included in the error message for backwards compatibility
		// with older nodes that expect to see that string in the error message.
		if strategy != types.RoutingStrategy_MOST_RECENT {
			return nil, trace.Wrap(teleport.ErrNodeIsAmbiguous, teleport.NodeIsAmbiguous)
		}

		var recentServer types.Server
		for _, m := range matches {
			if recentServer == nil || m.Expiry().After(recentServer.Expiry()) {
				recentServer = m
			}
		}
		return recentServer, nil

	default: // no matches
		if routeMatcher.MatchesServerIDs() {
			idType := "UUID"
			if aws.IsEC2NodeID(host) {
				idType = "EC2"
			}
			return nil, trace.NotFound("unable to locate node matching %s-like target %s", idType, host)
		}

		return nil, trace.ConnectionProblem(nil, "target host %s is offline or does not exist", host)
	}
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
		conn, err := r.localCluster.DialAuthServer(reversetunnelclient.DialParams{From: clientSrcAddr, OriginalClientDstAddr: clientDstAddr})
		return conn, trace.Wrap(err)
	}

	// lookup the cluster and dial its auth server
	cluster, err := r.clusterGetter.Cluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := cluster.DialAuthServer(reversetunnelclient.DialParams{From: clientSrcAddr, OriginalClientDstAddr: clientDstAddr})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewProxiedMetricConn(conn), trace.Wrap(err)
}

// GetSiteClient returns an auth client for the provided cluster.
func (r *Router) GetSiteClient(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	if clusterName == r.clusterName {
		return r.localCluster.GetClient()
	}

	cluster, err := r.clusterGetter.Cluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster.GetClient()
}

func getGitHubServer(ctx context.Context, gitHubOrg string, cluster cluster) (types.Server, error) {
	servers, err := cluster.GetGitServers(ctx, func(s readonly.Server) bool {
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
