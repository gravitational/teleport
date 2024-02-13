package integration

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
)

// FakeStatusSink is a fake status sink that can be used when testing plugins.
type FakeStatusSink struct {
	status atomic.Pointer[types.PluginStatus]
}

// Emit implements the common.StatusSink interface.
func (s *FakeStatusSink) Emit(_ context.Context, status types.PluginStatus) error {
	s.status.Store(&status)
	return nil
}

// Get returns the last status stored by the plugin.
func (s *FakeStatusSink) Get() types.PluginStatus {
	status := s.status.Load()
	if status == nil {
		panic("expected status to be set, but it has not been")
	}
	return *status
}
