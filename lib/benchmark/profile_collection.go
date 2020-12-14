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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	// CPU command takes 30 seconds, a minute will be added to the benchmark (before and after, collecting the profiles during happens concurrently).
	// If you cancel the benchmark during collecting cpu data, you will need to wait until the cpu command is done on the remote server (no more than ~30s)
	// Benchmarks will need to be at least one minute long if you would like to collect profiles.
	cpuEndpoint          = "http://127.0.0.1:3434/debug/pprof/profile"
	goroutineEndpoint    = "http://127.0.0.1:3434/debug/pprof/goroutine"
	heapEndpoint         = "http://127.0.0.1:3434/debug/pprof/heap"
	checkEndpointCommand = "curl -s -w \"%{http_code}\n\" http://127.0.0.1:3434/debug/pprof/ -o /dev/null"
)

// collectProfiles collects cpu, heap, and goroutine profiles
func collectProfiles(ctx context.Context, prefix, path string, tc *client.TeleportClient) error {
	var endpoints = []string{cpuEndpoint, goroutineEndpoint, heapEndpoint}
	var timeStamp = time.Now().Format("2006-01-02_15:04:05")
	var files = []string{fmt.Sprintf("%s_cpu-%s.profile", prefix, timeStamp),
		fmt.Sprintf("%s_goroutine-%s.profile", prefix, timeStamp),
		fmt.Sprintf("%s_heap-%s.profile", prefix, timeStamp)}

	err := checkPath(path)
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
	for i := range endpoints {
		cmd := fmt.Sprintf("%s %s %s", "curl -o", files[i], endpoints[i])
		err = tc.SSH(ctx, strings.Split(cmd, " "), false)
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

// checkPath checks if path is ok to save profiles to
func checkPath(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// statusOK checks diagnostic endpoint status code on the remote server
func statusOK(tc *client.TeleportClient, out *bytes.Buffer) error {
	err := tc.SSH(context.TODO(), strings.Split(checkEndpointCommand, " "), false)
	if (err == nil) && !(strings.Contains(out.String(), "200")) {
		return nil
	}
	return trace.Wrap(err, ": be sure --diag-addr and -d flags are were enabled on server startup.")
}

// scpProfiles securely copies all profiles from remote server to local
func scpProfiles(files []string, path string, tc *client.TeleportClient) error {
	var remote string
	for i := range files {
		remote = fmt.Sprintf("%s@%s:%s", tc.HostLogin, tc.Host, files[i])
		err := tc.SCP(context.TODO(), []string{remote, path}, tc.HostPort, false, true)
		if err != nil {
			return trace.Wrap(err)
		}
		logrus.Println("securley copied:", remote, " -->", filepath.Join(path, files[i]))
	}
	return nil
}
