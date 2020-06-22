/*
Copyright 2015 Gravitational, Inc.

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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/socks"

	"github.com/gravitational/trace"
)

// ProxyClient implements ssh client to a teleport proxy
// It can provide list of nodes or connect to nodes
type ProxyClient struct {
	teleportClient  *TeleportClient
	Client          *ssh.Client
	hostLogin       string
	proxyAddress    string
	proxyPrincipal  string
	hostKeyCallback ssh.HostKeyCallback
	authMethod      ssh.AuthMethod
	siteName        string
	clientAddr      string
}

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Namespace string
	Client    *ssh.Client
	Proxy     *ProxyClient
	TC        *TeleportClient
}

// GetSites returns list of the "sites" (AKA teleport clusters) connected to the proxy
// Each site is returned as an instance of its auth server
//
func (proxy *ProxyClient) GetSites() ([]services.Site, error) {
	proxySession, err := proxy.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxySession.Close()
	stdout := &bytes.Buffer{}
	reader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := io.Copy(stdout, reader); err != nil {
			log.Warningf("Error reading STDOUT from proxy: %v", err)
		}
		close(done)
	}()
	// this function is async because,
	// the function call StdoutPipe() could fail if proxy rejected
	// the session request, and then RequestSubsystem call could hang
	// forever
	go func() {
		if err := proxySession.RequestSubsystem("proxysites"); err != nil {
			log.Warningf("Failed to request subsystem: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(defaults.DefaultDialTimeout):
		return nil, trace.ConnectionProblem(nil, "timeout")
	}
	log.Debugf("Found clusters: %v", stdout.String())
	var sites []services.Site
	if err := json.Unmarshal(stdout.Bytes(), &sites); err != nil {
		return nil, trace.Wrap(err)
	}
	return sites, nil
}

// GenerateCertsForCluster generates certificates for the user
// that have a metadata instructing server to route the requests to the cluster
func (proxy *ProxyClient) GenerateCertsForCluster(ctx context.Context, routeToCluster string) error {
	localAgent := proxy.teleportClient.LocalAgent()
	key, err := localAgent.GetKey()
	if err != nil {
		return trace.Wrap(err)
	}
	cert, err := key.SSHCert()
	if err != nil {
		return trace.Wrap(err)
	}
	tlsCert, err := key.TLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName, err := tlsca.ClusterName(tlsCert.Issuer)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := proxy.ConnectToCluster(ctx, clusterName, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// Before requesting a certificate, check if the requested cluster is valid.
	_, err = clt.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: routeToCluster,
	}, false)
	if err != nil {
		return trace.NotFound("cluster %v not found", routeToCluster)
	}

	req := proto.UserCertsRequest{
		Username:       cert.KeyId,
		PublicKey:      key.Pub,
		Expires:        time.Unix(int64(cert.ValidBefore), 0),
		RouteToCluster: routeToCluster,
	}
	if _, ok := cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles]; !ok {
		req.Format = teleport.CertificateFormatOldSSH
	}

	certs, err := clt.GenerateUserCerts(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	// save the cert to the local storage (~/.tsh usually):
	_, err = localAgent.AddKey(key)
	return trace.Wrap(err)
}

// ReissueParams encodes optional parameters for
// user certificate reissue.
type ReissueParams struct {
	RouteToCluster string
	AccessRequests []string
}

// ReissueUserCerts generates certificates for the user
// that have a metadata instructing server to route the requests to the cluster
func (proxy *ProxyClient) ReissueUserCerts(ctx context.Context, params ReissueParams) error {
	localAgent := proxy.teleportClient.LocalAgent()
	key, err := localAgent.GetKey()
	if err != nil {
		return trace.Wrap(err)
	}
	cert, err := key.SSHCert()
	if err != nil {
		return trace.Wrap(err)
	}
	tlsCert, err := key.TLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName, err := tlsca.ClusterName(tlsCert.Issuer)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := proxy.ConnectToCluster(ctx, clusterName, true)
	if err != nil {
		return trace.Wrap(err)
	}

	if params.RouteToCluster != "" {
		// Before requesting a certificate, check if the requested cluster is valid.
		_, err = clt.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: params.RouteToCluster,
		}, false)
		if err != nil {
			return trace.NotFound("cluster %v not found", params.RouteToCluster)
		}
	}
	req := proto.UserCertsRequest{
		Username:       cert.KeyId,
		PublicKey:      key.Pub,
		Expires:        time.Unix(int64(cert.ValidBefore), 0),
		RouteToCluster: params.RouteToCluster,
		AccessRequests: params.AccessRequests,
	}
	if _, ok := cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles]; !ok {
		req.Format = teleport.CertificateFormatOldSSH
	}

	certs, err := clt.GenerateUserCerts(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	// save the cert to the local storage (~/.tsh usually):
	_, err = localAgent.AddKey(key)
	return trace.Wrap(err)
}

// CreateAccessRequest registers a new access request with the auth server.
func (proxy *ProxyClient) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	site, err := proxy.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	return site.CreateAccessRequest(ctx, req)
}

// GetAccessRequests loads all access requests matching the spupplied filter.
func (proxy *ProxyClient) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
	site, err := proxy.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqs, err := site.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reqs, nil
}

// NewWatcher sets up a new event watcher.
func (proxy *ProxyClient) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	site, err := proxy.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	watcher, err := site.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return watcher, nil
}

// FindServersByLabels returns list of the nodes which have labels exactly matching
// the given label set.
//
// A server is matched when ALL labels match.
// If no labels are passed, ALL nodes are returned.
func (proxy *ProxyClient) FindServersByLabels(ctx context.Context, namespace string, labels map[string]string) ([]services.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter(auth.MissingNamespaceError)
	}
	nodes := make([]services.Server, 0)
	site, err := proxy.CurrentClusterAccessPoint(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	siteNodes, err := site.GetNodes(namespace, services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// look at every node on this site and see which ones match:
	for _, node := range siteNodes {
		if node.MatchAgainst(labels) {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// CurrentClusterAccessPoint returns cluster access point to the currently
// selected cluster and is used for discovery
// and could be cached based on the access policy
func (proxy *ProxyClient) CurrentClusterAccessPoint(ctx context.Context, quiet bool) (auth.AccessPoint, error) {
	// get the current cluster:
	cluster, err := proxy.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxy.ClusterAccessPoint(ctx, cluster.Name, quiet)
}

// ClusterAccessPoint returns cluster access point used for discovery
// and could be cached based on the access policy
func (proxy *ProxyClient) ClusterAccessPoint(ctx context.Context, clusterName string, quiet bool) (auth.AccessPoint, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("parameter clusterName is missing")
	}
	clt, err := proxy.ConnectToCluster(ctx, clusterName, quiet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxy.teleportClient.accessPoint(clt, proxy.proxyAddress, clusterName)
}

// ConnectToCurrentCluster connects to the auth server of the currently selected
// cluster via proxy. It returns connected and authenticated auth server client
//
// if 'quiet' is set to true, no errors will be printed to stdout, otherwise
// any connection errors are visible to a user.
func (proxy *ProxyClient) ConnectToCurrentCluster(ctx context.Context, quiet bool) (auth.ClientI, error) {
	cluster, err := proxy.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxy.ConnectToCluster(ctx, cluster.Name, quiet)
}

// ConnectToCluster connects to the auth server of the given cluster via proxy.
// It returns connected and authenticated auth server client
//
// if 'quiet' is set to true, no errors will be printed to stdout, otherwise
// any connection errors are visible to a user.
func (proxy *ProxyClient) ConnectToCluster(ctx context.Context, clusterName string, quiet bool) (auth.ClientI, error) {
	dialer := auth.ContextDialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
		return proxy.dialAuthServer(ctx, clusterName)
	})

	if proxy.teleportClient.SkipLocalAuth {
		return auth.NewTLSClient(auth.ClientConfig{
			Dialer: dialer,
			TLS:    proxy.teleportClient.TLS,
		})
	}

	localAgent := proxy.teleportClient.LocalAgent()
	key, err := localAgent.GetKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS key for %v", proxy.teleportClient.Username)
	}
	tlsConfig, err := key.ClientTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate client TLS config")
	}
	clt, err := auth.NewTLSClient(auth.ClientConfig{
		Dialer: dialer,
		TLS:    tlsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// nodeName removes the port number from the hostname, if present
func nodeName(node string) string {
	n, _, err := net.SplitHostPort(node)
	if err != nil {
		return node
	}
	return n
}

type proxyResponse struct {
	isRecord bool
	err      error
}

// isRecordingProxy returns true if the proxy is in recording mode. Note, this
// function can only be called after authentication has occurred and should be
// called before the first session is created.
func (proxy *ProxyClient) isRecordingProxy() (bool, error) {
	responseCh := make(chan proxyResponse)

	// we have to run this in a goroutine because older version of Teleport handled
	// global out-of-band requests incorrectly: Teleport would ignore requests it
	// does not know about and never reply to them. So if we wait a second and
	// don't hear anything back, most likley we are trying to connect to an older
	// version of Teleport and we should not try and forward our agent.
	go func() {
		ok, responseBytes, err := proxy.Client.SendRequest(teleport.RecordingProxyReqType, true, nil)
		if err != nil {
			responseCh <- proxyResponse{isRecord: false, err: trace.Wrap(err)}
			return
		}
		if !ok {
			responseCh <- proxyResponse{isRecord: false, err: trace.AccessDenied("unable to determine proxy type")}
			return
		}

		recordingProxy, err := strconv.ParseBool(string(responseBytes))
		if err != nil {
			responseCh <- proxyResponse{isRecord: false, err: trace.Wrap(err)}
			return
		}

		responseCh <- proxyResponse{isRecord: recordingProxy, err: nil}
	}()

	select {
	case resp := <-responseCh:
		if resp.err != nil {
			return false, trace.Wrap(resp.err)
		}
		return resp.isRecord, nil
	case <-time.After(1 * time.Second):
		// probably the older version of the proxy or at least someone that is
		// responding incorrectly, don't forward agent to it
		return false, nil
	}
}

// dialAuthServer returns auth server connection forwarded via proxy
func (proxy *ProxyClient) dialAuthServer(ctx context.Context, clusterName string) (net.Conn, error) {
	log.Debugf("Client %v is connecting to auth server on cluster %q.", proxy.clientAddr, clusterName)

	address := "@" + clusterName

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxySession, err := proxy.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyWriter, err := proxySession.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyReader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyErr, err := proxySession.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = proxySession.RequestSubsystem("proxy:" + address)
	if err != nil {
		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := ioutil.ReadAll(proxyErr)
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s",
			nodeName(strings.Split(address, "@")[0]), serverErrorMsg)
	}
	return utils.NewPipeNetConn(
		proxyReader,
		proxyWriter,
		proxySession,
		localAddr,
		fakeAddr,
	), nil
}

// NodeAddr is a full node address
type NodeAddr struct {
	// Addr is an address to dial
	Addr string
	// Namespace is the node namespace
	Namespace string
	// Cluster is the name of the target cluster
	Cluster string
}

// String returns a user-friendly name
func (n NodeAddr) String() string {
	parts := []string{nodeName(n.Addr)}
	if n.Cluster != "" {
		parts = append(parts, "on cluster", n.Cluster)
	}
	return strings.Join(parts, " ")
}

// ProxyFormat returns the address in the format
// used by the proxy subsystem
func (n *NodeAddr) ProxyFormat() string {
	parts := []string{n.Addr}
	if n.Namespace != "" {
		parts = append(parts, n.Namespace)
	}
	if n.Cluster != "" {
		parts = append(parts, n.Cluster)
	}
	return strings.Join(parts, "@")
}

// requestSubsystem sends a subsystem request on the session. If the passed
// in context is canceled first, unblocks.
func requestSubsystem(ctx context.Context, session *ssh.Session, name string) error {
	errCh := make(chan error)

	go func() {
		er := session.RequestSubsystem(name)
		errCh <- er
	}()

	select {
	case err := <-errCh:
		return trace.Wrap(err)
	case <-ctx.Done():
		err := session.Close()
		if err != nil {
			log.Debugf("Failed to close session: %v.", err)
		}
		return trace.Wrap(ctx.Err())
	}
}

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(ctx context.Context, nodeAddress NodeAddr, user string, quiet bool) (*NodeClient, error) {
	log.Infof("Client=%v connecting to node=%v", proxy.clientAddr, nodeAddress)
	if len(proxy.teleportClient.JumpHosts) > 0 {
		return proxy.PortForwardToNode(ctx, nodeAddress, user, quiet)
	}

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + nodeAddress.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// after auth but before we create the first session, find out if the proxy
	// is in recording mode or not
	recordingProxy, err := proxy.isRecordingProxy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxySession, err := proxy.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyWriter, err := proxySession.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyReader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyErr, err := proxySession.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// pass the true client IP (if specified) to the proxy so it could pass it into the
	// SSH session for proper audit
	if len(proxy.clientAddr) > 0 {
		if err = proxySession.Setenv(sshutils.TrueClientAddrVar, proxy.clientAddr); err != nil {
			log.Error(err)
		}
	}

	// the client only tries to forward an agent when the proxy is in recording
	// mode. we always try and forward an agent here because each new session
	// creates a new context which holds the agent. if ForwardToAgent returns an error
	// "already have handler for" we ignore it.
	if recordingProxy {
		err = agent.ForwardToAgent(proxy.Client, proxy.teleportClient.localAgent.Agent)
		if err != nil && !strings.Contains(err.Error(), "agent: already have handler for") {
			return nil, trace.Wrap(err)
		}

		err = agent.RequestAgentForwarding(proxySession)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = requestSubsystem(ctx, proxySession, "proxy:"+nodeAddress.ProxyFormat())
	if err != nil {
		// If the user pressed Ctrl-C, no need to try and read the error from
		// the proxy, return an error right away.
		if trace.Unwrap(err) == context.Canceled {
			return nil, trace.Wrap(err)
		}

		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := ioutil.ReadAll(proxyErr)
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s",
			nodeName(nodeAddress.Addr), serverErrorMsg)
	}
	pipeNetConn := utils.NewPipeNetConn(
		proxyReader,
		proxyWriter,
		proxySession,
		localAddr,
		fakeAddr,
	)
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{proxy.authMethod},
		HostKeyCallback: proxy.hostKeyCallback,
	}
	conn, chans, reqs, err := newClientConn(ctx, pipeNetConn, nodeAddress.ProxyFormat(), sshConfig)
	if err != nil {
		if utils.IsHandshakeFailedError(err) {
			proxySession.Close()
			return nil, trace.AccessDenied(`access denied to %v connecting to %v`, user, nodeAddress)
		}
		return nil, trace.Wrap(err)
	}

	// We pass an empty channel which we close right away to ssh.NewClient
	// because the client need to handle requests itself.
	emptyCh := make(chan *ssh.Request)
	close(emptyCh)

	client := ssh.NewClient(conn, chans, emptyCh)

	nc := &NodeClient{
		Client:    client,
		Proxy:     proxy,
		Namespace: defaults.Namespace,
		TC:        proxy.teleportClient,
	}

	// Start a goroutine that will run for the duration of the client to process
	// global requests from the client. Teleport clients will use this to update
	// terminal sizes when the remote PTY size has changed.
	go nc.handleGlobalRequests(ctx, reqs)

	return nc, nil
}

// PortForwardToNode connects to the ssh server via Proxy
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) PortForwardToNode(ctx context.Context, nodeAddress NodeAddr, user string, quiet bool) (*NodeClient, error) {
	log.Infof("Client=%v jumping to node=%s", proxy.clientAddr, nodeAddress)

	// after auth but before we create the first session, find out if the proxy
	// is in recording mode or not
	recordingProxy, err := proxy.isRecordingProxy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// the client only tries to forward an agent when the proxy is in recording
	// mode. we always try and forward an agent here because each new session
	// creates a new context which holds the agent. if ForwardToAgent returns an error
	// "already have handler for" we ignore it.
	if recordingProxy {
		err = agent.ForwardToAgent(proxy.Client, proxy.teleportClient.localAgent.Agent)
		if err != nil && !strings.Contains(err.Error(), "agent: already have handler for") {
			return nil, trace.Wrap(err)
		}
	}

	proxyConn, err := proxy.Client.Dial("tcp", nodeAddress.Addr)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s", nodeAddress, err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{proxy.authMethod},
		HostKeyCallback: proxy.hostKeyCallback,
	}
	conn, chans, reqs, err := newClientConn(ctx, proxyConn, nodeAddress.Addr, sshConfig)
	if err != nil {
		if utils.IsHandshakeFailedError(err) {
			proxyConn.Close()
			return nil, trace.AccessDenied(`access denied to %v connecting to %v`, user, nodeAddress)
		}
		return nil, trace.Wrap(err)
	}

	// We pass an empty channel which we close right away to ssh.NewClient
	// because the client need to handle requests itself.
	emptyCh := make(chan *ssh.Request)
	close(emptyCh)

	client := ssh.NewClient(conn, chans, emptyCh)

	nc := &NodeClient{
		Client:    client,
		Proxy:     proxy,
		Namespace: defaults.Namespace,
		TC:        proxy.teleportClient,
	}

	// Start a goroutine that will run for the duration of the client to process
	// global requests from the client. Teleport clients will use this to update
	// terminal sizes when the remote PTY size has changed.
	go nc.handleGlobalRequests(ctx, reqs)

	return nc, nil
}

func (c *NodeClient) handleGlobalRequests(ctx context.Context, requestCh <-chan *ssh.Request) {
	for {
		select {
		case r := <-requestCh:
			// When the channel is closing, nil is returned.
			if r == nil {
				return
			}

			switch r.Type {
			case teleport.SessionEvent:
				// Parse event and create events.EventFields that can be consumed directly
				// by caller.
				var e events.EventFields
				err := json.Unmarshal(r.Payload, &e)
				if err != nil {
					log.Warnf("Unable to parse event: %v: %v.", string(r.Payload), err)
					continue
				}

				// Send event to event channel.
				err = c.TC.SendEvent(ctx, e)
				if err != nil {
					log.Warnf("Unable to send event %v: %v.", string(r.Payload), err)
					continue
				}
			default:
				// This handles keep-alive messages and matches the behaviour of OpenSSH.
				err := r.Reply(false, nil)
				if err != nil {
					log.Warnf("Unable to reply to %v request.", r.Type)
					continue
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// newClientConn is a wrapper around ssh.NewClientConn
func newClientConn(ctx context.Context,
	conn net.Conn,
	nodeAddress string,
	config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {

	type response struct {
		conn   ssh.Conn
		chanCh <-chan ssh.NewChannel
		reqCh  <-chan *ssh.Request
		err    error
	}

	respCh := make(chan response, 1)
	go func() {
		conn, chans, reqs, err := ssh.NewClientConn(conn, nodeAddress, config)
		respCh <- response{conn, chans, reqs, err}
	}()

	select {
	case resp := <-respCh:
		if resp.err != nil {
			return nil, nil, nil, trace.Wrap(resp.err, "failed to connect to %q", nodeAddress)
		}
		return resp.conn, resp.chanCh, resp.reqCh, nil
	case <-ctx.Done():
		errClose := conn.Close()
		if errClose != nil {
			log.Error(errClose)
		}
		// drain the channel
		resp := <-respCh
		return nil, nil, nil, trace.ConnectionProblem(resp.err, "failed to connect to %q", nodeAddress)
	}
}

func (proxy *ProxyClient) Close() error {
	return proxy.Client.Close()
}

// ExecuteSCP runs remote scp command(shellCmd) on the remote server and
// runs local scp handler using SCP Command
func (client *NodeClient) ExecuteSCP(cmd scp.Command) error {
	shellCmd, err := cmd.GetRemoteShellCmd()
	if err != nil {
		return trace.Wrap(err)
	}

	s, err := client.Client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	defer s.Close()

	stdin, err := s.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	stdout, err := s.StdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	ch := utils.NewPipeNetConn(
		stdout,
		stdin,
		utils.MultiCloser(),
		&net.IPAddr{},
		&net.IPAddr{},
	)

	closeC := make(chan interface{}, 1)
	go func() {
		if err = cmd.Execute(ch); err != nil {
			log.Error(err)
		}
		stdin.Close()
		close(closeC)
	}()

	runErr := s.Run(shellCmd)
	<-closeC

	if runErr != nil && (err == nil || trace.IsEOF(err)) {
		err = runErr
	}
	if trace.IsEOF(err) {
		err = nil
	}
	return trace.Wrap(err)
}

type netDialer interface {
	Dial(string, string) (net.Conn, error)
}

func proxyConnection(ctx context.Context, conn net.Conn, remoteAddr string, dialer netDialer) error {
	defer conn.Close()
	defer log.Debugf("Finished proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)

	var (
		remoteConn net.Conn
		err        error
	)

	log.Debugf("Attempting to connect proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)
	for attempt := 1; attempt <= 5; attempt++ {
		remoteConn, err = dialer.Dial("tcp", remoteAddr)
		if err != nil {
			log.Debugf("Proxy connection attempt %v: %v.", attempt, err)

			timer := time.NewTimer(time.Duration(100*attempt) * time.Millisecond)
			defer timer.Stop()

			// Wait and attempt to connect again, if the context has closed, exit
			// right away.
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-timer.C:
				continue
			}
		}
		// Connection established, break out of the loop.
		break
	}
	if err != nil {
		return trace.BadParameter("failed to connect to node: %v", remoteAddr)
	}
	defer remoteConn.Close()

	// Start proxying, close the connection if a problem occurs on either leg.
	errCh := make(chan error, 2)
	go func() {
		defer conn.Close()
		defer remoteConn.Close()

		_, err := io.Copy(conn, remoteConn)
		errCh <- err
	}()
	go func() {
		defer conn.Close()
		defer remoteConn.Close()

		_, err := io.Copy(remoteConn, conn)
		errCh <- err
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Warnf("Failed to proxy connection: %v.", err)
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}

	return trace.NewAggregate(errs...)
}

// listenAndForward listens on a given socket and forwards all incoming
// commands to the remote address through the SSH tunnel.
func (c *NodeClient) listenAndForward(ctx context.Context, ln net.Listener, remoteAddr string) {
	defer ln.Close()
	defer c.Close()

	for {
		// Accept connections from the client.
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("Port forwarding failed: %v.", err)
			break
		}

		// Proxy the connection to the remote address.
		go func() {
			err := proxyConnection(ctx, conn, remoteAddr, c.Client)
			if err != nil {
				log.Warnf("Failed to proxy connection: %v.", err)
			}
		}()
	}
}

// dynamicListenAndForward listens for connections, performs a SOCKS5
// handshake, and then proxies the connection to the requested address.
func (c *NodeClient) dynamicListenAndForward(ctx context.Context, ln net.Listener) {
	defer ln.Close()
	defer c.Close()

	for {
		// Accept connection from the client. Here the client is typically
		// something like a web browser or other SOCKS5 aware application.
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("Dynamic port forwarding (SOCKS5) failed: %v.", err)
			break
		}

		// Perform the SOCKS5 handshake with the client to find out the remote
		// address to proxy.
		remoteAddr, err := socks.Handshake(conn)
		if err != nil {
			log.Errorf("SOCKS5 handshake failed: %v.", err)
			break
		}
		log.Debugf("SOCKS5 proxy forwarding requests to %v.", remoteAddr)

		// Proxy the connection to the remote address.
		go func() {
			err := proxyConnection(ctx, conn, remoteAddr, c.Client)
			if err != nil {
				log.Warnf("Failed to proxy connection: %v.", err)
			}
		}()
	}
}

// Close closes client and it's operations
func (client *NodeClient) Close() error {
	return client.Client.Close()
}

// currentCluster returns the connection to the API of the current cluster
func (proxy *ProxyClient) currentCluster() (*services.Site, error) {
	sites, err := proxy.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sites) == 0 {
		return nil, trace.NotFound("no clusters registered")
	}
	if proxy.siteName == "" {
		return &sites[0], nil
	}
	for _, site := range sites {
		if site.Name == proxy.siteName {
			return &site, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", proxy.siteName)
}
