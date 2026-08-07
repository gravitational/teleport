// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package pgcommon

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type fakeSafeError struct {
	safe bool
}

func (e *fakeSafeError) Error() string     { return "fake connection error" }
func (e *fakeSafeError) SafeToRetry() bool { return e.safe }

// sequencer returns a mocked closure suitable for passing to retry(). It
// returns the given errors in order
func sequencer(t *testing.T, errs ...error) (f func() (int, error), callCount func() int) {
	t.Helper()
	calls := 0
	return func() (int, error) {
		require.Lessf(t, calls, len(errs), "f() called more times than expected")
		err := errs[calls]
		calls++
		return 0, err
	}, func() int { return calls }
}

func TestRetry_SucceedsFirstTry(t *testing.T) {
	t.Parallel()
	f, callCount := sequencer(t, nil)

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)

	require.NoError(t, err)
	require.Equal(t, 1, callCount())
}

func TestRetry_SerializationConflictRetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	f, callCount := sequencer(t,
		&pgconn.PgError{Code: pgerrcode.SerializationFailure},
		&pgconn.PgError{Code: pgerrcode.DeadlockDetected},
		nil,
	)

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)

	require.NoError(t, err)
	require.Equal(t, 3, callCount())
}

func TestRetry_NonRetryablePgErrorReturnsImmediately(t *testing.T) {
	t.Parallel()
	// NotNullViolation is neither a serialization conflict nor an idempotent
	// DDL conflict
	f, callCount := sequencer(t,
		&pgconn.PgError{Code: pgerrcode.NotNullViolation},
		nil,
	)

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)
	require.Error(t, err)
	require.Equal(t, 1, callCount())
}

func TestRetry_IdempotentDDLConflictRetriedOnlyWhenIdempotent(t *testing.T) {
	t.Parallel()
	t.Run("idempotent: retried and succeeds", func(t *testing.T) {
		f, callCount := sequencer(t,
			&pgconn.PgError{Code: pgerrcode.DuplicateTable},
			nil,
		)

		_, err := retry(t.Context(), logtest.NewLogger(), true, f)

		require.NoError(t, err)
		require.Equal(t, 2, callCount())
	})

	t.Run("non-idempotent: not retried", func(t *testing.T) {
		f, callCount := sequencer(t,
			&pgconn.PgError{Code: pgerrcode.DuplicateTable},
			nil,
		)

		_, err := retry(t.Context(), logtest.NewLogger(), false, f)

		require.Error(t, err)
		require.Equal(t, 1, callCount())
	})
}

func TestRetry_AmbiguousNonSafeErrorNotRetriedWhenNotIdempotent(t *testing.T) {
	t.Parallel()
	f, callCount := sequencer(t,
		&fakeSafeError{safe: false},
		nil,
	)

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)

	require.Error(t, err)
	require.Equal(t, 1, callCount())
}

func TestRetry_SafeToRetryNonPgErrorIsRetried(t *testing.T) {
	t.Parallel()
	f, callCount := sequencer(t,
		&fakeSafeError{safe: true},
		nil,
	)

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)

	require.NoError(t, err)
	require.Equal(t, 2, callCount())
}

func TestRetry_IdempotentNonPgErrorAlwaysRetried(t *testing.T) {
	t.Parallel()
	// A plain error is still retried when the caller told us f is idempotent.
	f, callCount := sequencer(t,
		errors.New("boom"),
		errors.New("BOOM"),
		nil,
	)

	_, err := retry(t.Context(), logtest.NewLogger(), true, f)

	require.NoError(t, err)
	require.Equal(t, 3, callCount())
}

func TestRetry_AlreadyCancelledContextReturnsImmediately(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	f, callCount := sequencer(t,
		&pgconn.PgError{Code: pgerrcode.SerializationFailure},
		nil,
	)

	_, err := retry(ctx, logtest.NewLogger(), false, f)

	require.ErrorIs(t, err, context.Canceled)
	// f is always called at least once
	require.Equal(t, 1, callCount())
}

func TestRetry_TooManyRetriesGivesUp(t *testing.T) {
	t.Parallel()
	calls := 0
	f := func() (int, error) {
		calls++
		return calls, &pgconn.PgError{Code: pgerrcode.SerializationFailure}
	}

	_, err := retry(t.Context(), logtest.NewLogger(), false, f)

	require.Error(t, err)
	require.True(t, trace.IsLimitExceeded(err), "expected a LimitExceeded error, got: %v", err)
	// Initial call + 9 retries.
	require.Equal(t, 10, calls)

}
