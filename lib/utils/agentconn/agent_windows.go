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

package agentconn

import (
	"net"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"

	"github.com/Microsoft/go-winio"
)

// Dial creates net.Conn to a SSH agent listening on a Windows named pipe.
// This is behind a build flag because winio.DialPipe is only available on
// Windows.
func Dial(socket string) (net.Conn, error) {
	conn, err := winio.DialPipe(defaults.WindowsOpenSSHNamedPipe, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
