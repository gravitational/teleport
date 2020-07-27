// +build windows

/*
Copyright 2018 Gravitational, Inc.

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

package client

import (
	"context"
	"io"

	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

// NodeSession is a bare minimum implementation to get Windows to compile.
// This sits behind a build flag because github.com/docker/docker/pkg/term
// on Windows does not support "SetWinsize". Because tsh on Windows does not
// support "tsh ssh" this code will never be called.
type NodeSession struct {
	ExitMsg string
}

func newSession(
	client *NodeClient,
	joinSession *session.Session,
	env map[string]string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	legacyID bool,
	enableEscapeSequences bool,
) (*NodeSession, error) {

	return nil, trace.BadParameter("sessions not supported on Windows")
}

func (ns *NodeSession) runCommand(ctx context.Context, cmd []string, callback ShellCreatedCallback, interactive bool) error {
	return trace.BadParameter("sessions not supported on Windows")
}

func (ns *NodeSession) runShell(callback ShellCreatedCallback) error {
	return trace.BadParameter("sessions not supported on Windows")
}

func (ns *NodeSession) NodeClient() *NodeClient {
	return nil
}

func (ns *NodeSession) Close() error {
	return trace.BadParameter("sessions not supported on Windows")
}
