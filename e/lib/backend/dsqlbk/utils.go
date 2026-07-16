package dsqlbk

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gravitational/teleport/api/utils/retryutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// revision is transparently converted to and from Postgres UUIDs.
type revision = [16]byte

// randomRevision returns a new random revision.
func randomRevision() revision {
	return revision(uuid.New())
}

// revisionToString converts a revision to its string form, usable in
// [backend.Item].
func revisionToString(r revision) string {
	return uuid.UUID(r).String()
}

// revisionFromString converts a revision from its string form, returning false
// in second position.
func revisionFromString(s string) (r revision, ok bool) {
	u, err := uuid.Parse(s)
	if err != nil {
		return revision{}, false
	}
	return u, true
}

// nonNil replaces a nil slice with an empty, non-nil one.
func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func toExpiry(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}
	}
	return pgtype.Timestamptz{Valid: true, Time: t.UTC()}
}

func fromExpiry(t pgtype.Timestamptz) time.Time {
	if !t.Valid || t.InfinityModifier != pgtype.Finite {
		return time.Time{}
	}
	return t.Time.UTC()
}

// retry runs a func potentially more than once, retrying quickly on
// serialization or deadlock errors and backing off more on other errors. If the
// func is flagged as idempotent then it will be retried even in case of
// ambiguous errors that might have caused some change to be persisted,
// otherwise the func will only be retried if we are sure that no change has
// happened (typically, if no data has been sent to the database). Uniqueness
// constraint and exclusion constraint violations will be retried, so the func
// should not rely on those. [pgx.ErrNoRows] and [pgx.ErrTooManyRows] errors
// will not be retried, even if the func is idempotent.
func retry[T any](ctx context.Context, log *slog.Logger, isIdempotent bool, f func() (T, error)) (T, error) {
	v, err := f()
	if err == nil {
		return v, nil
	}

	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, pgx.ErrTooManyRows) {
		return *new(T), trace.Wrap(err)
	}

	if ctx.Err() != nil {
		return *new(T), trace.Wrap(ctx.Err())
	}

	retry, retryErr := retryutils.NewLinear(retryutils.LinearConfig{
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.HalfJitter,
	})
	if retryErr != nil {
		return *new(T), trace.Wrap(retryErr)
	}

	// similar retry count to the one of dynamodbbk and more than the pgbk retry
	// count (DSQL can hit more conflicts than PostgreSQL when doing AtomicWrite
	// since there's only "exclusive locks" in DSQL)
	for i := 1; i < 30; i++ {
		var pgErr *pgconn.PgError
		_ = errors.As(err, &pgErr)

		if pgErr != nil && isSerializationErrorCode(pgErr.Code) {
			level := slog.LevelDebug
			if i < 3 {
				level = logutils.TraceLevel
			}
			log.LogAttrs(ctx, level,
				"Operation failed due to conflicts, retrying.",
				slog.Int("attempt", i),
				slog.Any("error", err),
			)
			retry.Inc()
		} else if (isIdempotent && pgErr == nil) || pgconn.SafeToRetry(err) {
			log.LogAttrs(ctx, slog.LevelDebug,
				"Operation failed, retrying.",
				slog.Int("attempt", i),
				slog.Any("error", err),
			)
			retry.Inc()
			retry.Inc()
		} else {
			// we either know we shouldn't retry (on a database error), or we
			// are not in idempotent mode and we don't know if we should retry
			// (ambiguous error after sending some data)
			return *new(T), trace.Wrap(err)
		}

		select {
		case <-retry.After():
		case <-ctx.Done():
			return *new(T), trace.Wrap(ctx.Err())
		}

		v, err = f()
		if err == nil {
			return v, nil
		}

		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, pgx.ErrTooManyRows) {
			return *new(T), trace.Wrap(err)
		}

		if ctx.Err() != nil {
			return *new(T), trace.Wrap(ctx.Err())
		}
	}

	return *new(T), trace.LimitExceeded("too many retries, last error: %v", err)
}

// isSerializationErrorCode returns true if the error code is for a
// serialization error; this also includes unique_violation and
// exclusion_violation, which are sometimes returned as a result of
// serialization failures (and thus can be meaningfully retried) but can also be
// a result of actual logical/relational errors, which would then cause the same
// error to be raised again.
func isSerializationErrorCode(code string) bool {
	// source:
	// https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
	// https://docs.aws.amazon.com/aurora-dsql/latest/userguide/working-with-concurrency-control.html
	switch code {
	case
		pgerrcode.UniqueViolation,
		pgerrcode.ExclusionViolation,
		pgerrcode.SerializationFailure,
		pgerrcode.DeadlockDetected:
		return true
	default:
		return false
	}
}

func shuffleSlice[S ~[]T, T any](s S) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

type sharedRawMessage []byte

func (m *sharedRawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("sharedRawMessage: UnmarshalJSON on nil pointer")
	}
	*m = data
	return nil
}
