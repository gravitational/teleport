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
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ProxyClient implements ssh client to a teleport proxy
// It can provide list of nodes or connect to nodes
type ProxyClient struct {
	Client          *ssh.Client
	hostLogin       string
	proxyAddress    string
	hostKeyCallback utils.HostKeyCallback
	authMethods     []ssh.AuthMethod
	siteName        string
}

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Client *ssh.Client
	Proxy  *ProxyClient
}

func (proxy *ProxyClient) getSite() (*services.Site, error) {
	sites, err := proxy.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sites) == 0 {
		return nil, trace.NotFound("no sites registered")
	}
	if proxy.siteName == "" {
		return &sites[0], nil
	}
	for _, site := range sites {
		if site.Name == proxy.siteName {
			return &site, nil
		}
	}
	return nil, trace.NotFound("site %v not found", proxy.siteName)
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
	case <-time.After(teleport.DefaultTimeout):
		return nil, trace.Errorf("timeout")
	}

	log.Infof("[CLIENT] found sites: %v", stdout.String())

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
func (proxy *ProxyClient) FindServersByLabels(labels map[string]string) ([]services.Server, error) {
	nodes := make([]services.Server, 0)
	site, err := proxy.ConnectToSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	siteNodes, err := site.GetNodes()
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

// ConnectToSite connects to the auth server of the given site via proxy.
// It returns connected and authenticated auth server client
func (proxy *ProxyClient) ConnectToSite() (auth.ClientI, error) {
	site, err := proxy.getSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this connects us to a node which is an auth server for this site
	// note the addres we're using: "@sitename", which in practice looks like "@{site-global-id}"
	// the Teleport proxy interprets such address as a request to connec to the active auth server
	// of the named site
	nodeClient, err := proxy.ConnectToNode("@"+site.Name, proxy.hostLogin)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// crate HTTP client to Auth API over SSH connection:
	sshDialer := func(network, addr string) (net.Conn, error) {
		return nodeClient.Client.Dial(network, addr)
	}
	clt, err := auth.NewClient("http://stub:0", sshDialer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(nodeAddress string, user string) (*NodeClient, error) {
	log.Infof("[CLIENT] connecting to node: %s", nodeAddress)
	e := trace.Errorf("unknown Error")

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + nodeAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// we have to try every auth method separatedly,
	// because go SSH will try only one (fist) auth method
	// of a given type, so if you have 2 different public keys
	// you have to try each one differently
	for _, authMethod := range proxy.authMethods {
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
		printErrors := func() {
			n, _ := io.Copy(os.Stderr, proxyErr)
			if n > 0 {
				os.Stderr.WriteString("\n")
			}
		}
		err = proxySession.RequestSubsystem("proxy:" + nodeAddress)
		if err != nil {
			defer printErrors()

			parts := strings.Split(nodeAddress, "@")
			siteName := parts[len(parts)-1]
			return nil, trace.Errorf("Failed connecting to cluster %v: %v", siteName, err)
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
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: proxy.hostKeyCallback,
		}
		conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
			nodeAddress, sshConfig)
		if err != nil {
			if utils.IsHandshakeFailedError(err) {
				e = trace.Wrap(err)
				proxySession.Close()
				continue
			}
			return nil, trace.Wrap(err)
		}
		client := ssh.NewClient(conn, chans, reqs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &NodeClient{Client: client, Proxy: proxy}, nil
	}
	if utils.IsHandshakeFailedError(e) {
		// remove the name of the site from the node address:
		parts := strings.Split(nodeAddress, "@")
		return nil, trace.Errorf(`access denied to login "%v" when connecting to %v: %v`, user, parts[0], e)
	}
	return nil, e
}

func (proxy *ProxyClient) Close() error {
	return proxy.Client.Close()
}

// Shell returns a configured remote shell (for a window of a requested size)
// as io.ReadWriterCloser object
//
// Parameters:
//	- width & height : size of the window
//  - session id     : ID of a session (if joining existing) or empty to create
//                     a new session
//  - env            : list of environment variables to set for a new session
//  - attachedTerm   : boolean indicating if this client is attached to a real terminal
func (client *NodeClient) Shell(nc *NodeSession) (io.ReadWriteCloser, error) {
	pipe, err := nc.allocateTerminal()
	// start the shell on the server:
	if err := nc.serverSession.Shell(); err != nil {
		return nil, trace.Wrap(err)
	}
	return pipe, err
}

// Run executes command on the remote server and writes its stdout to
// the 'output' argument
func (client *NodeClient) Run(cmd []string, stdin io.Reader, stdout, stderr io.Writer, env map[string]string) error {
	session, err := client.Client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	// pass environment variables set by client
	for key, val := range env {
		err = session.Setenv(key, val)
		if err != nil {
			log.Warn(err)
		}
	}
	session.Stdout = stdout
	session.Stderr = stderr
	session.Stdin = stdin
	return session.Run(strings.Join(cmd, " "))
}

// Upload uploads file or dir to the remote server
func (client *NodeClient) Upload(localSourcePath, remoteDestinationPath string, stderr io.Writer) error {
	file, err := os.Open(localSourcePath)
	if err != nil {
		return trace.Wrap(err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return trace.Wrap(err)
	}
	file.Close()

	scpConf := scp.Command{
		Source:      true,
		TargetIsDir: fileInfo.IsDir(),
		Recursive:   fileInfo.IsDir(),
		Target:      localSourcePath,
	}

	// "impersonate" scp to a server
	shellCmd := "/usr/bin/scp -t"
	if fileInfo.IsDir() {
		shellCmd += " -r"
	}
	shellCmd += " " + remoteDestinationPath

	return client.scp(scpConf, shellCmd, stderr)
}

// Download downloads file or dir from the remote server
func (client *NodeClient) Download(remoteSourcePath, localDestinationPath string, isDir bool, stderr io.Writer) error {
	scpConf := scp.Command{
		Sink:        true,
		TargetIsDir: isDir,
		Recursive:   isDir,
		Target:      localDestinationPath,
	}

	// "impersonate" scp to a server
	shellCmd := "/usr/bin/scp -f"
	if isDir {
		shellCmd += " -r"
	}
	shellCmd += " " + remoteSourcePath

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
