/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
