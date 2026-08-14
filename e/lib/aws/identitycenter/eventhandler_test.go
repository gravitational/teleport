package identitycenter

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/monitor"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
)

func TestBatchReadEventsDetectsClose(t *testing.T) {
	const (
		batchDuration = 30 * time.Second
		batchTimeout  = 2 * batchDuration
	)

	clock := clockwork.NewFakeClock()
	fixture := ictest.NewFixture(t, ictest.WithClock(clock))

	type producer struct {
		svc    *Service
		clock  *clockwork.FakeClock
		cancel context.CancelFunc
	}

	testCases := []struct {
		name string

		// closer is a function that performs some action that should cause the
		// message reader to stop, wither by closing the channel, canceling the
		// context or by some other as-yet-undefined mechanism
		closer func(*producer)
	}{
		{
			name: "close empty channel",
			closer: func(p *producer) {
				close(p.svc.principalEventCh)
			},
		},
		{
			name: "cancel context with empty channel",
			closer: func(p *producer) {
				p.cancel()
			},
		},
		{
			name: "timeout context",
			closer: func(p *producer) {
				p.clock.Advance(batchTimeout + time.Second)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			icSvc := newTestService(t, fixture)

			batchCtx, batchCancel := clockwork.WithTimeout(fixture.Ctx, fixture.Clock, batchTimeout)
			t.Cleanup(batchCancel)

			// GIVEN an asynchronous producer process that writes messages into
			// a channel and then does something to indicate that there will be
			// no more messages...
			p := producer{
				svc:    icSvc,
				clock:  clock,
				cancel: batchCancel,
			}

			// GIVEN an asynchronous consumer process that logs when it receives
			// a quit signal
			var closeDetected atomic.Bool
			go func() {
				for fixture.Ctx.Err() == nil {
					_, _, ok := icSvc.batchReadEvents(batchCtx, batchDuration)
					// A non-OK value indicates that the channel was closed or the
					// context expired
					if !ok {
						closeDetected.Store(true)
						return
					}
				}
			}()

			// WHEN I do something that should cause the batch read to exit
			// early...
			test.closer(&p)

			// EXPECT that the explicit close signal was detected.
			require.EventuallyWithT(t,
				func(c *assert.CollectT) {
					assert.True(c, closeDetected.Load())
				},
				5*time.Second,
				100*time.Millisecond)
		})
	}
}

func TestBatchReadEventsDropsDuplicatePrincipalEvents(t *testing.T) {
	const (
		batchDuration = 30 * time.Second
		batchRepeats  = 5
	)

	users := make([]*types.UserV2, 4)
	for i := range users {
		u, err := types.NewUser(fmt.Sprintf("User #%00d", i))
		require.NoError(t, err)
		users[i] = u.(*types.UserV2)
	}

	clock := clockwork.NewFakeClock()
	fixture := ictest.NewFixture(t, ictest.WithClock(clock))
	icSvc := newTestService(t, fixture)

	// Force the principal event channel to be an unbuffered channel. This way we
	// can be sure that our consumer routine has received all of the messages our
	// producer process has sent before we close the batch by advancing the clock
	// past the batch collection end time. This way we don't have to deal with
	// lingering messages in the queue at the batch boundary when asserting that
	// content has been processed.
	icSvc.principalEventCh = make(chan *monitor.PrincipalEvent)

	// GIVEN an asynchronous producer process that will create principal events
	// for a number of principals, writing the events the service's principal
	// event queue for `readBatch()` to pick up
	var writeComplete atomic.Bool
	go func() {
		for i := range batchRepeats {
			rev := strconv.Itoa(i + 1)
			for _, src := range users {
				dst := apiutils.CloneProtoMsg(src)
				dst.Metadata.Revision = rev

				icSvc.queueResourceEvent(fixture.Ctx, &monitor.PrincipalEvent{
					Verb:      monitor.VerbCalculate,
					Principal: dst,
				})
			}
		}
		writeComplete.Store(true)
	}()

	// GIVEN an asynchronous consumer process that logs the event batch it reads
	var readComplete atomic.Bool
	var eventBatch principalEventMap
	closeDetected := false
	fullRecalcRequested := false

	go func() {
		defer readComplete.Store(true)

		events, doFullRecalc, ok := icSvc.batchReadEvents(fixture.Ctx, batchDuration)
		if !ok {
			closeDetected = true
			return
		}
		eventBatch = events
		fullRecalcRequested = doFullRecalc
	}()

	// EXPECT That the producer process will eventually finish sending its events
	// to the service for processing
	require.Eventually(t, writeComplete.Load, 10*time.Second, 100*time.Millisecond)

	// WHEN I advance the system clock past the batch completion deadline...
	clock.Advance(batchDuration + time.Millisecond)

	// EXPECT That the consumer process will complete reading its batch of events
	// and return
	require.Eventually(t, readComplete.Load, 10*time.Second, 100*time.Millisecond)

	// EXPECT that that nothing special happened while reading the event batch
	require.False(t, fullRecalcRequested, "Batch read must not return full recalc signal")
	require.False(t, closeDetected, "Batch read must not indicate end-of-stream")

	// EXPECT that the returned event batch contains the latest event message
	// for each user
	require.Len(t, eventBatch, len(users))
	expectedRev := strconv.Itoa(batchRepeats)
	for _, src := range users {
		id, err := principal.GetIDForPrincipalResource(src)
		require.NoError(t, err)

		require.Contains(t, eventBatch, id)
		principal := eventBatch[id].Principal

		// EXPECT that the right principal is in the right map slot
		require.Equal(t, src.GetName(), principal.GetName())

		// EXPECT that the final event for this principal is the event recorded
		// and returned by the batch read, as determined by the principal's
		// revision text matching the expected revision
		require.Equal(t, expectedRev, principal.GetRevision(),
			"Expected revision for %s is %q, got %q", principal.GetName(), expectedRev, principal.GetRevision())
	}
}

func TestBatchReadEventsRecalcAllOverridesEverything(t *testing.T) {
	const (
		batchDuration = 30 * time.Second
	)

	u, err := types.NewUser("Test User")
	require.NoError(t, err)
	user := u.(*types.UserV2)

	testCases := []struct {
		name         string
		eventCount   int
		offset       int
		expectRecalc require.BoolAssertionFunc
		expectEvents require.ValueAssertionFunc
	}{
		{
			name:         "first",
			eventCount:   100,
			offset:       0,
			expectRecalc: require.True,
			expectEvents: require.Empty,
		},
		{
			name:         "middle",
			eventCount:   100,
			offset:       50,
			expectRecalc: require.True,
			expectEvents: require.Empty,
		},
		{
			name:         "last",
			eventCount:   100,
			offset:       99,
			expectRecalc: require.True,
			expectEvents: require.Empty,
		},
		{
			name:         "none",
			eventCount:   100,
			offset:       -1,
			expectRecalc: require.False,
			expectEvents: func(t require.TestingT, val any, msgAndArgs ...any) {
				require.IsType(t, principalEventMap{}, val)
				m := val.(principalEventMap)

				id, err := principal.GetIDForPrincipalResource(user)
				require.NoError(t, err)
				require.Contains(t, m, id)

				p := m[id]
				require.Equal(t, "99", p.Principal.GetRevision())
			},
		},
	}

	clock := clockwork.NewFakeClock()
	fixture := ictest.NewFixture(t, ictest.WithClock(clock))

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			icSvc := newTestService(t, fixture)

			// Force the principal event channel to be an unbuffered channel. This way we
			// can be sure that our consumer routine has received all of the messages our
			// producer process has sent before we close the batch by advancing the clock
			// past the batch collection end time. This way we don't have to deal with
			// lingering messages in the queue at the batch boundary when asserting that
			// content has been processed.
			icSvc.principalEventCh = make(chan *monitor.PrincipalEvent)

			// GIVEN an asynchronous producer process that will create principal events
			// with a "recalc all" event at a particular offset, writing the events
			// the service's principal event queue for `readBatch()` to pick up
			var writeComplete atomic.Bool
			go func() {
				for i := range test.eventCount {
					if i == test.offset {
						icSvc.queueResourceEvent(fixture.Ctx, &monitor.PrincipalEvent{
							Verb: monitor.VerbCalculateAll,
						})
						continue
					}

					dst := apiutils.CloneProtoMsg(user)
					dst.Metadata.Revision = strconv.Itoa(i)
					icSvc.queueResourceEvent(fixture.Ctx, &monitor.PrincipalEvent{
						Verb:      monitor.VerbCalculate,
						Principal: dst,
					})
				}
				writeComplete.Store(true)
			}()

			// GIVEN an asynchronous consumer process that logs the event batch it reads
			var readComplete atomic.Bool
			var eventBatch principalEventMap
			closeDetected := false
			fullRecalcRequested := false

			go func() {
				defer readComplete.Store(true)

				events, doFullRecalc, ok := icSvc.batchReadEvents(fixture.Ctx, batchDuration)
				if !ok {
					closeDetected = true
					return
				}
				eventBatch = events
				fullRecalcRequested = doFullRecalc
			}()

			// EXPECT that the producer process will eventually finish sending its events
			// to the service for processing
			require.Eventually(t, writeComplete.Load, 10*time.Second, 100*time.Millisecond)

			// WHEN I advance the system clock past the batch completion deadline...
			clock.Advance(batchDuration + time.Millisecond)

			// EXPECT that the consumer process will complete reading its batch of events
			// and return
			require.Eventually(t, readComplete.Load, 10*time.Second, 100*time.Millisecond)

			// EXPECT that `batchRead()` has coalesced the events
			require.False(t, closeDetected, "Batch read must not indicate end-of-stream")
			test.expectRecalc(t, fullRecalcRequested)
			test.expectEvents(t, eventBatch)
		})
	}
}
