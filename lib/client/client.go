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
	"github.com/gravitational/teleport/lib/defaults"
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
	hostKeyCallback utils.HostKeyCallback
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
	siteNodes, err := site.GetNodes(namespace)
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

	tlsConfig := utils.TLSConfig()
	localAgent := proxy.teleportClient.LocalAgent()
	pool, err := localAgent.GetCerts(proxy.teleportClient.ProxyHost())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.RootCAs = pool
	keys, err := localAgent.GetKeys(proxy.teleportClient.Username)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS keys for %v", proxy.teleportClient.Username)
	}
	for _, key := range keys {
		if len(key.TLSCert) != 0 {
			tlsCert, err := tls.X509KeyPair(key.TLSCert, key.Priv)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse TLS cert and key")
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, tlsCert)
		}
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
	log.Infof("[CLIENT] client=%v connecting to auth server on cluster ", proxy.clientAddr, clusterName)

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

	client := ssh.NewClient(conn, chans, reqs)

	return &NodeClient{Client: client, Proxy: proxy, Namespace: defaults.Namespace}, nil
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

// Upload uploads local file(s) or to the remote server's destination path
func (client *NodeClient) Upload(srcPath, rDestPath string, recursive bool, stderr, progressWriter io.Writer) error {
	scpConf := scp.Command{
		Source:    true,
		Recursive: recursive,
		Target:    []string{srcPath},
		Terminal:  progressWriter,
	}

	// "impersonate" scp to a server
	shellCmd := "/usr/bin/scp -t"
	if recursive {
		shellCmd += " -r"
	}
	shellCmd += (" " + rDestPath)
	return client.scp(scpConf, shellCmd, stderr)
}

// Download downloads file or dir from the remote server
func (client *NodeClient) Download(remoteSourcePath, localDestinationPath string, recursive bool, stderr, progressWriter io.Writer) error {
	scpConf := scp.Command{
		Sink:      true,
		Recursive: recursive,
		Target:    []string{localDestinationPath},
		Terminal:  progressWriter,
	}

	// "impersonate" scp to a server
	shellCmd := "/usr/bin/scp -f"
	if recursive {
		shellCmd += " -r"
	}
	shellCmd += (" " + remoteSourcePath)
	return client.scp(scpConf, shellCmd, stderr)
}

// scp runs remote scp command(shellCmd) on the remote server and
// runs local scp handler using scpConf
func (client *NodeClient) scp(scpCommand scp.Command, shellCmd string, errWriter io.Writer) error {
	session, err := client.Client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	stdout, err := session.StdoutPipe()
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
		if err = scpCommand.Execute(ch); err != nil {
			log.Error(err)
		}
		stdin.Close()
		close(closeC)
	}()

	runErr := session.Run(shellCmd)
	if runErr != nil && err == nil {
		err = runErr
	}
	<-closeC
	if trace.IsEOF(err) {
		err = nil
	}
	return trace.Wrap(err)
}

// listenAndForward listens on a given socket and forwards all incoming connections
// to the given remote address via
func (client *NodeClient) listenAndForward(socket net.Listener, remoteAddr string) {
	defer socket.Close()
	defer client.Close()
	proxyConnection := func(incoming net.Conn) {
		defer incoming.Close()
		var (
			conn net.Conn
			err  error
		)
		log.Debugf("nodeClient.listenAndForward(%v -> %v) started", incoming.RemoteAddr(), remoteAddr)
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
		log.Debugf("nodeClient.listenAndForward(%v -> %v) exited", incoming.RemoteAddr(), remoteAddr)
	}
	// request processing loop: accept incoming requests to be connected to nodes
	// and proxy them to 'remoteAddr'
	for {
		incoming, err := socket.Accept()
		if err != nil {
			log.Error(err)
			break
		}
		go proxyConnection(incoming)
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
