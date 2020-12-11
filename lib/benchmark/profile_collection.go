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
	"path"
	"strings"

	client "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

// collectProfiles collects cpu, heap, and goroutine profiles
func collectProfiles(ctx context.Context, prefix, localDirectory, remoteUser string, tc *client.TeleportClient) error {
	localDirectory = path.Clean(localDirectory)
	profiles := []string{"cpu.profile http://127.0.0.1:3434/debug/pprof/profile", "goroutine.profile http://127.0.0.1:3434/debug/pprof/goroutine", "heap.profile http://127.0.0.1:3434/debug/pprof/heap"}
	files := []string{"cpu.profile", "goroutine.profile", "heap.profile"}
	for i := range profiles {
		profiles[i] = fmt.Sprintf("%s %s %s_%s", "curl", "-o", prefix, profiles[i])
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
	for i := range profiles {
		err = client.SSH(context.TODO(), strings.Split(profiles[i], " "), false)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = scpProfiles(files, remoteUser, localDirectory, tc)
	if err != nil {
		return err
	}
	return nil
}

// statusOK checks diagnostic endpoint status code on the remote server
func statusOK(tc *client.TeleportClient, out *bytes.Buffer) error {
	command := []string{"curl", "-s", "-w", "\"%{http_code}\n \"", "http://127.0.0.1:3434/debug/pprof/", "-o", "/dev/null"}
	err := tc.SSH(context.TODO(), command, false)
	if (err == nil) && !(strings.Contains(out.String(), "200")) {
		return nil
	}
	return trace.Wrap(err, ": make sure --diag-addr and -d flags are enabled on server start up")
}

// scpProfiles securely copies all profiles from remote server to local
func scpProfiles(files []string, user, local string, tc *client.TeleportClient) error {
	var remote string
	config := tc.Config
	client, err := client.NewClient(&config)
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range files {
		remote = fmt.Sprintf("%s@%s:%s", user, tc.Host, files[i])
		fmt.Println("securley copying... ", remote, " --> ", local)
		err := client.SCP(context.TODO(), []string{remote, local}, tc.HostPort, false, true)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
