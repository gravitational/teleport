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

package reversetunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

// parseDialReq parses the dial request. Is backward compatible with legacy
// payload.
func parseDialReq(payload []byte) *sshutils.DialReq {
	var req sshutils.DialReq
	err := json.Unmarshal(payload, &req)
	if err != nil {
		// For backward compatibility, if the request is not a *DialReq, it is just
		// a raw string with the target host as the payload.
		return &sshutils.DialReq{
			Address: string(payload),
		}
	}
	return &req
}

// transport is used to build a connection to the target host.
type transport struct {
	component    string
	logger       *slog.Logger
	closeContext context.Context
	authClient   authclient.ProxyAccessPoint
	authServers  []string
	channel      ssh.Channel
	requestCh    <-chan *ssh.Request

	// localClusterName is the name of the cluster that the transport code is
	// running in.
	localClusterName string

	// kubeDialAddr is the address of the Kubernetes proxy.
	kubeDialAddr utils.NetAddr

	// sconn is a SSH connection to the remote host. Used for dial back nodes.
	sconn sshutils.Conn

	// reverseTunnelServer holds all reverse tunnel connections.
	reverseTunnelServer reversetunnelclient.Server

	// server is either an SSH or application server. It can handle a connection
	// (perform handshake and handle request).
	server ServerHandler

	// emitter is an audit event emitter.
	emitter apievents.Emitter

	// proxySigner is used to sign PROXY headers and securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner

	// forwardClientAddress indicates whether we should take into account ClientSrcAddr/ClientDstAddr on incoming
	// dial request. If false, we ignore those fields and take address from the parent ssh connection. It allows
	// preventing users connecting to the proxy tunnel listener spoofing their address; but we are still able to
	// correctly propagate client address in reverse tunnel agents of nodes/services.
	forwardClientAddress bool

	// trackUserConnection is an optional mechanism used to count active user sessions.
	trackUserConnection func() (release func())
}

// start will start the transporting data over the tunnel. This function will
// typically run in the agent or reverse tunnel server. It's used to establish
// connections from remote clusters into the main cluster or for remote nodes
// that have no direct network access to the cluster.
//
// TODO(awly): unit test this
func (p *transport) start() {
	defer p.channel.Close()

	// Always push space into stderr to make sure the caller can always
	// safely call read (stderr) without blocking. This stderr is only used
	// to request proxying of TCP/IP via reverse tunnel.
	fmt.Fprint(p.channel.Stderr(), " ")

	// Wait for a request to come in from the other side telling the server
	// where to dial to.
	var req *ssh.Request
	select {
	case <-p.closeContext.Done():
		return
	case req = <-p.requestCh:
		if req == nil {
			return
		}
	case <-time.After(apidefaults.DefaultIOTimeout):
		p.logger.WarnContext(p.closeContext, "Transport request failed: timed out waiting for request")
		return
	}

	// Parse and extract the dial request from the client.
	dreq := parseDialReq(req.Payload)
	if err := dreq.CheckAndSetDefaults(); err != nil {
		p.reply(req, false, []byte(err.Error()))
		return
	}

	if !p.forwardClientAddress {
		// This shouldn't happen in normal operation. Either malicious user or misconfigured client.
		if dreq.ClientSrcAddr != "" || dreq.ClientDstAddr != "" {
			const msg = "Received unexpected dial request with client source address and " +
				"client destination address populated, when they should be empty."
			p.logger.WarnContext(p.closeContext, msg,
				"client_src_addr", dreq.ClientSrcAddr,
				"client_dest_addr", dreq.ClientDstAddr,
			)
		}

		// Make sure address fields are overwritten.
		if p.sconn != nil {
			dreq.ClientSrcAddr = p.sconn.RemoteAddr().String()
			dreq.ClientDstAddr = p.sconn.LocalAddr().String()
		} else {
			dreq.ClientSrcAddr = ""
			dreq.ClientDstAddr = ""
		}
	}

	p.logger.DebugContext(p.closeContext, "Received out-of-band proxy transport request",
		"target_address", dreq.Address,
		"target_server_id", dreq.ServerID,
		"client_addr", dreq.ClientSrcAddr,
	)

	// directAddress will hold the address of the node to dial to, if we don't
	// have a tunnel for it.
	var directAddress string

	// Handle special non-resolvable addresses first.
	switch dreq.Address {
	// Connect to an Auth Server.
	case reversetunnelclient.RemoteAuthServer:
		if len(p.authServers) == 0 {
			p.logger.ErrorContext(p.closeContext, "connection rejected: no auth servers configured")
			p.reply(req, false, []byte("no auth servers configured"))

			return
		}

		directAddress = utils.ChooseRandomString(p.authServers)
	// Connect to the Kubernetes proxy.
	case reversetunnelclient.LocalKubernetes:
		switch p.component {
		case teleport.ComponentReverseTunnelServer:
			p.reply(req, false, []byte("connection rejected: no remote kubernetes proxy"))
			return
		case teleport.ComponentKube:
			// kubernetes_service can directly handle the connection, via
			// p.server.
			if p.server == nil {
				p.reply(req, false, []byte("connection rejected: server missing"))
				return
			}
			if p.sconn == nil {
				p.reply(req, false, []byte("connection rejected: server connection missing"))
				return
			}
			if err := req.Reply(true, []byte("Connected.")); err != nil {
				p.logger.ErrorContext(p.closeContext, "Failed responding OK to request",
					"request_type", req.Type,
					"error", err,
				)
				return
			}

			p.logger.DebugContext(p.closeContext, "Handing off connection to a local kubernetes service")

			// If dreq has ClientSrcAddr we wrap connection
			var clientConn net.Conn = sshutils.NewChConn(p.sconn, p.channel)
			src, err := utils.ParseAddr(dreq.ClientSrcAddr)
			if err == nil {
				clientConn = utils.NewConnWithSrcAddr(clientConn, getTCPAddr(src))
			}
			p.server.HandleConnection(clientConn)
			return
		default:
			// This must be a proxy.
			// If Kubernetes endpoint is not configured, reject the connection.
			if p.kubeDialAddr.IsEmpty() {
				p.reply(req, false, []byte("connection rejected: configure kubernetes proxy for this cluster."))
				return
			}
			p.logger.DebugContext(p.closeContext, "Forwarding connection to kubernetes proxy", "kube_proxy_addr", p.kubeDialAddr.Addr)
			directAddress = p.kubeDialAddr.Addr
		}

	// LocalNode requests are for the single server running in the agent pool.
	case reversetunnelclient.LocalNode, reversetunnelclient.LocalWindowsDesktop:
		// Transport is allocated with both teleport.ComponentReverseTunnelAgent
		// and teleport.ComponentReverseTunnelServer. However, dialing to this address
		// only makes sense when running within a teleport.ComponentReverseTunnelAgent.
		if p.component == teleport.ComponentReverseTunnelServer {
			p.reply(req, false, []byte("connection rejected: no local node"))
			return
		}

		if p.server != nil {
			if p.sconn == nil {
				p.logger.DebugContext(p.closeContext, "Connection rejected: server connection missing")
				p.reply(req, false, []byte("connection rejected: server connection missing"))
				return
			}

			if err := req.Reply(true, []byte("Connected.")); err != nil {
				p.logger.ErrorContext(p.closeContext, "Failed responding OK to request",
					"request_type", req.Type,
					"error", err,
				)
				return
			}

			p.logger.DebugContext(p.closeContext, "Handing off connection to a local service.", "conn_type", dreq.ConnType)

			// If dreq has ClientSrcAddr we wrap connection
			var clientConn net.Conn = sshutils.NewChConn(p.sconn, p.channel)
			src, err := utils.ParseAddr(dreq.ClientSrcAddr)
			if err == nil {
				clientConn = utils.NewConnWithSrcAddr(clientConn, getTCPAddr(src))
			}
			p.server.HandleConnection(clientConn)
			return
		}
		// If this is a proxy and not an SSH node, try finding an inbound
		// tunnel from the SSH node by dreq.ServerID. We'll need to forward
		// dreq.Address as well.
		directAddress = dreq.Address

		if p.trackUserConnection != nil {
			defer p.trackUserConnection()()
		}
	default:
		// Not a special address; could be empty.
		directAddress = dreq.Address
	}

	// Get a connection to the target address. If a tunnel exists with matching
	// search names, connection over the tunnel is returned. Otherwise a direct
	// net.Dial is performed.
	conn, useTunnel, err := p.getConn(directAddress, dreq)
	if err != nil {
		errorMessage := fmt.Sprintf("connection rejected: %v", err)
		fmt.Fprint(p.channel.Stderr(), errorMessage)
		p.reply(req, false, []byte(errorMessage))
		return
	}

	var clientSrc, clientDst net.Addr
	src, err := utils.ParseAddr(dreq.ClientSrcAddr)
	if err == nil {
		clientSrc = src
	}
	dst, err := utils.ParseAddr(dreq.ClientDstAddr)
	if err == nil {
		clientDst = dst
	}
	var signedHeader []byte
	if shouldSendSignedPROXYHeader(p.proxySigner, useTunnel, dreq.IsAgentlessNode, clientSrc, clientDst) {
		signedHeader, err = p.proxySigner.SignPROXYHeader(clientSrc, clientDst)
		if err != nil {
			errorMessage := fmt.Sprintf("connection rejected - could not create signed PROXY header: %v", err)
			fmt.Fprint(p.channel.Stderr(), errorMessage)
			p.reply(req, false, []byte(errorMessage))
			return
		}
	}

	// Dial was successful.
	if err := req.Reply(true, []byte("Connected.")); err != nil {
		p.logger.ErrorContext(p.closeContext, "Failed responding OK to request",
			"request_type", req.Type,
			"error", err,
		)
		if err := conn.Close(); err != nil {
			p.logger.ErrorContext(p.closeContext, "Failed closing connection", "error", err)
		}
		return
	}
	p.logger.DebugContext(p.closeContext, "Successfully dialed to target, starting to proxy",
		"target_addr", dreq.Address,
		"target_server_id", dreq.ServerID,
	)

	// Start processing channel requests. Pass in a context that wraps the passed
	// in context with a context that closes when this function returns to
	// mitigate a goroutine leak.
	ctx, cancel := context.WithCancel(p.closeContext)
	defer cancel()
	go p.handleChannelRequests(ctx, useTunnel)

	errorCh := make(chan error, 2)

	if len(signedHeader) > 0 {
		_, err = conn.Write(signedHeader)
		if err != nil {
			p.logger.ErrorContext(p.closeContext, "Could not write PROXY header to the connection", "error", err)
			if err := conn.Close(); err != nil {
				p.logger.ErrorContext(p.closeContext, "Failed closing connection", "error", err)
			}
			return
		}
	}

	go func() {
		// Make sure that we close the client connection on a channel
		// close, otherwise the other goroutine would never know
		// as it will block on read from the connection.
		defer conn.Close()
		_, err := io.Copy(conn, p.channel)
		errorCh <- err
	}()

	go func() {
		_, err := io.Copy(p.channel, conn)
		errorCh <- err
	}()

	// wait for both io.Copy goroutines to finish, or for
	// the context to be canceled.
	for i := 0; i < 2; i++ {
		select {
		case <-errorCh:
		case <-p.closeContext.Done():
			p.logger.WarnContext(p.closeContext, "Proxy transport failed: closing context")
			return
		}
	}
}

// handleChannelRequests processes client requests from the reverse tunnel
// server.
func (p *transport) handleChannelRequests(closeContext context.Context, useTunnel bool) {
	for {
		select {
		case req := <-p.requestCh:
			if req == nil {
				return
			}
			switch req.Type {
			case sshutils.ConnectionTypeRequest:
				p.reply(req, useTunnel, nil)
			default:
				p.reply(req, false, nil)
			}
		case <-closeContext.Done():
			return
		}
	}
}

// getConn checks if the local site holds a connection to the target host,
// and if it does, attempts to dial through the tunnel. Otherwise directly
// dials to host.
func (p *transport) getConn(addr string, r *sshutils.DialReq) (net.Conn, bool, error) {
	// This function doesn't attempt to dial if a host with one of the
	// search names is not registered. It's a fast check.
	p.logger.DebugContext(p.closeContext, "Attempting to dial server through tunnel", "target_server_id", r.ServerID)
	conn, err := p.tunnelDial(r)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, false, trace.Wrap(err)
		}

		// Connections to applications (including Okta applications) and databases should never occur over
		// a direct dial, return right away.
		switch r.ConnType {
		case types.AppTunnel:
			return nil, false, trace.ConnectionProblem(err, reversetunnelclient.NoApplicationTunnel)
		case types.OktaTunnel:
			return nil, false, trace.ConnectionProblem(err, reversetunnelclient.NoOktaTunnel)
		case types.DatabaseTunnel:
			return nil, false, trace.ConnectionProblem(err, reversetunnelclient.NoDatabaseTunnel)
		}

		errTun := err
		p.logger.DebugContext(p.closeContext, "Attempting to dial server directly", "target_addr", addr)
		conn, err = p.directDial(addr)
		if err != nil {
			return nil, false, trace.ConnectionProblem(err, "failed dialing through tunnel (%v) or directly (%v)", errTun, err)
		}

		p.logger.DebugContext(p.closeContext, "Returning direct dialed connection", "target_addr", addr)

		// Requests to get a connection to the remote auth server do not provide a ConnType,
		// and since an empty ConnType is converted to [types.NodeTunnel] in CheckAndSetDefaults,
		// we need to check the address of the request to prevent auth connections from being
		// counted as a proxied ssh session.
		if r.ConnType == types.NodeTunnel && r.Address != reversetunnelclient.RemoteAuthServer {
			return proxy.NewProxiedMetricConn(conn), false, nil
		}

		return conn, false, nil
	}

	p.logger.DebugContext(p.closeContext, "Returning connection to server dialed through tunnel", "target_server_id", r.ServerID)

	if r.ConnType == types.NodeTunnel {
		return proxy.NewProxiedMetricConn(conn), true, nil
	}

	return conn, true, nil
}

// tunnelDial looks up the search names in the local site for a matching tunnel
// connection. If a connection exists, it's used to dial through the tunnel.
func (p *transport) tunnelDial(r *sshutils.DialReq) (net.Conn, error) {
	// Extract the local site from the tunnel server. If no tunnel server
	// exists, then exit right away this code may be running outside of a
	// remote site.
	if p.reverseTunnelServer == nil {
		return nil, trace.NotFound("not found")
	}
	cluster, err := p.reverseTunnelServer.GetSite(p.localClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCluster, ok := cluster.(*localSite)
	if !ok {
		return nil, trace.BadParameter("did not find local cluster, found %T", cluster)
	}
	conn, err := localCluster.dialTunnel(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (p *transport) reply(req *ssh.Request, ok bool, msg []byte) {
	if !ok {
		p.logger.DebugContext(p.closeContext, "Non-ok reply to request", "request_type", req.Type, "error", string(msg))
	}
	if err := req.Reply(ok, msg); err != nil {
		p.logger.WarnContext(p.closeContext, "Failed sending reply to request", "request_type", req.Type, "error", err)
	}
}

// directDial attempts to directly dial to the target host.
func (p *transport) directDial(addr string) (net.Conn, error) {
	if addr == "" {
		return nil, trace.BadParameter("no address to dial")
	}

	d := net.Dialer{
		Timeout: apidefaults.DefaultIOTimeout,
	}
	conn, err := d.DialContext(p.closeContext, "tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// getTCPAddr converts net.Addr to *net.TCPAddr.
//
// SSH package requires net.TCPAddr for source-address check.
func getTCPAddr(addr net.Addr) *net.TCPAddr {
	ap, err := netip.ParseAddrPort(addr.String())
	if err != nil {
		return nil
	}
	return net.TCPAddrFromAddrPort(ap)
}
