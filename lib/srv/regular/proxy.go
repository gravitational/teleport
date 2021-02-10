/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package regular

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var proxiedSessions = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: teleport.MetricProxySSHSessions,
		Help: "Number of active sessions through this proxy",
	},
)

func init() {
	prometheus.MustRegister(proxiedSessions)
}

// proxySubsys implements an SSH subsystem for proxying listening sockets from
// remote hosts to a proxy client (AKA port mapping)
type proxySubsys struct {
	proxySubsysRequest
	srv       *Server
	ctx       *srv.ServerContext
	log       *logrus.Entry
	closeC    chan struct{}
	error     error
	closeOnce sync.Once
}

// parseProxySubsys looks at the requested subsystem name and returns a fully configured
// proxy subsystem
//
// proxy subsystem name can take the following forms:
//  "proxy:host:22"          - standard SSH request to connect to  host:22 on the 1st cluster
//  "proxy:@clustername"        - Teleport request to connect to an auth server for cluster with name 'clustername'
//  "proxy:host:22@clustername" - Teleport request to connect to host:22 on cluster 'clustername'
//  "proxy:host:22@namespace@clustername"
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
	namespace := defaults.Namespace
	var err error
	parts := strings.Split(requestBody, "@")
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

// SetDefaults sets default values.
func (p *proxySubsysRequest) SetDefaults() {
	if p.namespace == "" {
		p.namespace = defaults.Namespace
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
		_, err := srv.tunnelWithRoles(ctx).GetSite(req.clusterName)
		if err != nil {
			return nil, trace.BadParameter("invalid format for proxy request: unknown cluster %q", req.clusterName)
		}
	}
	log.Debugf("newProxySubsys(%v).", req)
	return &proxySubsys{
		proxySubsysRequest: req,
		ctx:                ctx,
		srv:                srv,
		log: logrus.WithFields(logrus.Fields{
			trace.Component:       teleport.ComponentSubsystemProxy,
			trace.ComponentFields: map[string]string{},
		}),
		closeC: make(chan struct{}),
	}, nil
}

func (t *proxySubsys) String() string {
	return fmt.Sprintf("proxySubsys(cluster=%s/%s, host=%s, port=%s)",
		t.namespace, t.clusterName, t.host, t.port)
}

// Start is called by Golang's ssh when it needs to engage this sybsystem (typically to establish
// a mapping connection between a client & remote node we're proxying to)
func (t *proxySubsys) Start(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	// once we start the connection, update logger to include component fields
	t.log = logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentSubsystemProxy,
		trace.ComponentFields: map[string]string{
			"src": sconn.RemoteAddr().String(),
			"dst": sconn.LocalAddr().String(),
		},
	})
	t.log.Debugf("Starting subsystem")

	var (
		site       reversetunnel.RemoteSite
		err        error
		tunnel     = t.srv.tunnelWithRoles(ctx)
		clientAddr = sconn.RemoteAddr()
	)
	// did the client pass us a true client IP ahead of time via an environment variable?
	// (usually the web client would do that)
	trueClientIP, ok := ctx.GetEnv(sshutils.TrueClientAddrVar)
	if ok {
		a, err := utils.ParseAddr(trueClientIP)
		if err == nil {
			clientAddr = a
		}
	}
	// get the cluster by name:
	if t.clusterName != "" {
		site, err = tunnel.GetSite(t.clusterName)
		if err != nil {
			t.log.Warn(err)
			return trace.Wrap(err)
		}
	}
	// connecting to a specific host:
	if t.host != "" {
		// no site given? use the 1st one:
		if site == nil {
			sites, err := tunnel.GetSites()
			if err != nil {
				return trace.Wrap(err)
			}
			if len(sites) == 0 {
				t.log.Error("Not connected to any remote clusters")
				return trace.NotFound("no connected sites")
			}
			site = sites[0]
			t.clusterName = site.GetName()
			t.log.Debugf("Cluster not specified. connecting to default='%s'", site.GetName())
		}
		return t.proxyToHost(ctx, site, clientAddr, ch)
	}
	// connect to a site's auth server:
	return t.proxyToSite(ctx, site, clientAddr, ch)
}

// proxyToSite establishes a proxy connection from the connected SSH client to the
// auth server of the requested remote site
func (t *proxySubsys) proxyToSite(
	ctx *srv.ServerContext, site reversetunnel.RemoteSite, remoteAddr net.Addr, ch ssh.Channel) error {

	conn, err := site.DialAuthServer()
	if err != nil {
		return trace.Wrap(err)
	}
	t.log.Infof("Connected to auth server: %v", conn.RemoteAddr())

	proxiedSessions.Inc()
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer ch.Close()
		_, err = io.Copy(ch, conn)
	}()
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer conn.Close()
		_, err = io.Copy(conn, ch)

	}()

	return nil
}

// proxyToHost establishes a proxy connection from the connected SSH client to the
// requested remote node (t.host:t.port) via the given site
func (t *proxySubsys) proxyToHost(
	ctx *srv.ServerContext, site reversetunnel.RemoteSite, remoteAddr net.Addr, ch ssh.Channel) error {
	//
	// first, lets fetch a list of servers at the given site. this allows us to
	// match the given "host name" against node configuration (their 'nodename' setting)
	//
	// but failing to fetch the list of servers is also OK, we'll use standard
	// network resolution (by IP or DNS)
	//
	var (
		servers []services.Server
		err     error
	)
	localCluster, _ := t.srv.authService.GetClusterName()
	// going to "local" CA? lets use the caching 'auth service' directly and avoid
	// hitting the reverse tunnel link (it can be offline if the CA is down)
	if site.GetName() == localCluster.GetName() {
		servers, err = t.srv.authService.GetNodes(t.namespace, resource.SkipValidation())
		if err != nil {
			t.log.Warn(err)
		}
	} else {
		// "remote" CA? use a reverse tunnel to talk to it:
		siteClient, err := site.CachingAccessPoint()
		if err != nil {
			t.log.Warn(err)
		} else {
			servers, err = siteClient.GetNodes(t.namespace, resource.SkipValidation())
			if err != nil {
				t.log.Warn(err)
			}
		}
	}

	// if port is 0, it means the client wants us to figure out
	// which port to use
	specifiedPort := len(t.port) > 0 && t.port != "0"
	ips, _ := net.LookupHost(t.host)
	t.log.Debugf("proxy connecting to host=%v port=%v, exact port=%v", t.host, t.port, specifiedPort)

	// check if hostname is a valid uuid.  If it is, we will preferentially match
	// by node ID over node hostname.
	hostIsUUID := uuid.Parse(t.host) != nil

	// enumerate and try to find a server with self-registered with a matching name/IP:
	var server services.Server
	matches := 0
	for i := range servers {
		// If the host parameter is a UUID and it matches the Node ID,
		// treat this as an unambiguous match.
		if hostIsUUID && servers[i].GetName() == t.host {
			server = servers[i]
			matches = 1
			break
		}
		// If the server has connected over a reverse tunnel, match only on hostname.
		if servers[i].GetUseTunnel() {
			if t.host == servers[i].GetHostname() {
				server = servers[i]
				matches++
			}
			continue
		}

		ip, port, err := net.SplitHostPort(servers[i].GetAddr())
		if err != nil {
			t.log.Errorf("Failed to parse address %q: %v.", servers[i].GetAddr(), err)
			continue
		}
		if t.host == ip || t.host == servers[i].GetHostname() || utils.SliceContainsStr(ips, ip) {
			if !specifiedPort || t.port == port {
				server = servers[i]
				matches++
				continue
			}
		}
	}

	// if we matched more than one server, then the target was ambiguous.
	if matches > 1 {
		return trace.NotFound(teleport.NodeIsAmbiguous)
	}

	// If we matched zero nodes but hostname is a UUID then it isn't sane
	// to fallback to dns based resolution.  This has the unfortunate
	// consequence of preventing users from calling OpenSSH nodes which
	// happen to use hostnames which are also valid UUIDs.  This restriction
	// is necessary in order to protect users attempting to connect to a
	// node by UUID from being re-routed to an unintended target if the node
	// is offline.  This restriction can be lifted if we decide to move to
	// explicit UUID based resoltion in the future.
	if hostIsUUID && matches < 1 {
		return trace.NotFound("unable to locate node matching uuid-like target %s", t.host)
	}

	// Create a slice of principals that will be added into the host certificate.
	// Here t.host is either an IP address or a DNS name as the user requested.
	principals := []string{t.host}

	// Used to store the server ID (hostUUID.clusterName) of a Teleport node.
	var serverID string

	// Resolve the IP address to dial to because the hostname may not be
	// DNS resolvable.
	var serverAddr string
	if server != nil {
		// Add hostUUID.clusterName to list of principals.
		serverID = fmt.Sprintf("%v.%v", server.GetName(), t.clusterName)
		principals = append(principals, serverID)

		// Add IP address (if it exists) of the node to list of principals.
		serverAddr = server.GetAddr()
		if serverAddr != "" {
			host, _, err := net.SplitHostPort(serverAddr)
			if err != nil {
				return trace.Wrap(err)
			}
			principals = append(principals, host)
		} else if server.GetUseTunnel() {
			serverAddr = reversetunnel.LocalNode
		}
	} else {
		if !specifiedPort {
			t.port = strconv.Itoa(defaults.SSHServerListenPort)
		}
		serverAddr = net.JoinHostPort(t.host, t.port)
		t.log.Warnf("server lookup failed: using default=%v", serverAddr)
	}

	// Pass the agent along to the site. If the proxy is in recording mode, this
	// agent is used to perform user authentication. Pass the DNS name to the
	// dialer as well so the forwarding proxy can generate a host certificate
	// with the correct hostname).
	toAddr := &utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        serverAddr,
	}
	conn, err := site.Dial(reversetunnel.DialParams{
		From:         remoteAddr,
		To:           toAddr,
		GetUserAgent: t.ctx.StartAgentChannel,
		Address:      t.host,
		ServerID:     serverID,
		Principals:   principals,
		ConnType:     services.NodeTunnel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// this custom SSH handshake allows SSH proxy to relay the client's IP
	// address to the SSH server
	t.doHandshake(remoteAddr, ch, conn)

	proxiedSessions.Inc()
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer ch.Close()
		_, err = io.Copy(ch, conn)
	}()
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer conn.Close()
		_, err = io.Copy(conn, ch)
	}()

	return nil
}

func (t *proxySubsys) close(err error) {
	t.closeOnce.Do(func() {
		proxiedSessions.Dec()
		t.error = err
		close(t.closeC)
	})
}

func (t *proxySubsys) Wait() error {
	<-t.closeC
	return t.error
}

// doHandshake allows a proxy server to send additional information (client IP)
// to an SSH server before establishing a bridge
func (t *proxySubsys) doHandshake(clientAddr net.Addr, clientConn io.ReadWriter, serverConn io.ReadWriter) {
	// on behalf of a client ask the server for it's version:
	buff := make([]byte, sshutils.MaxVersionStringBytes)
	n, err := serverConn.Read(buff)
	if err != nil {
		t.log.Error(err)
		return
	}
	// chop off extra unused bytes at the end of the buffer:
	buff = buff[:n]

	// is that a Teleport server?
	if bytes.HasPrefix(buff, []byte(sshutils.SSHVersionPrefix)) {
		// if we're connecting to a Teleport SSH server, send our own "handshake payload"
		// message, along with a client's IP:
		hp := &sshutils.HandshakePayload{
			ClientAddr: clientAddr.String(),
		}
		payloadJSON, err := json.Marshal(hp)
		if err != nil {
			t.log.Error(err)
		} else {
			// send a JSON payload sandwitched between 'teleport proxy signature' and 0x00:
			payload := fmt.Sprintf("%s%s\x00", sshutils.ProxyHelloSignature, payloadJSON)
			_, err = serverConn.Write([]byte(payload))
			if err != nil {
				t.log.Error(err)
			}
		}
	}
	// forwrd server's response to the client:
	_, err = clientConn.Write(buff)
	if err != nil {
		t.log.Error(err)
	}
}
