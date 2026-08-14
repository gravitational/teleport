package common_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestSingleProcessLocker(t *testing.T) {
	t.Parallel()

	t.Run("basic lock and unlock", func(t *testing.T) {
		t.Parallel()
		locker := common.NewSingleProcessLocker()
		ctx := context.Background()

		unlock, err := locker.Acquire(ctx, "test-key")
		require.NoError(t, err)
		err = unlock(ctx)
		require.NoError(t, err)
	})

	t.Run("test context cancellation", func(t *testing.T) {
		t.Parallel()
		locker := common.NewSingleProcessLocker()

		unlock1, err := locker.Acquire(t.Context(), "test-key")
		require.NoError(t, err)
		defer func() { require.NoError(t, unlock1(t.Context())) }()

		ctx2, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = locker.Acquire(ctx2, "test-key")
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("re-acquiring after unlock succeeds", func(t *testing.T) {
		t.Parallel()
		locker := common.NewSingleProcessLocker()
		ctx := t.Context()

		unlock1, err := locker.Acquire(ctx, "test-key")
		require.NoError(t, err)
		require.NoError(t, unlock1(ctx))

		unlock2, err := locker.Acquire(ctx, "test-key")
		require.NoError(t, err)
		require.NoError(t, unlock2(ctx))
	})

	t.Run("concurrent goroutines on same key", func(t *testing.T) {
		t.Parallel()
		locker := common.NewSingleProcessLocker()
		const goroutineCount = 200

		var concurrentCount int
		var maxConcurrent int
		var wg sync.WaitGroup

		for i := 0; i < goroutineCount; i++ {
			wg.Go(func() {

				unlock, err := locker.Acquire(t.Context(), "key")
				require.NoError(t, err)

				// It is safe to increment concurrentCount without
				// atomic operations since the lock ensures only one
				// goroutine is in this critical section at a time.
				concurrentCount++
				maxConcurrent = max(concurrentCount, maxConcurrent)
				concurrentCount--

				require.NoError(t, unlock(t.Context()))
			})
		}
		wg.Wait()
		require.Equal(t, 1, maxConcurrent)
	})
}

func BenchmarkLocker(b *testing.B) {
	ctx := context.Background()

	bk, err := memory.New(memory.Config{
		Clock: clockwork.NewRealClock(),
	})
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, bk.Close()) })

	ps := local.NewPresenceService(bk)

	benchmarks := []struct {
		name   string
		locker common.Locker
	}{
		{
			name: "DistributedLocker",
			locker: func() common.Locker {
				l, _ := common.NewDistributedLocker(ps)
				return l
			}(),
		},
		{
			name: "LockerChain",
			locker: common.NewLockerChain(
				common.NewSingleProcessLocker(),
				func() common.Locker {
					l, _ := common.NewDistributedLocker(ps)
					return l
				}(),
			),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.SetParallelism(10)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					unlock, err := bm.locker.Acquire(ctx, "some-key")
					require.NoError(b, err)
					require.NoError(b, unlock(ctx))
				}
			})
		})
	}
}
