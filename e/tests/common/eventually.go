package common

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// CollectT is a helper struct used to collect errors from a condition check.
// For use with [AssertEventually] and [RequireEventually]. CollectorT implements
// a superset of [assert.TestingT] so it can be used with existing testify
// assertions, but also exposes a [context.Context] that can be used to detect
// test cancellation.
type CollectT struct {
	ctx    context.Context
	errors []error
}

// Helper implements [assert.TestingT]. Does nothing.
func (CollectT) Helper() {}

// Errorf collects an error.
func (c *CollectT) Errorf(format string, args ...any) {
	c.errors = append(c.errors, fmt.Errorf(format, args...))
}

// FailNow stops execution by calling runtime.Goexit.
func (c *CollectT) FailNow() {
	c.fail()
	runtime.Goexit()
}

func (c *CollectT) fail() {
	if !c.failed() {
		c.errors = []error{} // Make it non-nil to mark a failure.
	}
}

func (c *CollectT) failed() bool {
	return c.errors != nil
}

// Context returns the [context.Context] associated with the current test.
func (c *CollectT) Context() context.Context {
	return c.ctx
}

// AssertEventually is patterned after the testify [assert.EventuallyWithT],
// replacing the test timeout with context-based expiry in order to make
// polling-based waits behave similarly to event-based waits like [WaitForPutEvent]
// and [WaitForDeleteEvent].
//
// Note that using poll-based waiting is deprecated in favor of event-based waits
// like [WaitForPutEvent] and [WaitForDeleteEvent]; use an even-based wait wherever
// possible.
func AssertEventually(t *testing.T, condition func(*CollectT), pollInterval time.Duration, msgAndArgs ...any) bool {
	t.Helper()

	// define a context to manage the test timeout
	testContext, cancel := watcherCtx(t)
	defer cancel()

	ch := make(chan *CollectT, 1)
	checkCondition := func() {
		collector := &CollectT{ctx: testContext}
		defer func() { ch <- collector }()
		condition(collector)
	}

	// Start a ticker that will trigger the condition check at regular
	// intervals.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// The channel that we will read the ticker events from. We set this
	// to `nil` to avoid new ticks interrupting a long-running check.
	var tickCh <-chan time.Time

	// Track the errors from the last complete check to report them if the
	// condition is not satisfied.
	var lastCompleteCheckErrors []error

	// kick off the first condition check
	go checkCondition()

	for {
		select {
		case <-testContext.Done():
			for _, err := range lastCompleteCheckErrors {
				t.Errorf("%v", err)
			}
			return assert.Fail(t, "Condition never satisfied", msgAndArgs...)

		case <-tickCh:
			tickCh = nil // disable ticks for the duration of the test
			go checkCondition()

		case checkResult := <-ch:
			if !checkResult.failed() {
				return true
			}
			lastCompleteCheckErrors = checkResult.errors
			tickCh = ticker.C // re-enable ticks to schedule next poll
		}
	}
}

// RequireEventually is patterned after the testify `require.EventuallyWithT`,
// replacing the test timeout with context-based expiry in order to make
// polling-based waits behave similarly to event-based waits like [WaitForPutEvent]
// and [WaitForDeleteEvent].
//
// Note that using poll-based waiting is deprecated in favor of event-based waits
// like [WaitForPutEvent] and [WaitForDeleteEvent]; use an even-based wait wherever
// possible.
func RequireEventually(t *testing.T, condition func(*CollectT), pollInterval time.Duration, msgAndArgs ...any) {
	t.Helper()
	if AssertEventually(t, condition, pollInterval, msgAndArgs...) {
		return
	}
	t.FailNow()
}
