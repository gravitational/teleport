/*
Copyright 2019 Gravitational, Inc.

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

package reversetunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// TunnelAuthDialer connects to the Auth Server through the reverse tunnel.
type TunnelAuthDialer struct {
	// ProxyAddr is the address of the proxy
	ProxyAddr string
	// ClientConfig is SSH tunnel client config
	ClientConfig *ssh.ClientConfig
}

// DialContext dials auth server via SSH tunnel
func (t *TunnelAuthDialer) DialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	// Connect to the reverse tunnel server.
	dialer := proxy.DialerFromEnvironment(t.ProxyAddr)
	sconn, err := dialer.Dial("tcp", t.ProxyAddr, t.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build a net.Conn over the tunnel. Make this an exclusive connection:
	// close the net.Conn as well as the channel upon close.
	conn, _, err := connectProxyTransport(sconn.Conn, &dialReq{
		Address:   RemoteAuthServer,
		Exclusive: true,
	})
	if err != nil {
		err2 := sconn.Close()
		return nil, trace.NewAggregate(err, err2)
	}
	return conn, nil
}

// dialReq is a request for the address to connect to. Supports special
// non-resolvable addresses and search names if connection over a tunnel.
type dialReq struct {
	// Address is the target host to make a connection to. Address may be a
	// special non-resolvable address like @remote-auth-server.
	Address string `json:"address,omitempty"`

	// ServerID is the hostUUID.clusterName of th enode. ServerID is used when
	// dialing through a tunnel.
	ServerID string `json:"server_id,omitempty"`

	// Exclusive indicates if the connection should be closed or only the
	// channel upon calling close on the net.Conn.
	Exclusive bool `json:"exclusive"`
}

// parseDialReq parses the dial request. Is backward compatible with legacy
// payload.
func parseDialReq(payload []byte) *dialReq {
	var req dialReq
	err := json.Unmarshal(payload, &req)
	if err != nil {
		// For backward compatibility, if the request is not a *dialReq, it is just
		// a raw string with the target host as the payload.
		return &dialReq{
			Address: string(payload),
		}
	}
	return &req
}

// marshalDialReq marshals the dial request to send over the wire.
func marshalDialReq(req *dialReq) ([]byte, error) {
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// connectProxyTransport opens a channel over the remote tunnel and connects
// to the requested host.
func connectProxyTransport(sconn ssh.Conn, req *dialReq) (*utils.ChConn, bool, error) {
	channel, _, err := sconn.OpenChannel(chanTransport, nil)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	payload, err := marshalDialReq(req)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	// Send a special SSH out-of-band request called "teleport-transport"
	// the agent on the other side will create a new TCP/IP connection to
	// 'addr' on its network and will start proxying that connection over
	// this SSH channel.
	ok, err := channel.SendRequest(chanTransportDialReq, true, payload)
	if err != nil {
		return nil, true, trace.Wrap(err)
	}
	if !ok {
		defer channel.Close()

		// Pull the error message from the tunnel client (remote cluster)
		// passed to us via stderr.
		errMessage, _ := ioutil.ReadAll(channel.Stderr())
		if errMessage == nil {
			errMessage = []byte(fmt.Sprintf("failed connecting to %v [%v]", req.Address, req.ServerID))
		}
		return nil, false, trace.Errorf(strings.TrimSpace(string(errMessage)))
	}

	if req.Exclusive {
		return utils.NewExclusiveChConn(sconn, channel), false, nil
	}
	return utils.NewChConn(sconn, channel), false, nil
}

// transport is used to build a connection to the target host.
type transport struct {
	component    string
	log          *logrus.Entry
	closeContext context.Context
	authClient   auth.AccessPoint
	channel      ssh.Channel
	requestCh    <-chan *ssh.Request

	// localClusterName is the name of the cluster that the transport code is
	// running in.
	localClusterName string

	// kubeDialAddr is the address of the Kubernetes proxy.
	kubeDialAddr utils.NetAddr

	// sconn is a SSH connection to the remote host. Used for dial back nodes.
	sconn ssh.Conn
	// server is the underlying SSH server. Used for dial back nodes.
	server ServerHandler

	// reverseTunnelServer holds all reverse tunnel connections.
	reverseTunnelServer Server
}

// start will start the transporting data over the tunnel. This function will
// typically run in the agent or reverse tunnel server. It's used to establish
// connections from remote clusters into the main cluster or for remote nodes
// that have no direct network access to the cluster.
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
	case <-time.After(defaults.DefaultDialTimeout):
		p.log.Warnf("Transport request failed: timed out waiting for request.")
		return
	}

	var servers []string

	dreq := parseDialReq(req.Payload)
	p.log.Debugf("Received out-of-band proxy transport request for %v [%v].", dreq.Address, dreq.ServerID)

	// Handle special non-resolvable addresses first.
	switch dreq.Address {
	// Connect to an Auth Server.
	case RemoteAuthServer:
		authServers, err := p.authClient.GetAuthServers()
		if err != nil {
			p.log.Errorf("Transport request failed: unable to get list of Auth Servers: %v.", err)
			req.Reply(false, []byte("connection rejected: failed to connect to auth server"))
			return
		}
		if len(authServers) == 0 {
			p.log.Errorf("Transport request failed: no auth servers found.")
			req.Reply(false, []byte("connection rejected: failed to connect to auth server"))
			return
		}
		for _, as := range authServers {
			servers = append(servers, as.GetAddr())
		}
	// Connect to the Kubernetes proxy.
	case RemoteKubeProxy:
		if p.component == teleport.ComponentReverseTunnelServer {
			req.Reply(false, []byte("connection rejected: no remote kubernetes proxy"))
			return
		}

		// If Kubernetes is not configured, reject the connection.
		if p.kubeDialAddr.IsEmpty() {
			req.Reply(false, []byte("connection rejected: configure kubernetes proxy for this cluster."))
			return
		}
		servers = append(servers, p.kubeDialAddr.Addr)
	// LocalNode requests are for the single server running in the agent pool.
	case LocalNode:
		if p.component == teleport.ComponentReverseTunnelServer {
			req.Reply(false, []byte("connection rejected: no local node"))
			return
		}
		if p.server == nil {
			req.Reply(false, []byte("connection rejected: server missing"))
			return
		}
		if p.sconn == nil {
			req.Reply(false, []byte("connection rejected: server connection missing"))
			return
		}

		req.Reply(true, []byte("Connected."))

		// Hand connection off to the SSH server.
		p.server.HandleConnection(utils.NewChConn(p.sconn, p.channel))
		return
	default:
		servers = append(servers, dreq.Address)
	}

	// Get a connection to the target address. If a tunnel exists with matching
	// search names, connection over the tunnel is returned. Otherwise a direct
	// net.Dial is performed.
	conn, useTunnel, err := p.getConn(servers, dreq.ServerID)
	if err != nil {
		errorMessage := fmt.Sprintf("connection rejected: %v", err)
		fmt.Fprint(p.channel.Stderr(), errorMessage)
		req.Reply(false, []byte(errorMessage))
		return
	}

	// Dial was successful.
	req.Reply(true, []byte("Connected."))
	p.log.Debugf("Successfully dialed to %v %v, start proxying.", dreq.Address, dreq.ServerID)

	// Start processing channel requests. Pass in a context that wraps the passed
	// in context with a context that closes when this function returns to
	// mitigate a goroutine leak.
	ctx, cancel := context.WithCancel(p.closeContext)
	defer cancel()
	go p.handleChannelRequests(ctx, useTunnel)

	errorCh := make(chan error, 2)

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
			p.log.Warnf("Proxy transport failed: closing context.")
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
			case utils.ConnectionTypeRequest:
				err := req.Reply(useTunnel, nil)
				if err != nil {
					p.log.Debugf("Failed to reply to %v request: %v.", req.Type, err)
					continue
				}
			default:
				err := req.Reply(false, nil)
				if err != nil {
					p.log.Debugf("Failed to reply to %v request: %v.", req.Type, err)
					continue
				}
			}
		case <-closeContext.Done():
			return
		}
	}
}

// getConn checks if the local site holds a connection to the target host,
// and if it does, attempts to dial through the tunnel. Otherwise directly
// dials to host.
func (p *transport) getConn(servers []string, serverID string) (net.Conn, bool, error) {
	// This function doesn't attempt to dial if a host with one of the
	// search names is not registered. It's a fast check.
	p.log.Debugf("Attempting to dial through tunnel with server ID %v.", serverID)
	conn, err := p.tunnelDial(serverID)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, false, trace.Wrap(err)
		}

		p.log.Debugf("Attempting to dial directly %v.", servers)
		conn, err = directDial(servers)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		p.log.Debugf("Returning direct dialed connection to %v.", servers)
		return conn, false, nil
	}

	p.log.Debugf("Returning connection dialed through tunnel with server ID %v.", serverID)
	return conn, true, nil
}

// tunnelDial looks up the search names in the local site for a matching tunnel
// connection. If a connection exists, it's used to dial through the tunnel.
func (p *transport) tunnelDial(serverID string) (net.Conn, error) {
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

	conn, err := localCluster.dialTunnel(DialParams{
		ServerID: serverID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// directDial attempst to directly dial to the target host.
func directDial(servers []string) (net.Conn, error) {
	var errors []error

	for _, addr := range servers {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn, nil
		}

		errors = append(errors, err)
	}

	return nil, trace.NewAggregate(errors...)
}
