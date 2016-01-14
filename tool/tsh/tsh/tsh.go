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
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func Connect(user, address, proxyAddress, command string, authMethods []ssh.AuthMethod) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, authMethods, user)
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
		fmt.Printf(out.String())
		return nil
	}

	// disable input buffering
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	// do not display entered characters on the screen
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

	// Catch term signals
	exitSignals := make(chan os.Signal, 1)
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		<-exitSignals
		fmt.Printf("\nConnection to %s closed\n", address)
		// restore the echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		os.Exit(0)
	}()

	width, height, err := getTerminalSize()
	if err != nil {
		return trace.Wrap(err)
	}

	shell, err := c.Shell(width, height)
	if err != nil {
		return trace.Wrap(err)
	}

	// Catch Ctrl-C signal
	ctrlCSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlCSignal, syscall.SIGINT)
	go func() {
		for {
			<-ctrlCSignal
			_, err := shell.Write([]byte{3})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()

	// Catch Ctrl-Z signal
	ctrlZSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlZSignal, syscall.SIGTSTP)
	go func() {
		for {
			<-ctrlZSignal
			_, err := shell.Write([]byte{26})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// copy from the remote shell to the local
	go func() {
		_, err := io.Copy(os.Stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
		wg.Done()
	}()

	// copy from the local shell to the remote
	go func() {
		defer wg.Done()
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				fmt.Println(trace.Wrap(err))
				return
			}
			if n > 0 {
				// catch Ctrl-D
				if buf[0] == 4 {
					fmt.Printf("\nConnection to %s closed\n", address)
					// restore the echoing state when exiting
					exec.Command("stty", "-F", "/dev/tty", "echo").Run()
					os.Exit(0)
				}
				_, err = shell.Write(buf[:n])
				if err != nil {
					fmt.Println(trace.Wrap(err))
					return
				}
			}
		}
	}()

	wg.Wait()

	return nil
}

// getTerminalSize() returns current local terminal size
func getTerminalSize() (width int, height int, e error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	n, err := fmt.Sscan(string(out), &height, &width)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	if n != 2 {
		return 0, 0, trace.Errorf("Can't get terminal size")
	}

	return width, height, nil
}

func Upload(user, address, proxyAddress, localSourcePath, remoteDestPath string, authMethods []ssh.AuthMethod) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, authMethods, user)
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

func Download(user, address, proxyAddress, remoteSourcePath, localDestPath string, isDir bool, authMethods []ssh.AuthMethod) error {
	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(address, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer c.Close()

	err := c.Download(remoteSourcePath, localDestPath, isDir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func GetServers(user, proxyAddress, labelName, labelValueRegexp string, authMethods []ssh.AuthMethod) error {
	proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, user)
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
	for _, server := range servers {
		fmt.Printf("%v(%v)\n", server.Hostname, server.Addr)
		for name, value := range server.Labels {
			fmt.Printf("\t%v: %v\n", name, value)
		}
		for name, value := range server.CmdLabels {
			fmt.Printf("\t%v: %v\n", name, value.Result)
		}

	}
	return nil
}
