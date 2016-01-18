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
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func RunCmd(user, target, proxyAddress, command string, authMethods []ssh.AuthMethod) error {
	addresses, err := parseAddress(target, user, proxyAddress,
		authMethods)
	if err != nil {
		return trace.Wrap(err)
	}

	var proxyClient *client.ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error

	for _, address := range addresses {
		fmt.Printf("Running command on %v...\n", address)
		var c *client.NodeClient
		if len(proxyAddress) > 0 {
			c, err = proxyClient.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		} else {
			var err error
			c, err = client.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		}
		defer c.Close()

		out := bytes.Buffer{}
		err := c.Run(command, &out)
		if err != nil {
			e = err
			fmt.Println("Error:", err.Error())
			continue
		}
		fmt.Printf(out.String())
		fmt.Printf("Disconnected from %v\n\n", address)

	}

	if e != nil {
		return fmt.Errorf("SSH finished with errors")
	} else {
		return nil
	}
}

func SSH(target, proxyAddress, command string, authMethods []ssh.AuthMethod) error {
	user, target := splitUserAndAddress(target)
	if len(user) == 0 {
		return fmt.Errorf("Error: please provide user name")
	}
	if len(command) > 0 {
		return RunCmd(user, target, proxyAddress, command, authMethods)
	}

	addresses, err := parseAddress(target, user, proxyAddress,
		authMethods)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(addresses) > 1 {
		return fmt.Errorf("Shell can't be run on multiple target servers")
	}
	address := addresses[0]

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

func Upload(user string, addresses []string, proxyAddress, localSourcePath, remoteDestPath string, authMethods []ssh.AuthMethod) error {
	var err error
	var proxyClient *client.ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error

	for _, address := range addresses {
		fmt.Printf("Uploading to %v\n", address)
		var c *client.NodeClient
		if len(proxyAddress) > 0 {
			c, err = proxyClient.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		} else {
			var err error
			c, err = client.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		}
		defer c.Close()

		fmt.Println("***UPLOAD", localSourcePath, remoteDestPath)
		err := c.Upload(localSourcePath, remoteDestPath)
		if err != nil {
			e = err
			fmt.Println("Error:", err.Error())
			continue
		}
		fmt.Printf("Disconnected from %v\n\n", address)
	}

	if e != nil {
		return fmt.Errorf("SCP finished with errors")
	} else {
		return nil
	}
}

func Download(user string, addresses []string, proxyAddress, remoteSourcePath, localDestPath string, isDir bool, authMethods []ssh.AuthMethod) error {
	_, filename := filepath.Split(remoteSourcePath)
	if len(addresses) > 1 {
		localDestPath = filepath.Join(localDestPath, filename)

		_, err := os.Stat(localDestPath)
		if os.IsNotExist(err) {
			err = os.MkdirAll(localDestPath, os.ModeDir|0777)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err != nil {
				return trace.Wrap(err)
			} else {
				return trace.Errorf("Error: Directory %v already exists", localDestPath)
			}
		}
	}

	var err error
	var proxyClient *client.ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error

	for _, address := range addresses {
		fmt.Printf("Downloading from %v\n", address)
		var c *client.NodeClient
		if len(proxyAddress) > 0 {
			c, err = proxyClient.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		} else {
			c, err = client.ConnectToNode(address, authMethods, user)
			if err != nil {
				e = err
				fmt.Println("Error:", err.Error())
				continue
			}
		}
		defer c.Close()
		dest := localDestPath
		if len(addresses) > 1 {
			dest = filepath.Join(localDestPath, address)
		}

		err := c.Download(remoteSourcePath, dest, isDir)
		if err != nil {
			e = err
			fmt.Println("Error:", err.Error())
			continue
		}
		fmt.Printf("Disconnected from %v\n\n", address)
	}

	if e != nil {
		return fmt.Errorf("SCP finished with errors")
	} else {
		return nil
	}
}

func GetServers(proxyAddress, labelName, labelValueRegexp string, authMethods []ssh.AuthMethod) error {
	user, proxyAddress := splitUserAndAddress(proxyAddress)
	if len(user) == 0 {
		return fmt.Errorf("Error: please provide user name")
	}
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

func SCP(proxyAddress, source, dest string, isDir bool, authMethods []ssh.AuthMethod) error {
	if strings.Contains(source, ":") {
		user, source := splitUserAndAddress(source)
		if len(user) == 0 {
			return fmt.Errorf("Error: please provide user name")
		}

		parts := strings.Split(source, ":")
		path := parts[len(parts)-1]
		target := strings.Join(parts[0:len(parts)-1], ":")
		addresses, err := parseAddress(target, user, proxyAddress,
			authMethods)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(addresses) == 0 {
			return fmt.Errorf("No target servers found")
		}
		return Download(user, addresses, proxyAddress, path,
			dest, isDir, authMethods)
	} else {
		user, dest := splitUserAndAddress(dest)
		if len(user) == 0 {
			return fmt.Errorf("Error: please provide user name")
		}
		parts := strings.Split(dest, ":")
		path := parts[len(parts)-1]
		target := strings.Join(parts[0:len(parts)-1], ":")
		addresses, err := parseAddress(target, user, proxyAddress,
			authMethods)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(addresses) == 0 {
			return fmt.Errorf("No target servers found")
		}
		return Upload(user, addresses, proxyAddress, source,
			path, authMethods)
	}
	return nil
}

func parseAddress(addr string, user, proxyAddress string, authMethods []ssh.AuthMethod) ([]string, error) {
	if addr[0] == '_' {
		// address is a label:value pair
		addr = addr[1:len(addr)]
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			return nil, trace.Errorf("Wrong address format, label address should have _label:value format")
		}
		label := parts[0]
		value := parts[1]

		if len(proxyAddress) == 0 {
			return nil, trace.Errorf("Proxy Address should be provided for server searching")
		}

		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer proxyClient.Close()

		var servers []services.Server

		if len(label) > 0 {
			servers, err = proxyClient.FindServers(label, value)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			servers, err = proxyClient.GetServers()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		resultAddresses := []string{}
		for _, server := range servers {
			resultAddresses = append(resultAddresses, server.Addr)
		}
		return resultAddresses, nil
	} else {
		return []string{addr}, nil
	}
}

func splitUserAndAddress(target string) (user, address string) {
	if !strings.Contains(target, "@") {
		return "", address
	}

	parts := strings.Split(target, "@")
	user = parts[0]
	address = strings.Join(parts[1:len(parts)], "@")
	return user, address
}
