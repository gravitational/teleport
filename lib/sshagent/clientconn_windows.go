//go:build windows

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshagent

import (
	"io"
	"os"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

const namedPipe = `\\.\pipe\openssh-ssh-agent`

// DialSystemAgent connects to the SSH agent listening on a Windows named pipe.
// If connecting to a named pipe fails and we're in a Cygwin environment, a
// connection to the Cygwin SSH agent advertised by SSH_AUTH_SOCK will be attempted.
//
// This is behind a build flag because winio.DialPipe is only available on Windows.
func DialSystemAgent() (io.ReadWriteCloser, error) {
	conn, err := winio.DialPipe(namedPipe, nil)
	if err == nil {
		return conn, nil
	}

	// MSYSTEM is used to specify what Cygwin environment is used;
	// if it exists, there's a very good chance we're in a Cygwin
	// environment
	msys := os.Getenv("MSYSTEM")
	socketPath := os.Getenv(teleport.SSHAuthSock)
	if msys != "" && socketPath != "" {
		conn, err := dialCygwin(socketPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	return nil, trace.Wrap(err)
}
