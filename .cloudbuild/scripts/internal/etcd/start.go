/*
Copyright 2021 Gravitational, Inc.

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

package etcd

import (
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gravitational/trace"
)

// Start starts the etcd server using the Makefile `run-etcd` task and waits for it to start.
func Start(ctx context.Context, workspace string, uid, gid int, env ...string) error {
	cmd := exec.CommandContext(ctx, "make", "run-etcd")
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	// make etcd run under the supplied account
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	log.Printf("Launching etcd")
	go cmd.Run()

	log.Printf("Waiting for etcd to start...")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			d := net.Dialer{Timeout: 100 * time.Millisecond}
			_, err := d.Dial("tcp", "127.0.0.1:2379")
			if err == nil {
				log.Printf("Etcd is up")
				return nil
			}

		case <-timeoutCtx.Done():
			return trace.Errorf("Timed out waiting for etcd to start")
		}
	}
}
