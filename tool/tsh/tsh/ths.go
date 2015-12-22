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
package tsh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
)

func Connect(user, address, proxyAddress, command string, agent agent.Agent) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer c.Close()

	if len(command) > 0 {
		out := bytes.Buffer{}
		err := c.Run(command, &out)
		if err != nil {
			return trace.Wrap(err, out.String())
		}
		fmt.Println(out.String())
		return nil
	}

	shell, err := c.Shell()
	if err != nil {
		return trace.Wrap(err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1000)
		for {
			n, err := shell.Read(buf)
			if err != nil && err != io.EOF {
				fmt.Println(trace.Wrap(err))
				return
			}
			if n > 0 {
				//fmt.Printf(string(buf[:n]))
				_, err = os.Stdout.Write(buf[:n])
				if err != nil {
					fmt.Println(trace.Wrap(err))
					return
				}
				/*err = os.Stdout.Sync()
				if err != nil {
					fmt.Println(trace.Wrap(err))
					return
				}*/
			}
		}
	}()
	go func() {
		_, err := io.Copy(shell, os.Stdin)
		if err != nil { // && err != io.EOF {
			fmt.Println(err.Error())
		}
		wg.Done()
	}()

	wg.Wait()

	return nil
}

func Upload(user, address, proxyAddress, localSourcePath, remoteDestPath string, agent agent.Agent) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer c.Close()

	err := c.Upload(localSourcePath, remoteDestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func Download(user, address, proxyAddress, localDestPath, remoteSourcePath string, isDir bool, agent agent.Agent) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, ssh.PublicKeysCallback(agent.Signers), user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer c.Close()

	err := c.Download(localDestPath, remoteSourcePath, isDir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func GetServers(user, proxyAddress, labelName, labelValueRegexp string, agent agent.Agent) error {
	proxyClient, err := client.ConnectToProxy(proxyAddress, ssh.PublicKeysCallback(agent.Signers), user)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	var servers []services.Server

	if (len(labelName) > 0) && (len(labelValueRegexp) > 0) {
		servers, err = proxyClient.FindServers(labelName, labelValueRegexp)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		servers, err = proxyClient.GetServers()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Println(servers)
	return nil
}
