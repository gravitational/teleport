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
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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

	stdout := &bytes.Buffer{}
	reader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	done := make(chan struct{})
	go func() {
		io.Copy(stdout, reader)
		close(done)
	}()

	if err := proxySession.RequestSubsystem("proxysites"); err != nil {
		return nil, trace.Wrap(err)
	}
	select {
	case <-done:
	case <-time.After(defaults.DefaultDialTimeout):
		return nil, trace.ConnectionProblem(nil, "timeout")
	}

	log.Debugf("[CLIENT] found clusters: %v", stdout.String())

	var sites []services.Site
	if err := json.Unmarshal(stdout.Bytes(), &sites); err != nil {
		return nil, trace.Wrap(err)
	}
	return sites, nil
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
	site, err := proxy.ClusterAccessPoint(ctx, false)
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

// ClusterAccessPoint returns cluster access point used for discovery
// and could be cached based on the access policy
func (proxy *ProxyClient) ClusterAccessPoint(ctx context.Context, quiet bool) (auth.AccessPoint, error) {
	// get the current cluster:
	cluster, err := proxy.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := proxy.ConnectToSite(ctx, quiet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxy.teleportClient.accessPoint(clt, proxy.proxyAddress, cluster.Name)
}

// ConnectToSite connects to the auth server of the given site via proxy.
// It returns connected and authenticated auth server client
//
// if 'quiet' is set to true, no errors will be printed to stdout, otherwise
// any connection errors are visible to a user.
func (proxy *ProxyClient) ConnectToSite(ctx context.Context, quiet bool) (auth.ClientI, error) {
	// get the current cluster:
	site, err := proxy.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer := func(ctx context.Context, network, _ string) (net.Conn, error) {
		return proxy.dialAuthServer(ctx, site.Name)
	}

	if proxy.teleportClient.SkipLocalAuth {
		return auth.NewTLSClientWithDialer(dialer, proxy.teleportClient.TLS)
	}

	// Because Teleport clients can't be configured (yet), they take the default
	// list of cipher suites from Go.
	tlsConfig := utils.TLSConfig(nil)
	localAgent := proxy.teleportClient.LocalAgent()
	pool, err := localAgent.GetCerts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.RootCAs = pool
	key, err := localAgent.GetKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS key for %v", proxy.teleportClient.Username)
	}
	if len(key.TLSCert) != 0 {
		tlsCert, err := tls.X509KeyPair(key.TLSCert, key.Priv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse TLS cert and key")
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, tlsCert)
	}
	if len(tlsConfig.Certificates) == 0 {
		return nil, trace.BadParameter("no TLS keys found for user %v, please relogin to get new credentials", proxy.teleportClient.Username)
	}
	clt, err := auth.NewTLSClientWithDialer(dialer, tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// closerConn wraps connection and attaches additional closers to it
type closerConn struct {
	net.Conn
	closers []io.Closer
}

// addCloser adds any closer in ctx that will be called
// whenever server closes session channel
func (c *closerConn) addCloser(closer io.Closer) {
	c.closers = append(c.closers, closer)
}

func (c *closerConn) Close() error {
	var errors []error
	for _, closer := range c.closers {
		errors = append(errors, closer.Close())
	}
	errors = append(errors, c.Conn.Close())
	return trace.NewAggregate(errors...)
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
// function can only be called after authentication has occured and should be
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

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(ctx context.Context, nodeAddress string, user string, quiet bool) (*NodeClient, error) {
	log.Infof("[CLIENT] client=%v connecting to node=%s", proxy.clientAddr, nodeAddress)

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + nodeAddress)
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

	err = proxySession.RequestSubsystem("proxy:" + nodeAddress)
	if err != nil {
		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := ioutil.ReadAll(proxyErr)
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s",
			nodeName(strings.Split(nodeAddress, "@")[0]), serverErrorMsg)
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
	conn, chans, reqs, err := newClientConn(ctx, pipeNetConn, nodeAddress, sshConfig)
	if err != nil {
		if utils.IsHandshakeFailedError(err) {
			proxySession.Close()
			parts := strings.Split(nodeAddress, "@")
			hostname := parts[0]
			if len(hostname) == 0 && len(parts) > 1 {
				hostname = "cluster " + parts[1]
			}
			return nil, trace.Errorf(`access denied to %v connecting to %v`, user, nodeName(hostname))
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
				// This handles keepalive messages and matches the behaviour of OpenSSH.
				r.Reply(false, nil)
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

func proxyConnection(client *NodeClient, incoming net.Conn, remoteAddr string) {
	defer incoming.Close()
	var (
		conn net.Conn
		err  error
	)
	log.Debugf("nodeClient.proxyConnection(%v -> %v) started", incoming.RemoteAddr(), remoteAddr)
	for attempt := 1; attempt <= 5; attempt++ {
		conn, err = client.Client.Dial("tcp", remoteAddr)
		if err != nil {
			log.Errorf("Connection attempt %v: %v", attempt, err)
			// failed to establish an outbound connection? try again:
			time.Sleep(time.Millisecond * time.Duration(100*attempt))
			continue
		}
		// connection established: continue:
		break
	}
	// permanent failure establishing connection
	if err != nil {
		log.Errorf("Failed to connect to node %v", remoteAddr)
		return
	}
	defer conn.Close()
	// start proxying:
	doneC := make(chan interface{}, 2)
	go func() {
		io.Copy(incoming, conn)
		doneC <- true
	}()
	go func() {
		io.Copy(conn, incoming)
		doneC <- true
	}()
	<-doneC
	<-doneC
	log.Debugf("nodeClient.proxyConnection(%v -> %v) exited", incoming.RemoteAddr(), remoteAddr)
}

// listenAndForward listens on a given socket and forwards all incoming connections
// to the given remote address via
func (client *NodeClient) listenAndForward(socket net.Listener, remoteAddr string) {
	defer socket.Close()
	defer client.Close()
	// request processing loop: accept incoming requests to be connected to nodes
	// and proxy them to 'remoteAddr'
	for {
		incoming, err := socket.Accept()
		if err != nil {
			log.Error(err)
			break
		}
		go proxyConnection(client, incoming, remoteAddr)
	}
}

func readByte(reader io.Reader) (byte, error) {
	buf := []byte{0}
	_, err := io.ReadFull(reader, buf)
	return buf[0], err
}

const (
	socks4Version                 byte = 0x04
	socks5Version                 byte = 0x05
	socks5Reserved                byte = 0x00
	socks5AuthNotRequired         byte = 0x00
	socks5AuthNoAcceptableMethods byte = 0xFF
	socks5CommandConnect          byte = 0x01
	socks5AddressTypeIPv4         byte = 0x01
	socks5AddressTypeDomainName   byte = 0x03
	socks5AddressTypeIPv6         byte = 0x04
	socks5Succeeded               byte = 0x00
)

func socks5ProxyAuthenticate(incoming net.Conn) error {
	nmethods, err := readByte(incoming)
	if err != nil {
		return err
	}

	chosenMethod := socks5AuthNoAcceptableMethods
	for i := byte(0); i < nmethods; i++ {
		method, err := readByte(incoming)
		if err != nil {
			return err
		}
		if method == socks5AuthNotRequired {
			chosenMethod = socks5AuthNotRequired
		}
	}

	_, err = incoming.Write([]byte{socks5Version, chosenMethod})
	if err != nil {
		return err
	}

	if chosenMethod == socks5AuthNoAcceptableMethods {
		return errors.New("Unable to find suitable authentication method")
	}

	return nil
}

func socks5ProxyConnectRequest(incoming net.Conn) (remoteAddr string, err error) {
	header := make([]byte, 4)
	_, err = io.ReadFull(incoming, header)
	if err != nil {
		return
	}
	if !bytes.Equal(header[0:3], []byte{socks5Version, socks5CommandConnect, socks5Reserved}) {
		err = errors.New("only connect command is supported for SOCKS5")
		return
	}

	var ip net.IP
	var remoteHost string
	switch header[3] {
	case socks5AddressTypeIPv4:
		ip = make([]byte, net.IPv4len)
	case socks5AddressTypeIPv6:
		ip = make([]byte, net.IPv6len)
	case socks5AddressTypeDomainName:
		var domainNameLen byte
		domainNameLen, err = readByte(incoming)
		if err != nil {
			return
		}
		remoteAddrBuf := make([]byte, domainNameLen)
		_, err = io.ReadFull(incoming, remoteAddrBuf)
		if err != nil {
			return
		}
		remoteHost = string(remoteAddrBuf)
	default:
		err = errors.New("Unsupported address type for SOCKS5 connect request")
		return
	}

	if ip != nil {
		// Still need to read the ip address
		_, err = io.ReadFull(incoming, ip)
		if err != nil {
			return
		}
		remoteHost = ip.String()
	}

	var remotePort uint16
	err = binary.Read(incoming, binary.BigEndian, &remotePort)
	if err != nil {
		return
	}

	// Send the same minimal response as openSSH does
	response := make([]byte, 4+net.IPv4len+2)
	copy(response, []byte{socks5Version, socks5Succeeded, socks5Reserved, socks5AddressTypeIPv4})
	_, err = incoming.Write(response)
	if err != nil {
		return
	}

	return net.JoinHostPort(remoteHost, strconv.Itoa(int(remotePort))), nil
}

func socks5ProxyConnection(client *NodeClient, incoming net.Conn) {
	err := socks5ProxyAuthenticate(incoming)
	if nil != err {
		log.Errorf("socks5ProxyConnection unable to authenticate (%v) [%v]", incoming, err)
		return
	}

	remoteAddr, err := socks5ProxyConnectRequest(incoming)
	if nil != err {
		log.Errorf("socks5ProxyConnection did not receive connect (%v) [%v]", incoming, err)
		return
	}

	proxyConnection(client, incoming, remoteAddr)
}

func dynamicProxyConnection(client *NodeClient, incoming net.Conn) {
	defer incoming.Close()
	log.Debugf("nodeClient.dynamicProxyConnection(%v) started", incoming.RemoteAddr())

	version := []byte{0}
	_, err := incoming.Read(version)
	if err != nil {
		log.Errorf("Failed to read first byte of %v", incoming)
		return
	}
	switch version[0] {
	case socks5Version:
		socks5ProxyConnection(client, incoming)
	case socks4Version:
		log.Errorf("SOCKS4 dynamic port forwarding is no yet supported (%v)", incoming)
	default:
		log.Errorf("Unknown dynamic port forwarding protocol requested by (%v)", incoming)
	}
}

// listenForDynamicForward listens on a given socket and forwards all incoming connections
// to the remote address they specify
func (client *NodeClient) listenForDynamicForward(socket net.Listener) {
	defer socket.Close()
	defer client.Close()
	// request processing loop: accept incoming requests to be connected to nodes
	// and proxy them
	for {
		incoming, err := socket.Accept()
		if err != nil {
			log.Error(err)
			break
		}
		go dynamicProxyConnection(client, incoming)
	}
}

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
