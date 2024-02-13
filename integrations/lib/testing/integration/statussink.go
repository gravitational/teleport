package integration

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
)

type FakeStatusSink struct {
	status atomic.Pointer[types.PluginStatus]
}

func (s *FakeStatusSink) Emit(_ context.Context, status types.PluginStatus) error {
	s.status.Store(&status)
	return nil
}

func (s *FakeStatusSink) Get() types.PluginStatus {
	status := s.status.Load()
	if status == nil {
		panic("expected status to be set, but it has not been")
	}
	return *status
}
