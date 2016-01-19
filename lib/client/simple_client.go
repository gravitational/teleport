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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

// RunCmd runs provided command on the target servers and
// prints result to stdout,
// target can be like "127.0.0.1:1234" or "_label:value".
func RunCmd(user, target, proxyAddress, command string, authMethods []ssh.AuthMethod) error {
	addresses, err := ParseTargetServers(target, user, proxyAddress,
		authMethods)
	if err != nil {
		return trace.Wrap(err)
	}

	var proxyClient *ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error
	stdoutMutex := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			output, err := runCmd(user, address, proxyClient, command, authMethods)
			stdoutMutex.Lock()
			defer stdoutMutex.Unlock()
			fmt.Printf("Running command on %v\n", address)
			fmt.Printf("-----------------------------\n")
			if err != nil {
				e = err
				fmt.Println(err.Error())
			} else {
				fmt.Printf(output)
			}
			fmt.Printf("-----------------------------\n\n")
		}(address)
	}

	wg.Wait()

	if e != nil {
		return fmt.Errorf("SSH finished with errors")
	} else {
		return nil
	}
}

// runCmd runs command on provided server and returns the output as string
func runCmd(user, address string,
	proxyClient *ProxyClient, command string,
	authMethods []ssh.AuthMethod) (output string, e error) {

	c, err := ConnectToNode(proxyClient, address, authMethods, user)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer c.Close()

	out := bytes.Buffer{}
	err = c.Run(command, &out)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return out.String(), nil
}

// Upload uploads file or dir to the target servers,
// target can be like "127.0.0.1:1234" or "_label:value".
// Processes for each server work in parallel
func Upload(user, target, proxyAddress, localSourcePath, remoteDestPath string, authMethods []ssh.AuthMethod) error {
	addresses, err := ParseTargetServers(target, user, proxyAddress, authMethods)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("No target servers found")
	}

	var proxyClient *ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error
	stdoutMutex := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()

			err := upload(user, address, proxyClient,
				localSourcePath, remoteDestPath, authMethods)

			stdoutMutex.Lock()
			defer stdoutMutex.Unlock()

			if err != nil {
				e = err
				fmt.Printf("Error uploading to %v: %v\n", address,
					err.Error())
			} else {
				fmt.Printf("Finished uploading to %v\n", address)
			}
		}(address)
	}

	wg.Wait()

	if e != nil {
		return e
	} else {
		return nil
	}
}

// upload uploads file or dir to the provided server
func upload(user, srvAddress string, proxyClient *ProxyClient,
	localSourcePath, remoteDestPath string,
	authMethods []ssh.AuthMethod) error {

	c, err := ConnectToNode(proxyClient, srvAddress, authMethods, user)
	if err != nil {
		return trace.Wrap(err)
	}
	defer c.Close()

	err = c.Upload(localSourcePath, remoteDestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Download downloads file or dir from target servers,
// target can be like "127.0.0.1:1234" or "_label:value".
// Processes for each server work in parallel.
// If there are more than one target server, result files will be
// arranged in a folder.
func Download(user, target, proxyAddress, remoteSourcePath, localDestPath string, isDir bool, authMethods []ssh.AuthMethod) error {
	addresses, err := ParseTargetServers(target, user, proxyAddress, authMethods)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("No target servers found")
	}

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

	var proxyClient *ProxyClient
	if len(proxyAddress) > 0 {
		proxyClient, err = ConnectToProxy(proxyAddress, authMethods, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
	}

	var e error
	stdoutMutex := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			dest := localDestPath
			if len(addresses) > 1 {
				dest = filepath.Join(localDestPath, address)
			}

			err := download(user, address, proxyClient,
				remoteSourcePath, dest, isDir, authMethods)

			stdoutMutex.Lock()
			defer stdoutMutex.Unlock()

			if err != nil {
				e = err
				fmt.Printf("Error downloading from %v: %v\n", address,
					err.Error())
			} else {
				fmt.Printf("Finished downloading from %v:%v to %v\n",
					address, remoteSourcePath, dest)
			}
		}(address)
	}

	wg.Wait()

	if e != nil {
		return e
	} else {
		return nil
	}
}

// download downloads file or dir from provided server
func download(user, srvAddress string, proxyClient *ProxyClient,
	remoteSourcePath, localDestPath string, isDir bool,
	authMethods []ssh.AuthMethod) error {

	c, err := ConnectToNode(proxyClient, srvAddress, authMethods, user)
	if err != nil {
		return trace.Wrap(err)
	}
	defer c.Close()

	err = c.Download(remoteSourcePath, localDestPath, isDir)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ParseTargetServers parses target to an array of server addresses,
// target can be like "127.0.0.1:1234" or "_label:value".
// If "_label:value" provided, it connects to the proxy server and
// finds target servers
func ParseTargetServers(target string, user, proxyAddress string, authMethods []ssh.AuthMethod) ([]string, error) {
	if target[0] == '_' {
		// address is a label:value pair
		target = target[1:len(target)]
		parts := strings.Split(target, ":")
		if len(parts) != 2 {
			return nil, trace.Errorf("Wrong address format, label address should have _label:value format")
		}
		label := parts[0]
		value := parts[1]

		if len(proxyAddress) == 0 {
			return nil, trace.Errorf("Proxy Address should be provided for server searching")
		}

		proxyClient, err := ConnectToProxy(proxyAddress, authMethods, user)
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
		return []string{target}, nil
	}
}

// SplitUserAndAddress splits target into user and address using "@"
// as delimiter. If target doesn't contain "@", it returns empty user
// and target as address
func SplitUserAndAddress(target string) (user, address string) {
	if !strings.Contains(target, "@") {
		return "", target
	}

	parts := strings.Split(target, "@")
	user = parts[0]
	address = strings.Join(parts[1:len(parts)], "@")
	return user, address
}
