/*
Copyright 2023 Gravitational, Inc.

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

package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
)

func TestChannelEmitter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	clock := clockwork.NewFakeClock()

	// Use a blocking channel for testing.
	channel := make(chan events.AuditEvent, 1)
	t.Cleanup(func() {
		close(channel)
	})

	emitter := NewChannelEmitter(ChannelEmitterConfig{
		Channel: channel,
	})

	// Make sure the event has been emitted to the channel.
	expectedEvent := &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "b",
			Type: SessionJoinEvent,
			Time: clock.Now().Add(time.Minute).UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "alice",
		},
	}
	require.NoError(t, emitter.EmitAuditEvent(ctx, expectedEvent))

	emittedEvent := <-channel
	require.Empty(t, cmp.Diff(expectedEvent, emittedEvent))

	// Make sure the context cancel works.
	require.NoError(t, emitter.EmitAuditEvent(ctx, expectedEvent))
	var err error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err = emitter.EmitAuditEvent(ctx, expectedEvent)
		wg.Done()
	}()

	cancel()
	wg.Wait()
	require.ErrorIs(t, trace.BadParameter("context canceled"), err)
}
