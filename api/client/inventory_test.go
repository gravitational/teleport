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

package client

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

// TestInventoryControlStreamPipe is a sanity-check to make sure that the in-memory
// pipe version of the ICS works as expected.  This test is trivial but it helps to
// keep accidental breakage of the pipe abstraction from showing up in an obscure
// way inside the tests that rely upon it.
func TestInventoryControlStreamPipe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upstream, downstream := InventoryControlStreamPipe()
	defer upstream.Close()

	upMsgs := []proto.UpstreamInventoryMessage{
		new(proto.UpstreamInventoryHello),
		new(proto.UpstreamInventoryPong),
		new(proto.InventoryHeartbeat),
	}

	downMsgs := []proto.DownstreamInventoryMessage{
		new(proto.DownstreamInventoryHello),
		new(proto.DownstreamInventoryPing),
		new(proto.DownstreamInventoryPing), // duplicate to pad downMsgs to same length as upMsgs
	}

	go func() {
		for _, m := range upMsgs {
			downstream.Send(ctx, m)
		}
	}()

	go func() {
		for _, m := range downMsgs {
			upstream.Send(ctx, m)
		}
	}()

	timeout := time.NewTimer(time.Second * 5)
	defer timeout.Stop()
	for i := range upMsgs {
		if !timeout.Stop() {
			<-timeout.C
		}
		timeout.Reset(time.Second * 5)

		// upstream handle recv
		select {
		case msg := <-upstream.Recv():
			require.IsType(t, upMsgs[i], msg)
		case <-timeout.C:
			t.Fatalf("timeout waiting for message: %T", upMsgs[i])
		}

		// downstream handle recv
		select {
		case msg := <-downstream.Recv():
			require.IsType(t, downMsgs[i], msg)
		case <-timeout.C:
			t.Fatalf("timeout waiting for message: %T", downMsgs[i])
		}
	}

	upstream.Close()

	if !timeout.Stop() {
		<-timeout.C
	}
	timeout.Reset(time.Second * 5)

	select {
	case <-downstream.Done():
	case <-timeout.C:
		t.Fatal("timeout waiting for close")
	}

	assert.ErrorIs(t, downstream.Error(), io.EOF)
}
