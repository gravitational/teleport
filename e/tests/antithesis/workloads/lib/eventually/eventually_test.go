package eventually_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
)

var (
	errSucceedAfter    = errors.New("succeedAfter error")
	errAlwaysFailFirst = errors.New("alwaysFail first error")
	errAlwaysFail      = errors.New("alwaysFail error")
	errSucceedThenFail = errors.New("succeedThenFail error")
	errSlowCondition   = errors.New("slowCondition error")
)

func succeedAfter(n int) eventually.ConditionFunc {
	calls := 0
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		calls++
		addDetail("calls", calls)
		addDetail("threshold", n)
		if calls > n {
			return true, nil
		}
		return false, errSucceedAfter
	}
}

func alwaysFail() eventually.ConditionFunc {
	calls := 0
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		calls++
		addDetail("calls", calls)
		if calls == 1 {
			return false, errAlwaysFailFirst
		}
		return false, errAlwaysFail
	}
}

func neverSucceed() eventually.ConditionFunc {
	calls := 0
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		calls++
		addDetail("calls", calls)
		return false, nil
	}
}

func succeedThenFail() eventually.ConditionFunc {
	calls := 0
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		calls++
		addDetail("calls", calls)
		return calls == 1, errSucceedThenFail
	}
}

func slowCondition(delay time.Duration, succeed bool) eventually.ConditionFunc {
	calls := 0
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		calls++
		addDetail("calls", calls)
		addDetail("delay_ms", delay.Milliseconds())

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(delay):
			if succeed {
				return true, nil
			}
			return false, errSlowCondition
		}
	}
}

func countingCondition(counter *int, succeed bool) eventually.ConditionFunc {
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		*counter++
		addDetail("count", *counter)
		return succeed, nil
	}
}

func TestAssert_SucceedsAfterDelay(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      10 * time.Second,
		Message:      "condition met after 3 polls",
		Condition:    succeedAfter(3),
		PollInterval: 100 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.True(t, condition)
			require.NotNil(t, details)
			assert.NotContains(t, details, "error")
		},
	})

	require.NoError(t, err)
}

func TestAssert_TimeoutExceeded(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      300 * time.Millisecond,
		Message:      "condition never met",
		Condition:    alwaysFail(),
		PollInterval: 50 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.False(t, condition)
			require.NotNil(t, details)
			assert.Equal(t, errAlwaysFail.Error(), details["error"])
		},
	})

	require.ErrorIs(t, err, errAlwaysFail)
}

func TestAssert_ConditionNotMetWithoutConditionError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      100 * time.Millisecond,
		Message:      "condition never met without condition error",
		Condition:    neverSucceed(),
		PollInterval: 10 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.False(t, condition)
			require.NotNil(t, details)
			assert.NotContains(t, details, "error")
			require.NotZero(t, details["calls"])
		},
	})

	require.NoError(t, err)
}

func TestAssert_ContextCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(150*time.Millisecond, cancel)

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      10 * time.Second,
		Message:      "context canceled before condition met",
		Condition:    alwaysFail(),
		PollInterval: 50 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.False(t, condition)
			require.NotNil(t, details)
			assert.Equal(t, errAlwaysFail.Error(), details["error"])
		},
	})

	require.ErrorIs(t, err, errAlwaysFail)
}

func TestAssert_ContextAlreadyCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := 0

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      10 * time.Second,
		Message:      "context already canceled",
		Condition:    countingCondition(&calls, false),
		PollInterval: 50 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.False(t, condition)
			require.NotNil(t, details)
			assert.NotContains(t, details, "error")
		},
	})

	require.NoError(t, err)
	assert.LessOrEqual(t, calls, 1)
}

func TestAssert_ConditionFlaps(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      2 * time.Second,
		Message:      "condition met on first poll then reverts",
		Condition:    succeedThenFail(),
		PollInterval: 50 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.True(t, condition)
			require.NotNil(t, details)
			assert.Equal(t, errSucceedThenFail.Error(), details["error"])
		},
	})

	require.ErrorIs(t, err, errSucceedThenFail)
}

func TestAssert_SlowConditionSucceeds(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      5 * time.Second,
		Message:      "slow condition eventually succeeds",
		Condition:    slowCondition(300*time.Millisecond, true),
		PollInterval: 100 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.True(t, condition)
			require.NotNil(t, details)
			assert.NotContains(t, details, "error")
		},
	})

	require.NoError(t, err)
}

func TestAssert_SlowConditionTimesOut(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      400 * time.Millisecond,
		Message:      "slow condition times out",
		Condition:    slowCondition(300*time.Millisecond, false),
		PollInterval: 100 * time.Millisecond,
		Assertion: func(condition bool, message string, details map[string]any) {
			require.False(t, condition)
			require.NotNil(t, details)
			assert.Equal(t, context.DeadlineExceeded.Error(), details["error"])
		},
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestAssert_WithDetails(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	assertFun := func(met bool, message string, details map[string]any) {
		require.True(t, met)
		assert.Equal(t, "auth1", details["node"])
		assert.Equal(t, "role/auditors", details["resource"])
		assert.Equal(t, "abc123", details["revision"])
		assert.NotContains(t, details, "error")

	}

	err := eventually.Assert(ctx, eventually.AssertParams{
		Timeout:      5 * time.Second,
		Message:      "condition met with extra details",
		Condition:    succeedAfter(1),
		PollInterval: 100 * time.Millisecond,
		Details: map[string]any{
			"node":     "auth1",
			"resource": "role/auditors",
			"revision": "abc123",
		},
		Assertion: assertFun,
	})
	require.NoError(t, err)
}
