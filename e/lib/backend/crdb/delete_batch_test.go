package crdb

import (
	"context"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestDeleteBatch_Empty(t *testing.T) {
	b := &Backend{
		log:  logtest.NewLogger(),
		pool: fakePool{},
	}

	err := b.DeleteBatch(t.Context(), nil)
	require.NoError(t, err)

	err = b.DeleteBatch(t.Context(), []backend.Key{})
	require.NoError(t, err)
}

func TestDeleteBatch_Success(t *testing.T) {
	callCount := 0
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				callCount++
				return nil
			},
		},
	}

	keys := []backend.Key{
		backend.NewKey("key1"),
		backend.NewKey("key2"),
		backend.NewKey("key3"),
	}

	err := b.DeleteBatch(t.Context(), keys)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)
}

func TestDeleteBatch_ProtocolViolationError(t *testing.T) {
	callCount := 0
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				callCount++
				if callCount == 1 {
					return &pgconn.PgError{
						Code:    pgerrcode.ProtocolViolation,
						Message: "message size exceeds maximum",
					}
				}
				return nil
			},
		},
	}

	keys := []backend.Key{
		backend.NewKey("key1"),
		backend.NewKey("key2"),
		backend.NewKey("key3"),
		backend.NewKey("key4"),
	}

	err := b.DeleteBatch(t.Context(), keys)
	require.NoError(t, err)
	// First call fails with protocol violation, batch size is halved, then
	// all keys still fit in one chunk so the retry succeeds in a single call.
	require.Equal(t, 2, callCount)
}

func TestDeleteBatch_CommitDeadlineExceeded(t *testing.T) {
	callCount := 0
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				callCount++
				if callCount == 1 {
					return &pgconn.PgError{
						Code:    pgerrcode.SerializationFailure,
						Message: "commit deadline exceeded",
					}
				}
				return nil
			},
		},
	}

	keys := []backend.Key{
		backend.NewKey("key1"),
		backend.NewKey("key2"),
		backend.NewKey("key3"),
		backend.NewKey("key4"),
		backend.NewKey("key5"),
		backend.NewKey("key6"),
	}

	err := b.DeleteBatch(t.Context(), keys)
	require.NoError(t, err)
	// First call fails with commit deadline exceeded, batch size is halved,
	// then all keys still fit in one chunk so the retry succeeds in a single call.
	require.Equal(t, 2, callCount)
}

func TestDeleteBatch_UnrecoverableError(t *testing.T) {
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				return &pgconn.PgError{
					Code:    pgerrcode.InternalError,
					Message: "something went wrong",
				}
			},
		},
	}

	keys := []backend.Key{
		backend.NewKey("key1"),
		backend.NewKey("key2"),
	}

	err := b.DeleteBatch(t.Context(), keys)
	require.Error(t, err)
}

func TestDeleteBatch_SingleItemCannotReduceFurther(t *testing.T) {
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				return &pgconn.PgError{
					Code:    pgerrcode.ProtocolViolation,
					Message: "message size exceeds maximum",
				}
			},
		},
	}

	// With a single key, batch size cannot be reduced further so the error
	// should propagate.
	keys := []backend.Key{
		backend.NewKey("key1"),
	}

	err := b.DeleteBatch(t.Context(), keys)
	require.Error(t, err)
}
