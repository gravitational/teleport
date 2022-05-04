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

package tracing

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
)

const (
	// SSHRequest is sent by clients to server to pass along tracing context.
	SSHRequest = "x-teleport-tracing"
)

// NewSSHSession creates a new SSH session that is passed tracing context so that spans may be correlated
// properly over the ssh connection.
func NewSSHSession(ctx context.Context, client *ssh.Client) (*ssh.Session, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	span := oteltrace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return session, nil
	}

	traceCtx := PropagationContextFromContext(ctx)
	if len(traceCtx.Keys()) == 0 {
		return session, nil
	}

	payload, err := json.Marshal(traceCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := session.SendRequest(SSHRequest, false, payload); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}
