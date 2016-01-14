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
// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

// ProxyClient implements ssh client to a teleport proxy
// It can provide list of nodes or connect to nodes
type ProxyClient struct {
	Client       *ssh.Client
	proxyAddress string
}

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Client *ssh.Client
}

// ConnectToProxy returns connected and authenticated ProxyClient
func ConnectToProxy(proxyAddress string, authMethods []ssh.AuthMethod,
	user string) (*ProxyClient, error) {
	e := trace.Errorf("No authMethods were provided")

	for _, authMethod := range authMethods {
		sshConfig := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{authMethod},
		}

		proxyClient, err := ssh.Dial("tcp", proxyAddress, sshConfig)
		if err != nil {
			if strings.Contains(err.Error(), "handshake failed") {
				e = trace.Wrap(err)
				continue
			}
			return nil, trace.Wrap(err)
		}

		return &ProxyClient{
			Client:       proxyClient,
			proxyAddress: proxyAddress,
		}, nil
	}

	return nil, e
}

// GetServers returns list of the nodes connected to the proxy
func (proxy *ProxyClient) GetServers() ([]services.Server, error) {
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

	servers := make(map[string][]services.Server)
	if err := json.Unmarshal(stdout.Bytes(), &servers); err != nil {
		return nil, trace.Wrap(err)
	}
	serversList := make([]services.Server, 0)

	for _, srvs := range servers {
		serversList = append(serversList, srvs...)
	}

	return serversList, nil
}

// FindServers returns list of the nodes which have labels "labelName" and
// corresponding values matches "labelValueRegexp"
func (proxy *ProxyClient) FindServers(labelName string,
	labelValueRegexp string) ([]services.Server, error) {

	labelRegex, err := regexp.Compile(labelValueRegexp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allServers, err := proxy.GetServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	foundServers := make([]services.Server, 0)
	for _, srv := range allServers {
		alreadyAdded := false
		for name, label := range srv.CmdLabels {
			if name == labelName && labelRegex.MatchString(label.Result) {
				foundServers = append(foundServers, srv)
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}
		for name, value := range srv.Labels {
			if name == labelName && labelRegex.MatchString(value) {
				foundServers = append(foundServers, srv)
				break
			}
		}
	}

	return foundServers, nil
}

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(nodeAddress string, authMethods []ssh.AuthMethod, user string) (*NodeClient, error) {
	if len(authMethods) == 0 {
		return nil, trace.Errorf("No authMethods were provided")
	}

	e := trace.Errorf("Unknown Error")

	for _, authMethod := range authMethods {

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

		err = proxySession.RequestSubsystem(fmt.Sprintf("proxy:%v", nodeAddress))
		if err != nil {
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
			User: user,
			Auth: []ssh.AuthMethod{authMethod},
		}

		conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
			nodeAddress, sshConfig)
		if err != nil {
			if strings.Contains(err.Error(), "handshake failed") {
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
func ConnectToNode(nodeAddress string, authMethods []ssh.AuthMethod, user string) (*NodeClient, error) {
	e := trace.Errorf("No authMethods were provided")

	for _, authMethod := range authMethods {
		sshConfig := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{authMethod},
		}

		client, err := ssh.Dial("tcp", nodeAddress, sshConfig)
		if err != nil {
			if strings.Contains(err.Error(), "handshake failed") {
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
func (client *NodeClient) Shell(width, height int) (io.ReadWriteCloser, error) {
	session, err := client.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
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
	done := make(chan struct{})

	go func() {
		scpServer.Serve(ch)
		stdin.Close()
		close(done)
	}()

	err = session.Run(shellCmd)
	if err != nil {
		return trace.Wrap(err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		return trace.Errorf("timeout")
	}

	return nil
}

func (client *NodeClient) Close() error {
	return client.Client.Close()
}
