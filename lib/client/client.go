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
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

type ProxyClient struct {
	*ssh.Client
	proxyAddress string
}

type NodeClient struct {
	*ssh.Client
}

func ConnectToProxy(proxyAddress string, authMethod ssh.AuthMethod, user string) (*ProxyClient, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{authMethod},
	}

	proxyClient, err := ssh.Dial("tcp", proxyAddress, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ProxyClient{
		Client:       proxyClient,
		proxyAddress: proxyAddress,
	}, nil
}

// Returns list of the nodes connected to the proxy
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
	<-done

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

// Returns list of the nodes which have labels "labelName" and
// corresponding values containing "labelValue"
func (proxy *ProxyClient) FindServers(labelName string,
	labelValue string) ([]services.Server, error) {

	allServers, err := proxy.GetServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	foundServers := make([]services.Server, 0)
	for _, srv := range allServers {
		alreadyAdded := false
		for name, label := range srv.CmdLabels {
			if name == labelName && strings.Contains(label.Result, labelValue) {
				foundServers = append(foundServers, srv)
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}
		for name, value := range srv.Labels {
			if name == labelName && strings.Contains(value, labelValue) {
				foundServers = append(foundServers, srv)
				break
			}
		}
	}

	return foundServers, nil
}

// Connects to Node via Proxy
func (proxy *ProxyClient) ConnectToNode(nodeAddress string, authMethod ssh.AuthMethod, user string) (*NodeClient, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{authMethod},
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

	err = proxySession.RequestSubsystem(fmt.Sprintf("proxy:%v", nodeAddress))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localAddr, err := net.ResolveTCPAddr("tcp", proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteAddr, err := net.ResolveTCPAddr("tcp", nodeAddress)
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

	conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
		nodeAddress, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := ssh.NewClient(conn, chans, reqs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &NodeClient{Client: client}, nil
}

func ConnectToNode(nodeAddress string, authMethod ssh.AuthMethod, user string) (*NodeClient, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{authMethod},
	}

	client, err := ssh.Dial("tcp", nodeAddress, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &NodeClient{Client: client}, nil
}

// Returns remote shell as io.ReadWriterCloser object
func (client *NodeClient) Shell() (io.ReadWriteCloser, error) {
	session, err := client.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	writer, err := session.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stdout := &bytes.Buffer{}
	session.Stdout = stdout

	err = session.Shell()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return utils.NewPipeNetConn(
		stdout,
		writer,
		utils.MultiCloser(writer, session),
		&net.IPAddr{},
		&net.IPAddr{},
	), nil
}

func (client *NodeClient) Run(cmd string) (string, error) {
	session, err := client.Client.NewSession()
	if err != nil {
		return "", trace.Wrap(err)
	}

	out, err := session.Output(cmd)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(out), nil
}
