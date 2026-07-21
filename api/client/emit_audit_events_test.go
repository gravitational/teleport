/*
Copyright 2025 Gravitational, Inc.

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
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types/events"
)

type batchEmitService struct {
	proto.UnimplementedAuthServiceServer
	mu      sync.Mutex
	batches [][]string
}

func (s *batchEmitService) EmitAuditEvents(_ context.Context, req *proto.EmitAuditEventsRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(req.Events))
	for _, oneOf := range req.Events {
		event, err := events.FromOneOf(*oneOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ids = append(ids, event.GetID())
	}
	s.batches = append(s.batches, ids)
	return &emptypb.Empty{}, nil
}

type unaryEmitService struct {
	proto.UnimplementedAuthServiceServer
	mu     sync.Mutex
	events []string
}

func (s *unaryEmitService) EmitAuditEvent(_ context.Context, oneOf *events.OneOf) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, err := events.FromOneOf(*oneOf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.events = append(s.events, event.GetID())
	return &emptypb.Empty{}, nil
}

func testAuditEvents(ids ...string) []events.AuditEvent {
	batch := make([]events.AuditEvent, len(ids))
	for i, id := range ids {
		event := &events.UserLogin{}
		event.SetID(id)
		batch[i] = event
	}
	return batch
}

func TestClientEmitAuditEvents(t *testing.T) {
	ctx := context.Background()

	t.Run("server supports batch RPC", func(t *testing.T) {
		svc := &batchEmitService{}
		srv := startMockServer(t, mockServices{auth: svc})
		clt, err := New(ctx, srv.clientCfg())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clt.Close()) })

		require.NoError(t, clt.EmitAuditEvents(ctx, testAuditEvents("a", "b", "c")))
		require.Equal(t, [][]string{{"a", "b", "c"}}, svc.batches, "events should arrive as a single batch")
	})

	t.Run("old server falls back to per-event", func(t *testing.T) {
		svc := &unaryEmitService{}
		srv := startMockServer(t, mockServices{auth: svc})
		clt, err := New(ctx, srv.clientCfg())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clt.Close()) })

		require.NoError(t, clt.EmitAuditEvents(ctx, testAuditEvents("a", "b", "c")))
		require.Equal(t, []string{"a", "b", "c"}, svc.events, "events should be delivered individually on fallback")
	})
}
