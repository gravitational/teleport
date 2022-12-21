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

package auth

import (
	"context"
	"strings"
	"testing"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/stretchr/testify/require"
)

type emitter struct {
	count int
}

func (e *emitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	loginEvent, ok := event.(*apievents.UserLogin)
	if ok && len(loginEvent.UserAgent) <= maxUserAgentLen {
		e.count++
	}
	return nil
}

func TestServerAuthenticateUserUserAgentTrim(t *testing.T) {
	emitter := &emitter{}

	r := AuthenticateUserRequest{
		ClientMetadata: &ForwardedClientMetadata{
			UserAgent: strings.Repeat("A", maxUserAgentLen+1),
		},
	}
	// Ignoring the error here because we really just care that the event was logged.
	(&Server{emitter: emitter}).AuthenticateUser(r)

	require.Equal(t, 1, emitter.count)
}
