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

package regular

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// PROXYHeaderSigner allows to sign PROXY headers for securely propagating original client IP information
type PROXYHeaderSigner interface {
	SignPROXYHeader(source, destination net.Addr) ([]byte, error)
}

// CertAuthorityGetter allows to get cluster's host CA for verification of signed PROXY headers.
// We define our own version to avoid circular dependencies in multiplexer package (it can't depend on 'services'),
// where this function is used.
type CertAuthorityGetter = func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

// proxySubsys implements an SSH subsystem for proxying listening sockets from
// remote hosts to a proxy client (AKA port mapping)
type proxySubsys struct {
	proxySubsysRequest
	router       *proxy.Router
	ctx          *srv.ServerContext
	log          *logrus.Entry
	closeC       chan error
	proxySigner  PROXYHeaderSigner
	localCluster string
}

// parseProxySubsys looks at the requested subsystem name and returns a fully configured
// proxy subsystem
//
// proxy subsystem name can take the following forms:
//
//	"proxy:host:22"          - standard SSH request to connect to  host:22 on the 1st cluster
//	"proxy:@clustername"        - Teleport request to connect to an auth server for cluster with name 'clustername'
//	"proxy:host:22@clustername" - Teleport request to connect to host:22 on cluster 'clustername'
//	"proxy:host:22@namespace@clustername"
func parseProxySubsysRequest(request string) (proxySubsysRequest, error) {
	log.Debugf("parse_proxy_subsys(%q)", request)
	var (
		clusterName  string
		targetHost   string
		targetPort   string
		paramMessage = fmt.Sprintf("invalid format for proxy request: %q, expected 'proxy:host:port@cluster'", request)
	)
	const prefix = "proxy:"
	// get rid of 'proxy:' prefix:
	if strings.Index(request, prefix) != 0 {
		return proxySubsysRequest{}, trace.BadParameter(paramMessage)
	}
	requestBody := strings.TrimPrefix(request, prefix)
	namespace := apidefaults.Namespace
	parts := strings.Split(requestBody, "@")

	var err error
	switch {
	case len(parts) == 0: // "proxy:"
		return proxySubsysRequest{}, trace.BadParameter(paramMessage)
	case len(parts) == 1: // "proxy:host:22"
		targetHost, targetPort, err = utils.SplitHostPort(parts[0])
		if err != nil {
			return proxySubsysRequest{}, trace.BadParameter(paramMessage)
		}
	case len(parts) == 2: // "proxy:@clustername" or "proxy:host:22@clustername"
		if parts[0] != "" {
			targetHost, targetPort, err = utils.SplitHostPort(parts[0])
			if err != nil {
				return proxySubsysRequest{}, trace.BadParameter(paramMessage)
			}
		}
		clusterName = parts[1]
		if clusterName == "" && targetHost == "" {
			return proxySubsysRequest{}, trace.BadParameter("invalid format for proxy request: missing cluster name or target host in %q", request)
		}
	case len(parts) >= 3: // "proxy:host:22@namespace@clustername"
		clusterName = strings.Join(parts[2:], "@")
		namespace = parts[1]
		targetHost, targetPort, err = utils.SplitHostPort(parts[0])
		if err != nil {
			return proxySubsysRequest{}, trace.BadParameter(paramMessage)
		}
	}

	return proxySubsysRequest{
		namespace:   namespace,
		host:        targetHost,
		port:        targetPort,
		clusterName: clusterName,
	}, nil
}

// parseProxySubsys decodes a proxy subsystem request and sets up a proxy subsystem instance.
// See parseProxySubsysRequest for details on the request format.
func parseProxySubsys(request string, srv *Server, ctx *srv.ServerContext) (*proxySubsys, error) {
	req, err := parseProxySubsysRequest(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subsys, err := newProxySubsys(ctx, srv, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return subsys, nil
}

// proxySubsysRequest encodes proxy subsystem request parameters.
type proxySubsysRequest struct {
	namespace   string
	host        string
	port        string
	clusterName string
}

func (p *proxySubsysRequest) String() string {
	return fmt.Sprintf("host=%v, port=%v, cluster=%v", p.host, p.port, p.clusterName)
}

// SpecifiedPort returns whether the port is set, and it has a non-zero value
func (p *proxySubsysRequest) SpecifiedPort() bool {
	return len(p.port) > 0 && p.port != "0"
}

// SetDefaults sets default values.
func (p *proxySubsysRequest) SetDefaults() {
	if p.namespace == "" {
		p.namespace = apidefaults.Namespace
	}
}

// newProxySubsys is a helper that creates a proxy subsystem from
// a port forwarding request, used to implement ProxyJump feature in proxy
// and reuse the code
func newProxySubsys(ctx *srv.ServerContext, srv *Server, req proxySubsysRequest) (*proxySubsys, error) {
	req.SetDefaults()
	if req.clusterName == "" && ctx.Identity.RouteToCluster != "" {
		log.Debugf("Proxy subsystem: routing user %q to cluster %q based on the route to cluster extension.",
			ctx.Identity.TeleportUser, ctx.Identity.RouteToCluster,
		)
		req.clusterName = ctx.Identity.RouteToCluster
	}
	if req.clusterName != "" && srv.proxyTun != nil {
		checker, err := srv.tunnelWithAccessChecker(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if _, err := checker.GetSite(req.clusterName); err != nil {
			return nil, trace.BadParameter("invalid format for proxy request: unknown cluster %q", req.clusterName)
		}
	}
	log.Debugf("newProxySubsys(%v).", req)
	return &proxySubsys{
		proxySubsysRequest: req,
		ctx:                ctx,
		log: logrus.WithFields(logrus.Fields{
			teleport.ComponentKey:    teleport.ComponentSubsystemProxy,
			teleport.ComponentFields: map[string]string{},
		}),
		closeC:       make(chan error),
		router:       srv.router,
		proxySigner:  srv.proxySigner,
		localCluster: ctx.ClusterName,
	}, nil
}

func (t *proxySubsys) String() string {
	return fmt.Sprintf("proxySubsys(cluster=%s/%s, host=%s, port=%s)",
		t.namespace, t.clusterName, t.host, t.port)
}

// Start is called by Golang's ssh when it needs to engage this sybsystem (typically to establish
// a mapping connection between a client & remote node we're proxying to)
func (t *proxySubsys) Start(ctx context.Context, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, serverContext *srv.ServerContext) error {
	// once we start the connection, update logger to include component fields
	t.log = logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: teleport.ComponentSubsystemProxy,
		teleport.ComponentFields: map[string]string{
			"src":       sconn.RemoteAddr().String(),
			"dst":       sconn.LocalAddr().String(),
			"subsystem": t.String(),
		},
	})
	t.log.Debugf("Starting subsystem")

	clientAddr := sconn.RemoteAddr()

	// connect to a site's auth server
	if t.host == "" {
		return t.proxyToSite(ctx, ch, t.clusterName, clientAddr, sconn.LocalAddr())
	}

	// connect to a server
	return t.proxyToHost(ctx, ch, clientAddr, sconn.LocalAddr())
}

// proxyToSite establishes a proxy connection from the connected SSH client to the
// auth server of the requested remote site
func (t *proxySubsys) proxyToSite(ctx context.Context, ch ssh.Channel, clusterName string, clientSrcAddr, clientDstAddr net.Addr) error {
	t.log.Debugf("Connecting from cluster %q to site: %q", t.localCluster, clusterName)

	conn, err := t.router.DialSite(ctx, clusterName, clientSrcAddr, clientDstAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	t.log.Infof("Connected to cluster %v at %v", clusterName, conn.RemoteAddr())

	go func() {
		t.close(utils.ProxyConn(ctx, ch, conn))
	}()
	return nil
}

// proxyToHost establishes a proxy connection from the connected SSH client to the
// requested remote node (t.host:t.port) via the given site
func (t *proxySubsys) proxyToHost(ctx context.Context, ch ssh.Channel, clientSrcAddr, clientDstAddr net.Addr) error {
	t.log.Debugf("proxy connecting to host=%v port=%v, exact port=%v", t.host, t.port, t.SpecifiedPort())

	authClient, err := t.router.GetSiteClient(ctx, t.localCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	identity := t.ctx.Identity

	signer := agentless.SignerFromSSHIdentity(identity.UnmappedIdentity, authClient, t.clusterName, identity.TeleportUser)

	aGetter := t.ctx.StartAgentChannel
	conn, err := t.router.DialHost(ctx, clientSrcAddr, clientDstAddr, t.host, t.port, t.clusterName, t.ctx.Identity.AccessChecker, aGetter, signer)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		t.close(utils.ProxyConn(ctx, ch, conn))
	}()

	return nil
}

func (t *proxySubsys) close(err error) {
	t.closeC <- err
}

func (t *proxySubsys) Wait() error {
	return <-t.closeC
}
