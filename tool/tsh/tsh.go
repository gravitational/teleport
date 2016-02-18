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
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strings"
	"sync"
	"syscall"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/hangout"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func SSH(target, proxyAddress, command, port, sessionID string, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {
	user, target := client.SplitUserAndAddress(target)
	if !strings.Contains(target, ":") {
		target += ":" + port
	}

	if len(user) == 0 {
		return fmt.Errorf("Error: please provide user name")
	}
	if len(command) > 0 {
		return client.RunCmd(user, target, proxyAddress, command, authMethods, hostKeyCallback)
	}

	addresses, err := client.ParseTargetServers(target, user, proxyAddress,
		authMethods, hostKeyCallback)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(addresses) > 1 {
		return fmt.Errorf("Shell can't be run on multiple target servers")
	}
	address := addresses[0]

	var c *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		c, err = proxyClient.ConnectToNode(address, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		c, err = client.ConnectToNode(nil, address, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return shell(c, address, sessionID)
}

func shell(c *client.NodeClient, address string, sessionID string) error {
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
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		os.Exit(0)
	}()

	width, height, err := getTerminalSize()
	if err != nil {
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		return trace.Wrap(err)
	}

	shell, err := c.Shell(width, height, sessionID)
	if err != nil {
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
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
		fmt.Printf("\nConnection to %s closed from the remote side\n", address)
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		os.Exit(0)
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
					// restore the console echoing state when exiting
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
	// restore the console echoing state when exiting
	exec.Command("stty", "-F", "/dev/tty", "echo").Run()
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

func GetServers(proxyAddress, labelName, labelValueRegexp string, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {
	user, proxyAddress := client.SplitUserAndAddress(proxyAddress)
	if len(user) == 0 {
		return fmt.Errorf("Error: please provide user name")
	}
	proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, hostKeyCallback, user)
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

func SCP(proxyAddress, source, dest string, isDir bool, port string, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {
	if strings.Contains(source, ":") {
		user, source := client.SplitUserAndAddress(source)
		if len(user) == 0 {
			return fmt.Errorf("Error: please provide user name")
		}

		parts := strings.Split(source, ":")
		path := parts[len(parts)-1]
		targetServers := strings.Join(parts[0:len(parts)-1], ":")
		if !strings.Contains(targetServers, ":") {
			targetServers += ":" + port
		}
		return client.Download(user, targetServers, proxyAddress, path,
			dest, isDir, authMethods, hostKeyCallback)
	} else {
		user, dest := client.SplitUserAndAddress(dest)
		if len(user) == 0 {
			return fmt.Errorf("Error: please provide user name")
		}
		parts := strings.Split(dest, ":")
		path := parts[len(parts)-1]
		targetServers := strings.Join(parts[0:len(parts)-1], ":")
		if !strings.Contains(targetServers, ":") {
			targetServers += ":" + port
		}

		return client.Upload(user, targetServers, proxyAddress, source,
			path, authMethods, hostKeyCallback)
	}
	return nil
}

func Share(proxyAddress, hangoutProxyAddress, nodeListeningAddress,
	authListeningAddress string, readOnly bool, authMethods []ssh.AuthMethod,
	hostKeyCallback utils.HostKeyCallback) error {

	hangoutServer, err := hangout.New(hangoutProxyAddress, nodeListeningAddress,
		authListeningAddress, readOnly, authMethods, hostKeyCallback)

	url := proxyAddress + "/hangout/" + hangoutServer.HangoutID

	fmt.Printf("\nURL:\n\n%v\n\n", url)

	if err != nil {
		return trace.Wrap(err)
	}

	u, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}

	return SSH(u.Username+"@"+nodeListeningAddress, "", "", "", "hangoutSession", []ssh.AuthMethod{hangoutServer.ClientAuthMethod}, hangoutServer.HostKeyCallback)
}

func Join(hangoutURL string, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {
	/*
		// Debug Mode
		hangoutServer, err := hangout.New("localhost:33009", "localhost:33010",
			"localhost:33011", false, authMethods, hostKeyCallback)
		if err != nil {
			return trace.Wrap(err)
		}
		time.Sleep(time.Second * 1)

		hangoutURL = "localhost:33008" + "/hangout/" + hangoutServer.HangoutID
	*/
	urlParts := strings.Split(hangoutURL, "/")
	if len(urlParts) < 3 {
		return trace.Errorf("invalid URL")
	}

	proxyAddress := strings.Join(urlParts[:len(urlParts)-2], "/")
	hangoutID := urlParts[len(urlParts)-1 : len(urlParts)][0]

	proxy, err := client.ConnectToProxy(proxyAddress, authMethods, hostKeyCallback, "123")
	if err != nil {
		return trace.Wrap(err)
	}

	authConn, err := proxy.ConnectToHangout(hangoutID+":"+utils.HangoutAuthPortAlias, authMethods)
	if err != nil {
		return trace.Wrap(err)
	}

	authClient, err := auth.NewClientFromSSHClient(authConn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	nodeAuthMethod, err := hangout.Authorize(authClient)
	if err != nil {
		return trace.Wrap(err)
	}

	nodeConn, err := proxy.ConnectToHangout(hangoutID+":"+utils.HangoutNodePortAlias, []ssh.AuthMethod{nodeAuthMethod})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Connected\n")

	return shell(nodeConn, hangoutID, "hangoutSession")
}
