/*
Copyright 2021 Gravitational, Inc.

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

package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

var _ types.Events = (*errorWatcher)(nil)

type errorWatcher struct {
}

func (e errorWatcher) GetProxies() ([]types.Server, error) {
	return nil, nil
}

func (e errorWatcher) NewWatcher(context.Context, types.Watch) (types.Watcher, error) {
	return nil, errors.New("watcher error")
}

func TestProxyWatcher_Backoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	w, err := services.NewProxyWatcher(services.ProxyWatcherConfig{
		Context:        ctx,
		Component:      "test",
		Clock:          clock,
		MaxRetryPeriod: defaults.MaxWatcherBackoff,
		Client:         &errorWatcher{},
		ProxiesC:       make(chan []types.Server, 1),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, w.Close()) })

	step := w.MaxRetryPeriod / 5.0
	for i := 0; i < 5; i++ {
		// wait for watcher to reload
		select {
		case duration := <-w.ResetC:
			stepMin := step * time.Duration(i) / 2
			stepMax := step * time.Duration(i+1)

			require.GreaterOrEqual(t, duration, stepMin)
			require.LessOrEqual(t, duration, stepMax)
			// add some extra to the duration to ensure the retry occurs
			clock.Advance(duration * 3)
		case <-time.After(time.Minute):
			t.Fatalf("timeout waiting for reset")
		}
	}
}
