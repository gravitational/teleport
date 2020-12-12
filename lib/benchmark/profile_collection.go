/*
Copyright 2020 Gravitational, Inc.
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

package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

const (
	cpuEndpoint       = "http://127.0.0.1:3434/debug/pprof/profile"
	goroutineEndpoint = "http://127.0.0.1:3434/debug/pprof/goroutine"
	heapEndpoint      = "http://127.0.0.1:3434/debug/pprof/heap"
)

// collectProfiles collects cpu, heap, and goroutine profiles
func collectProfiles(ctx context.Context, prefix, path, remoteUser string, tc *client.TeleportClient) error {
	var profileCommands = new([3]string)
	endpoints := []string{cpuEndpoint, goroutineEndpoint, heapEndpoint}
	files := []string{"cpu.profile", "goroutine.profile", "heap.profile"}
	ok, err := exists(path)
	if !ok || err != nil {
		return trace.Wrap(err)
	}
	for i := range endpoints {
		profileCommands[i] = fmt.Sprintf("%s %s_%s %s", "curl -o", prefix, files[i], endpoints[i])
		files[i] = fmt.Sprintf("%s_%s", prefix, files[i])
	}
	config := tc.Config
	client, err := client.NewClient(&config)
	if err != nil {
		return trace.Wrap(err)
	}
	out := &bytes.Buffer{}
	tc.Stdout = out
	tc.Stderr = out
	err = statusOK(tc, out)
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range profileCommands {
		err = client.SSH(context.TODO(), strings.Split(profileCommands[i], " "), false)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = scpProfiles(files, path, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// exists checks if path exists
func exists(path string) (bool, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

// statusOK checks diagnostic endpoint status code on the remote server
func statusOK(tc *client.TeleportClient, out *bytes.Buffer) error {
	command := fmt.Sprint("curl -s -w \"%{http_code}\n\" http://127.0.0.1:3434/debug/pprof/ -o /dev/null")
	err := tc.SSH(context.TODO(), strings.Split(command, " "), false)
	if (err == nil) && !(strings.Contains(out.String(), "200")) {
		return nil
	}
	return trace.Wrap(err, ": make sure --diag-addr and -d flags are enabled on server start up")
}

// scpProfiles securely copies all profiles from remote server to local
func scpProfiles(files []string, path string, tc *client.TeleportClient) error {
	var remote string
	config := tc.Config
	client, err := client.NewClient(&config)
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range files {
		remote = fmt.Sprintf("%s@%s:%s", tc.HostLogin, tc.Host, files[i])
		log.Println("securley copying... ", remote, " --> ", path)
		err := client.SCP(context.TODO(), []string{remote, path}, tc.HostPort, false, true)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
