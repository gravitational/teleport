package proxy

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

type RouterConfig struct {
	ClusterName    string
	Log            *logrus.Entry
	AccessPoint    auth.ReadProxyAccessPoint
	LocalSite      reversetunnel.RemoteSite
	Tunnel         reversetunnel.Tunnel
	TracerProvider oteltrace.TracerProvider
}

func (c *RouterConfig) CheckAndSetDefaults() error {
	if c.ClusterName == "" {
		return trace.BadParameter("clusterName must be provided")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("accessPoint must be provided")
	}

	if c.LocalSite == nil {
		return trace.BadParameter("localSite must be provided")
	}

	if c.Tunnel == nil {
		return trace.BadParameter("tunnel must be provided")
	}

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.DefaultProvider()
	}

	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "Router")
	}

	return nil
}

type Router struct {
	clusterName string
	log         *logrus.Entry
	accessPoint auth.ReadProxyAccessPoint
	localSite   reversetunnel.RemoteSite
	tunnel      reversetunnel.Tunnel
	tracer      oteltrace.Tracer
}

func NewRouter(cfg RouterConfig) (*Router, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Router{
		clusterName: cfg.ClusterName,
		log:         cfg.Log,
		accessPoint: cfg.AccessPoint,
		localSite:   cfg.LocalSite,
		tunnel:      cfg.Tunnel,
		tracer:      cfg.TracerProvider.Tracer("Router"),
	}, nil

}

func (r *Router) DialHost(ctx context.Context, from net.Addr, host, port, clusterName string, accessChecker services.AccessChecker, agentGetter teleagent.Getter) (net.Conn, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/DialHost",
		oteltrace.WithAttributes(
			attribute.String("host", host),
			attribute.String("port", port),
			attribute.String("site", clusterName),
		),
	)
	defer span.End()

	site := r.localSite
	if clusterName != r.clusterName {
		remoteSite, err := r.getRemoteCluster(ctx, clusterName, accessChecker)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		site = remoteSite
	}

	span.AddEvent("looking up server")
	target, err := getServer(ctx, host, port, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("retrieved target server")

	principals := []string{host}

	var (
		serverID   string
		serverAddr string
		proxyIDs   []string
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
			serverAddr = reversetunnel.LocalNode
		}
	} else {
		if port == "" || port == "0" {
			port = strconv.Itoa(defaults.SSHServerListenPort)
		}

		serverAddr = net.JoinHostPort(host, port)
		r.log.Warn("server lookup failed: using default=%v", serverAddr)
	}

	conn, err := site.Dial(reversetunnel.DialParams{
		From:         from,
		To:           &utils.NetAddr{AddrNetwork: "tcp", Addr: serverAddr},
		GetUserAgent: agentGetter,
		Address:      host,
		ServerID:     serverID,
		ProxyIDs:     proxyIDs,
		Principals:   principals,
		ConnType:     types.NodeTunnel,
	})

	return conn, trace.Wrap(err)
}

func (r *Router) getRemoteCluster(ctx context.Context, clusterName string, checker services.AccessChecker) (reversetunnel.RemoteSite, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/getRemoteCluster",
		oteltrace.WithAttributes(
			attribute.String("site", clusterName),
		),
	)
	defer span.End()

	site, err := r.tunnel.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := r.accessPoint.GetRemoteCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	return site, nil
}

func getServer(ctx context.Context, host, port string, site reversetunnel.RemoteSite) (types.Server, error) {
	strategy := types.RoutingStrategy_UNAMBIGUOUS_MATCH
	ap, err := site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg, err := ap.GetClusterNetworkingConfig(ctx); err == nil {
		strategy = cfg.GetRoutingStrategy()
	}

	_, err = uuid.Parse(host)
	dialByID := err == nil || utils.IsEC2NodeID(host)

	ips, _ := net.LookupHost(host)

	var unambiguousIDMatch bool
	watcher, err := site.NodeWatcher()
	if err != nil {
		return nil, err
	}

	matches := watcher.GetNodes(func(server services.Node) bool {
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

func (r *Router) DialSite(ctx context.Context, clusterName string) (net.Conn, error) {
	ctx, span := r.tracer.Start(
		ctx,
		"router/DialSite",
		oteltrace.WithAttributes(
			attribute.String("site", clusterName),
		),
	)
	defer span.End()

	switch {
	case clusterName == r.clusterName, clusterName == "":
		conn, err := r.localSite.DialAuthServer()
		return conn, trace.Wrap(err)
	case clusterName != "":
		site, err := r.tunnel.GetSite(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := site.DialAuthServer()
		return conn, trace.Wrap(err)
	}

	return nil, trace.BadParameter("invalid clusterName provided")
}
