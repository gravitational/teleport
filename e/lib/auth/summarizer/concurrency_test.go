package summarizer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/types"
)

func TestDesktopConcurrencyLimit_CapsConcurrentDesktopSummaries(t *testing.T) {
	s := &SessionSummarizer{
		desktopConcurrencyLimiter: semaphore.NewWeighted(desktopConcurrencyLimit),
	}
	ctx := t.Context()

	started := make(chan struct{}, desktopConcurrencyLimit+2)
	proceed := make(chan struct{})

	var wg sync.WaitGroup
	for range desktopConcurrencyLimit + 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := s.acquireDesktopSlot(ctx, types.WindowsDesktopSessionKind)
			if err != nil {
				return
			}

			defer release()
			started <- struct{}{}
			<-proceed
		}()
	}

	// Exactly desktopConcurrencyLimit slots should be acquired up front.
	for i := range desktopConcurrencyLimit {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("expected %d desktop slots acquired, only got %d", desktopConcurrencyLimit, i)
		}
	}

	// No additional slot may be acquired while the first batch holds them.
	select {
	case <-started:
		t.Fatalf("acquired more than the desktop concurrency limit of %d", desktopConcurrencyLimit)
	case <-time.After(200 * time.Millisecond):
	}

	// Releasing the first batch lets the remaining acquire and finish.
	close(proceed)
	wg.Wait()
}

func TestDesktopConcurrencyLimit_NonDesktopNotLimited(t *testing.T) {
	s := &SessionSummarizer{
		desktopConcurrencyLimiter: semaphore.NewWeighted(desktopConcurrencyLimit),
	}
	ctx := t.Context()

	// Exhaust every desktop slot.
	for range desktopConcurrencyLimit {
		_, err := s.acquireDesktopSlot(ctx, types.WindowsDesktopSessionKind)
		require.NoError(t, err)
	}

	done := make(chan struct{})
	go func() {
		release, err := s.acquireDesktopSlot(ctx, types.SSHSessionKind)
		require.NoError(t, err)

		release()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("non-desktop session was blocked by the desktop concurrency limiter")
	}
}

// TestAcquireSlots_TimesOutWhenSlotsExhausted verifies that acquireSlots gives up after
// its own slotAcquireTimeout (rather than waiting on the inference budget) when no slot
// is available, and fails fast rather than hanging.
func TestAcquireSlots_TimesOutWhenSlotsExhausted(t *testing.T) {
	s := &SessionSummarizer{
		concurrencyLimiter:        semaphore.NewWeighted(concurrencyLimit),
		desktopConcurrencyLimiter: semaphore.NewWeighted(desktopConcurrencyLimit),
		slotAcquireTimeout:        50 * time.Millisecond,
	}
	ctx := t.Context()

	// Hold every desktop slot so the next desktop acquisition must wait.
	for range desktopConcurrencyLimit {
		release, err := s.acquireDesktopSlot(ctx, types.WindowsDesktopSessionKind)
		require.NoError(t, err)
		t.Cleanup(release)
	}

	start := time.Now()
	release, err := s.acquireSlots(ctx, types.WindowsDesktopSessionKind, "test-model")
	require.Error(t, err)
	require.Nil(t, release)
	require.Less(t, time.Since(start), 2*time.Second, "acquireSlots should fast-fail on slotAcquireTimeout, not hang")
}

// TestAcquireSlots_AcquiresAndReleasesBothLimiters verifies the happy path acquires the
// desktop and global slots and that the returned release frees the desktop slot.
func TestAcquireSlots_AcquiresAndReleasesBothLimiters(t *testing.T) {
	s := &SessionSummarizer{
		concurrencyLimiter:        semaphore.NewWeighted(concurrencyLimit),
		desktopConcurrencyLimiter: semaphore.NewWeighted(desktopConcurrencyLimit),
		slotAcquireTimeout:        5 * time.Second,
	}
	ctx := t.Context()

	release, err := s.acquireSlots(ctx, types.WindowsDesktopSessionKind, "test-model")
	require.NoError(t, err)
	require.NotNil(t, release)
	release()

	// After release, every desktop slot must be free again. Use a bounded context so a
	// missing release surfaces as a fast deadline error rather than an indefinite hang.
	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for range desktopConcurrencyLimit {
		r, err := s.acquireDesktopSlot(verifyCtx, types.WindowsDesktopSessionKind)
		require.NoError(t, err)
		t.Cleanup(r)
	}
}
