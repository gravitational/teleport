// Copyright 2023 Gravitational, Inc
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

package limiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestLimiter(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	memory, err := memory.New(memory.Config{
		Clock:   clock,
		Context: ctx,
	})
	require.NoError(t, err)
	store, err := local.NewSecReportsService(memory, clock)
	require.NoError(t, err)

	const (
		name               = "athena_limiter"
		totalLimit         = 10
		preAllocationValue = 1
	)

	l, err := NewLimiter(Config{
		Store:              store,
		Semaphore:          &mockSemaphore{},
		Name:               name,
		Log:                logrus.New(),
		Clock:              clock,
		TotalLimit:         totalLimit,
		PreAllocationValue: preAllocationValue,
		RefillAfter:        defaultRefillAfter,
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(totalLimit)
	for i := 0; i < totalLimit; i++ {
		go func() {
			defer wg.Done()
			update, err := l.AllocateLimit(ctx)
			require.NoError(t, err)
			require.NoError(t, update(1))
		}()
	}
	wg.Wait()

	item, err := store.GetCostLimiter(ctx, name)
	require.NoError(t, err)
	require.Equal(t, uint64(totalLimit), item.Spec.BytesScanned)

	_, err = l.AllocateLimit(ctx)
	require.True(t, trace.IsLimitExceeded(err))
	clock.Advance(defaultRefillAfter + time.Second)

	update, err := l.AllocateLimit(ctx)
	require.NoError(t, err)
	require.NoError(t, update(1))
}

type mockSemaphore struct {
	lease types.SemaphoreLease
	types.Semaphores
	mtx sync.Mutex
}

func (m *mockSemaphore) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	m.mtx.Lock()
	return &m.lease, nil
}

func (m *mockSemaphore) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	m.mtx.Unlock()
	return nil
}
