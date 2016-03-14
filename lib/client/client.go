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

// Package client contains SSH and HTTPS clients to connect
// to the Teleport proxy
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
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
	proxyAddress    string
	hostKeyCallback utils.HostKeyCallback
	authMethods     []ssh.AuthMethod
}

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Client *ssh.Client
}

// GetSites returns list of the "sites" (AKA teleport clusters) connected to the proxy
// Each site is returned as an instance of its auth server
//
// NOTE: this version of teleport supports only one site per proxy
func (proxy *ProxyClient) GetSites() ([]services.Site, error) {
	proxySession, err := proxy.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stdout := &bytes.Buffer{}
	reader, err := proxySession.StdoutPipe()
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
	case <-time.After(10 * time.Second):
		return nil, trace.Errorf("timeout")
	}

	log.Infof("proxyClient.GetSites() returned: %v", stdout.String())

	var sites []services.Site
	if err := json.Unmarshal(stdout.Bytes(), &sites); err != nil {
		return nil, trace.Wrap(err)
	}
	return sites, nil
}

// FindServers returns list of the nodes which have labels "labelName" and
// corresponding values matches "labelValueRegexp"
func (proxy *ProxyClient) FindServers(labelName string, labelRegex string) ([]services.Server, error) {
	regex, err := regexp.Compile(labelRegex)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// see which sites (AKA auth servers) this proxy is connected to
	_, err = proxy.GetSites()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: ev. we need to take the 1st site and, using its auth API,
	// request a list of servers from it. and THAT would be stored in "allServers"
	var allServers []services.Server
	foundServers := make([]services.Server, 0)
	for _, srv := range allServers {
		alreadyAdded := false
		for name, label := range srv.CmdLabels {
			if name == labelName && regex.MatchString(label.Result) {
				foundServers = append(foundServers, srv)
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}
		for name, value := range srv.Labels {
			if name == labelName && regex.MatchString(value) {
				foundServers = append(foundServers, srv)
				break
			}
		}
	}

	return foundServers, nil
}

// ConnectToSite connects to the auth server of the given site via proxy.
// It returns connected and authenticated auth server client
func (proxy *ProxyClient) ConnectToSite(site *services.Site, user string) (authC *auth.Client, err error) {
	var nc *NodeClient
	for _, authServer := range site.AuthServers {
		nc, err = proxy.ConnectToNode(authServer.Addr, user)
		if err != nil {
			log.Error(err)
			continue
		}
		fmt.Println("I got the node client!", nc)
	}
	return authC, err
}

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(nodeAddress string, user string) (*NodeClient, error) {
	log.Debugf("connecting to node: %s", nodeAddress)
	e := trace.Errorf("unknown Error")

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
			buf := &bytes.Buffer{}
			io.Copy(buf, proxyErr)
			if buf.String() != "" {
				fmt.Println("ERROR: " + buf.String())
			}
		}
		err = proxySession.RequestSubsystem("proxy:" + nodeAddress)
		if err != nil {
			defer printErrors()
			return nil, trace.Wrap(err)
		}
		localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		remoteAddr, err := utils.ParseAddr("tcp://" + nodeAddress)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pipeNetConn := utils.NewPipeNetConn(
			proxyReader,
			proxyWriter,
			proxySession,
			localAddr,
			remoteAddr,
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
		return &NodeClient{Client: client}, nil
	}
	return nil, e
}

func (proxy *ProxyClient) Close() error {
	return proxy.Client.Close()
}

// ConnectToNode returns connected and authenticated NodeClient
func ConnectToNode(optionalProxy *ProxyClient, nodeAddress string, authMethods []ssh.AuthMethod,
	hostKeyCallback utils.HostKeyCallback, user string) (*NodeClient, error) {
	if optionalProxy != nil {
		return optionalProxy.ConnectToNode(nodeAddress, user)
	}

	e := trace.Errorf("no authMethods were provided")

	for _, authMethod := range authMethods {
		sshConfig := &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: hostKeyCallback,
		}
		log.Debugf("connecting to SSH node: %v", nodeAddress)
		client, err := ssh.Dial("tcp", nodeAddress, sshConfig)
		if err != nil {
			if utils.IsHandshakeFailedError(err) {
				e = trace.Wrap(err)
				continue
			}
			return nil, trace.Wrap(err)
		}

		return &NodeClient{Client: client}, nil
	}

	return nil, e
}

// Shell returns remote shell as io.ReadWriterCloser object
func (client *NodeClient) Shell(width, height int, sessionID string) (io.ReadWriteCloser, error) {
	session, err := client.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(sessionID) > 0 {
		err = session.Setenv(sshutils.SessionEnvVar, sessionID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// pass language info into the remote session.
	// TODO: in the future support passing of arbitrary environment variables
	evarsToPass := []string{"LANG", "LANGUAGE"}
	for _, evar := range evarsToPass {
		if value := os.Getenv(evar); value != "" {
			err = session.Setenv(evar, value)
			if err != nil {
				log.Warn(err)
			}
		}
	}

	terminalModes := ssh.TerminalModes{}

	err = session.RequestPty("xterm", height, width, terminalModes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	writer, err := session.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reader, err := session.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		buf := &bytes.Buffer{}
		io.Copy(buf, stderr)
		if buf.String() != "" {
			fmt.Println("ERROR: " + buf.String())
		}
	}()

	err = session.Shell()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return utils.NewPipeNetConn(
		reader,
		writer,
		utils.MultiCloser(writer, session),
		&net.IPAddr{},
		&net.IPAddr{},
	), nil
}

// Run executes command on the remote server and writes its stdout to
// the 'output' argument
func (client *NodeClient) Run(cmd string, output io.Writer) error {
	session, err := client.Client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}

	session.Stdout = output

	if err := session.Run(cmd); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upload uploads file or dir to the remote server
func (client *NodeClient) Upload(localSourcePath, remoteDestinationPath string) error {
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

	shellCmd := "/usr/bin/scp -t"
	if fileInfo.IsDir() {
		shellCmd += " -r"
	}
	shellCmd += " " + remoteDestinationPath

	return client.scp(scpConf, shellCmd)
}

// Download downloads file or dir from the remote server
func (client *NodeClient) Download(remoteSourcePath, localDestinationPath string, isDir bool) error {
	scpConf := scp.Command{
		Sink:        true,
		TargetIsDir: isDir,
		Recursive:   isDir,
		Target:      localDestinationPath,
	}

	shellCmd := "/usr/bin/scp -f"
	if isDir {
		shellCmd += " -r"
	}
	shellCmd += " " + remoteSourcePath

	return client.scp(scpConf, shellCmd)
}

// scp runs remote scp command(shellCmd) on the remote server and
// runs local scp handler using scpConf
func (client *NodeClient) scp(scpConf scp.Command, shellCmd string) error {
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

	stderr, err := session.StderrPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	serverErrors := make(chan error, 2)
	go func() {
		var errMsg bytes.Buffer
		io.Copy(&errMsg, stderr)
		if len(errMsg.Bytes()) > 0 {
			serverErrors <- trace.Errorf(errMsg.String())
		} else {
			close(serverErrors)
		}
	}()

	ch := utils.NewPipeNetConn(
		stdout,
		stdin,
		utils.MultiCloser(),
		&net.IPAddr{},
		&net.IPAddr{},
	)

	scpServer, err := scp.New(scpConf)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		err := scpServer.Serve(ch)
		if err != nil {
			log.Errorf(err.Error())
		}
		stdin.Close()
	}()

	err = session.Run(shellCmd)

	select {
	case serverError := <-serverErrors:
		return trace.Wrap(serverError)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (client *NodeClient) Close() error {
	return client.Client.Close()
}
