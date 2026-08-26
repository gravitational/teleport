//go:build unix

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
	"net"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// DialSystemAgent connects to the SSH agent advertised by SSH_AUTH_SOCK.
func DialSystemAgent() (io.ReadWriteCloser, error) {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	if socketPath == "" {
		return nil, trace.NotFound("no system agent advertised by SSH_AUTH_SOCK")
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
