/*
Copyright 2015 Gravitational, Inc.

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
package srv

import (
	"fmt"

	"strings"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/lib/sshutils"
	"golang.org/x/crypto/ssh"
)

// tunSubsys is an SSH subsystem for easy tunneling through proxy server
// This subsystem creates a new SSH connection using auth with agent forwarding
// and launches a remote shell on the target server
type tunSubsys struct {
	host string
	port string
}

func parseTunSubsys(name string) (*tunSubsys, error) {
	out := strings.Split(name, ":")
	if len(out) != 3 {
		return nil, fmt.Errorf("invalid format for tun request: '%v', expected 'tun:host:port'", name)
	}
	return &tunSubsys{
		host: out[1],
		port: out[2],
	}, nil
}

func (t *tunSubsys) String() string {
	return fmt.Sprintf("tunSubsys(host=%v, port=%v)", t.host, t.port)
}

func (t *tunSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v execute()", t)
	a := ctx.getAgent()
	if a == nil {
		return fmt.Errorf("%v agent forwarding turned off, can not authorize", ctx)
	}
	signers, err := a.Signers()
	if err != nil {
		return fmt.Errorf("%v failed to get signers from agent: %v", err, ctx)
	}
	if len(signers) == 0 {
		return fmt.Errorf("%v no signers in the agent", ctx)
	}
	up, err := sshutils.DialUpstream(sconn.User(), fmt.Sprintf("%v:%v", t.host, t.port), signers)
	if err != nil {
		return fmt.Errorf("%v failed to connect to upstream, error: %v", ctx, err)
	}
	ctx.addCloser(up)
	return up.PipeShellToCh(ch)
}
