// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
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

// proxiedMetricConn wraps [net.Conn] opened by
// the [Router] so that the proxiedSessions counter
// can be decremented when it is closed.
type proxiedMetricConn struct {
	// once ensures that proxiedSessions is only decremented
	// a single time per [net.Conn]
	once sync.Once
	net.Conn
}

// newProxiedMetricConn increments proxiedSessions and creates
// a proxiedMetricConn that defers to the provided [net.Conn].
func newProxiedMetricConn(conn net.Conn) *proxiedMetricConn {
	proxiedSessions.Inc()
	return &proxiedMetricConn{Conn: conn}
}

func (c *proxiedMetricConn) Close() error {
	c.once.Do(proxiedSessions.Dec)
	return trace.Wrap(c.Conn.Close())
}

type serverResolverFn = func(ctx context.Context, host, port string, site site) (types.Server, error)

// SiteGetter provides access to connected local or remote sites
type SiteGetter interface {
	// GetSite returns the site matching the provided clusterName
	GetSite(clusterName string) (reversetunnelclient.RemoteSite, error)
}

// RemoteClusterGetter provides access to remote cluster resources
type RemoteClusterGetter interface {
	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)
}

// RouterConfig contains all the dependencies required
// by the Router
type RouterConfig struct {
	// ClusterName indicates which cluster the router is for
	ClusterName string
	// Log is the logger to use
	Log *logrus.Entry
	// AccessPoint is the proxy cache
	RemoteClusterGetter RemoteClusterGetter
	// SiteGetter allows looking up sites
	SiteGetter SiteGetter
	// TracerProvider allows tracers to be created
	TracerProvider oteltrace.TracerProvider

	// serverResolver is used to resolve hosts, used by tests
	serverResolver serverResolverFn
}

// CheckAndSetDefaults ensures the required items were populated
func (c *RouterConfig) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "Router")
	}

	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName must be provided")
	}

	if c.RemoteClusterGetter == nil {
		return trace.BadParameter("RemoteClusterGetter must be provided")
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
	clusterName    string
	log            *logrus.Entry
	clusterGetter  RemoteClusterGetter
	localSite      reversetunnelclient.RemoteSite
	siteGetter     SiteGetter
	tracer         oteltrace.Tracer
	serverResolver serverResolverFn
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
		clusterName:    cfg.ClusterName,
		log:            cfg.Log,
		clusterGetter:  cfg.RemoteClusterGetter,
		localSite:      localSite,
		siteGetter:     cfg.SiteGetter,
		tracer:         cfg.TracerProvider.Tracer("Router"),
		serverResolver: cfg.serverResolver,
	}, nil
}

// DialHost dials the node that matches the provided host, port and cluster. If no matching node
// is found an error is returned. If more than one matching node is found and the cluster networking
// configuration is not set to route to the most recent an error is returned. Also returns teleport version of the
// target server if it's a teleport server
// DELETE IN 14.0: remove returning teleport version, it was needed for compatibility
func (r *Router) DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, clusterName string, accessChecker services.AccessChecker, agentGetter teleagent.Getter, signer agentless.SignerCreator) (_ net.Conn, teleportVersion string, err error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/DialHost",
		oteltrace.WithAttributes(
			attribute.String("host", host),
			attribute.String("port", port),
			attribute.String("cluster", clusterName),
		),
	)
	defer func() {
		if err != nil {
			failedConnectingToNode.Inc()
		}
		span.End()
	}()

	var targetTeleportVersion string
	site := r.localSite
	if clusterName != r.clusterName {
		remoteSite, err := r.getRemoteCluster(ctx, clusterName, accessChecker)
		if err != nil {
			return nil, targetTeleportVersion, trace.Wrap(err)
		}
		site = remoteSite
	}

	span.AddEvent("looking up server")
	target, err := r.serverResolver(ctx, host, port, remoteSite{site})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	span.AddEvent("retrieved target server")

	principals := []string{host}

	var (
		isAgentlessNode bool
		serverID        string
		serverAddr      string
		proxyIDs        []string
	)

	if target != nil {
		isAgentlessNode = target.GetSubKind() == types.SubKindOpenSSHNode
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
				return nil, "", trace.Wrap(err)
			}

			principals = append(principals, h)
		case serverAddr == "" && target.GetUseTunnel():
			serverAddr = reversetunnelclient.LocalNode
		}

		targetTeleportVersion = target.GetTeleportVersion()
	} else {
		if port == "" || port == "0" {
			port = strconv.Itoa(defaults.SSHServerListenPort)
		}

		serverAddr = net.JoinHostPort(host, port)
		r.log.Warnf("server lookup failed: using default=%v", serverAddr)
	}

	// if the node is a registered openssh node, create a signer for auth
	// and don't set agentGetter so a SSH user agent will not be created
	// when connecting to the remote node
	var sshSigner ssh.Signer
	if isAgentlessNode {
		client, err := r.GetSiteClient(ctx, clusterName)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		sshSigner, err = signer(ctx, client)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		agentGetter = nil
	}

	conn, err := site.Dial(reversetunnelclient.DialParams{
		From:                  clientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: serverAddr},
		OriginalClientDstAddr: clientDstAddr,
		GetUserAgent:          agentGetter,
		AgentlessSigner:       sshSigner,
		Address:               host,
		Principals:            principals,
		ServerID:              serverID,
		ProxyIDs:              proxyIDs,
		TeleportVersion:       targetTeleportVersion,
		ConnType:              types.NodeTunnel,
		TargetServer:          target,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return newProxiedMetricConn(conn), targetTeleportVersion, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
	}

	rc, err := r.clusterGetter.GetRemoteCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	return site, nil
}

// site is the minimum interface needed to match servers
// for a reversetunnelclient.RemoteSite. It makes testing easier.
type site interface {
	GetNodes(ctx context.Context, fn func(n services.Node) bool) ([]types.Server, error)
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)
}

// remoteSite is a site implementation that wraps
// a reversetunnelclient.RemoteSite
type remoteSite struct {
	site reversetunnelclient.RemoteSite
}

// GetNodes uses the wrapped sites NodeWatcher to filter nodes
func (r remoteSite) GetNodes(ctx context.Context, fn func(n services.Node) bool) ([]types.Server, error) {
	watcher, err := r.site.NodeWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return watcher.GetNodes(ctx, fn), nil
}

// GetClusterNetworkingConfig uses the wrapped sites cache to retrieve the ClusterNetworkingConfig
func (r remoteSite) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	ap, err := r.site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := ap.GetClusterNetworkingConfig(ctx, opts...)
	return cfg, trace.Wrap(err)
}

// getServer attempts to locate a node matching the provided host and port in
// the provided site.
func getServer(ctx context.Context, host, port string, site site) (types.Server, error) {
	if site == nil {
		return nil, trace.BadParameter("invalid remote site provided")
	}

	strategy := types.RoutingStrategy_UNAMBIGUOUS_MATCH
	if cfg, err := site.GetClusterNetworkingConfig(ctx); err == nil {
		strategy = cfg.GetRoutingStrategy()
	}

	_, err := uuid.Parse(host)
	dialByID := err == nil || utils.IsEC2NodeID(host)

	ips, _ := net.LookupHost(host)

	var unambiguousIDMatch bool
	matches, err := site.GetNodes(ctx, func(server services.Node) bool {
		if unambiguousIDMatch {
			return false
		}

		// if host is a UUID or EC2 ID match only
		// by server name and treat matches as unambiguous
		if dialByID && server.GetName() == host {
			unambiguousIDMatch = true
			return true
		}

		// if the server has connected over a reverse tunnel
		// then match only by hostname
		if server.GetUseTunnel() {
			return host == server.GetHostname()
		}

		ip, nodePort, err := net.SplitHostPort(server.GetAddr())
		if err != nil {
			return false
		}

		if (host == ip || host == server.GetHostname() || slices.Contains(ips, ip)) &&
			(port == "" || port == "0" || port == nodePort) {
			return true
		}

		return false
	})
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, trace.NotFound(teleport.NodeIsAmbiguous)
	case len(matches) == 1:
		server = matches[0]
	}

	if dialByID && server == nil {
		idType := "UUID"
		if utils.IsEC2NodeID(host) {
			idType = "EC2"
		}

		return nil, trace.NotFound("unable to locate node matching %s-like target %s", idType, host)
	}

	return server, nil
}

// DialSite establishes a connection to the auth server in the provided
// cluster. If the clusterName is an empty string then a connection to
// the local auth server will be established.
func (r *Router) DialSite(ctx context.Context, clusterName string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	_, span := r.tracer.Start(
		ctx,
		"router/DialSite",
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer span.End()

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

	return newProxiedMetricConn(conn), trace.Wrap(err)
}

// GetSiteClient returns an auth client for the provided cluster.
func (r *Router) GetSiteClient(ctx context.Context, clusterName string) (auth.ClientI, error) {
	if clusterName == r.clusterName {
		return r.localSite.GetClient()
	}

	site, err := r.siteGetter.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return site.GetClient()
}
