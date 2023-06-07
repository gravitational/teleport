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

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestServerAuthenticateUserUserAgentTrim(t *testing.T) {
	ctx := context.Background()
	emitter := &eventstest.MockEmitter{}
	r := AuthenticateUserRequest{
		ClientMetadata: &ForwardedClientMetadata{
			UserAgent: strings.Repeat("A", maxUserAgentLen+1),
		},
	}
	// Ignoring the error here because we really just care that the event was logged.
	(&Server{emitter: emitter}).AuthenticateUser(ctx, r)
	event := emitter.LastEvent()
	loginEvent, ok := event.(*apievents.UserLogin)
	require.True(t, ok)
	require.True(t, len(loginEvent.UserAgent) <= maxUserAgentLen)
}
