/*
Copyright 2022 Gravitational, Inc.

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

package regular

import (
	"context"
	"io"
	"os"
	"os/user"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/srv"
)

type homeDirSubsys struct {
	done chan struct{}
}

func newHomeDirSubsys() *homeDirSubsys {
	return &homeDirSubsys{
		done: make(chan struct{}),
	}
}

func (h *homeDirSubsys) Start(_ context.Context, serverConn *ssh.ServerConn, ch ssh.Channel, _ *ssh.Request, _ *srv.ServerContext) error {
	defer close(h.done)

	connUser := serverConn.User()
	localUser, err := user.Lookup(connUser)
	if err != nil {
		return trace.Wrap(err)
	}

	exists, err := srv.CheckHomeDir(localUser)
	if err != nil {
		return trace.Wrap(err)
	}
	homeDir := localUser.HomeDir
	if !exists {
		homeDir = string(os.PathSeparator)
	}
	_, err = io.Copy(ch, strings.NewReader(homeDir))

	return trace.Wrap(err)
}

func (h *homeDirSubsys) Wait() error {
	<-h.done
	return nil
}
