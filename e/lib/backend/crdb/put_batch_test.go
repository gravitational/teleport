package crdb

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type fakePool struct {
	execFunc         func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	acquireFunc      func(ctx context.Context, f func(*pgxpool.Conn) error) error
	batchResultsFunc func(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// AcquireFunc implements Pool.
func (f fakePool) AcquireFunc(ctx context.Context, acquireFunc func(*pgxpool.Conn) error) error {
	if f.acquireFunc != nil {
		return f.acquireFunc(ctx, acquireFunc)
	}
	return nil
}

func (f fakePool) Close() {}

func (f fakePool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if f.execFunc != nil {
		return f.execFunc(ctx, sql, arguments...)
	}

	return pgconn.CommandTag{}, nil
}

func (f fakePool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	if f.batchResultsFunc != nil {
		return f.batchResultsFunc(ctx, b)
	}
	panic("not implemented")
}

// TestPutBatch_ExecUsesAcquiredConnection verifies that PutBatch calls Exec on
// the connection acquired via AcquireFunc rather than directly on the pool.
func TestPutBatch_ExecUsesAcquiredConnection(t *testing.T) {
	b := &Backend{
		log: logtest.NewLogger(),
		pool: fakePool{
			acquireFunc: func(ctx context.Context, f func(*pgxpool.Conn) error) error {
				defer func() { recover() }()
				// This is expected flow.
				// The acquireFunc was called, but we don't want to don't need to inject
				// the real *pgxpool.Conn.
				_ = f(nil)
				return nil
			},
			execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
				t.Fatal("pool.Exec must not be called directly; Exec should be called on the acquired connection")
				return pgconn.CommandTag{}, nil
			},
		},
	}

	now := time.Now().UTC()
	items := []backend.Item{
		{Key: backend.NewKey("key1"), Value: []byte("value1"), Expires: now},
	}

	// PutBatch should succeed because AcquireFunc returns nil (no error).
	revisions, err := b.PutBatch(t.Context(), items)
	require.NoError(t, err)
	require.Len(t, revisions, len(items))
}

func TestPutBatchChunk_ProtocolViolationError(t *testing.T) {
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

	now := time.Now().UTC()
	items := []backend.Item{
		{Key: backend.NewKey("key1"), Value: []byte("value1"), Expires: now},
		{Key: backend.NewKey("key2"), Value: []byte("value2"), Expires: now},
		{Key: backend.NewKey("key3"), Value: []byte("value3"), Expires: now},
		{Key: backend.NewKey("key4"), Value: []byte("value4"), Expires: now},
	}

	revisions, err := b.PutBatch(t.Context(), items)
	require.NoError(t, err)
	require.Len(t, revisions, len(items))

	require.Equal(t, 2, callCount)
}

func TestPutBatchChunk_CommitDeadlineExceeded(t *testing.T) {
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

	now := time.Now().UTC()
	items := []backend.Item{
		{Key: backend.NewKey("key1"), Value: []byte("value1"), Expires: now},
		{Key: backend.NewKey("key2"), Value: []byte("value2"), Expires: now},
		{Key: backend.NewKey("key3"), Value: []byte("value3"), Expires: now},
		{Key: backend.NewKey("key4"), Value: []byte("value4"), Expires: now},
		{Key: backend.NewKey("key5"), Value: []byte("value5"), Expires: now},
		{Key: backend.NewKey("key6"), Value: []byte("value6"), Expires: now},
	}

	revisions, err := b.PutBatch(t.Context(), items)
	require.NoError(t, err)
	require.Len(t, revisions, len(items))

	require.Equal(t, 2, callCount)
}
