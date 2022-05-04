// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshutils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// NewClientConn creates a new SSH client connection that is passed tracing context so that spans may be correlated
// properly over the ssh connection.
func NewClientConn(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	hp := &HandshakePayload{
		TracingContext: tracing.PropagationContextFromContext(ctx),
	}

	if len(hp.TracingContext.Keys()) > 0 {
		payloadJSON, err := json.Marshal(hp)
		if err == nil {
			payload := fmt.Sprintf("%s%s\x00", ProxyHelloSignature, payloadJSON)
			_, err = conn.Write([]byte(payload))
			if err != nil {
				log.WithError(err).Warnf("Failed to pass along tracing context to proxy %v", addr)
			}
		}
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return c, chans, reqs, nil
}
