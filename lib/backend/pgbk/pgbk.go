package pgbk

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
)

const deleteBatchSize = 1000

func New(ctx context.Context, params backend.Params) (*Backend, error) {
	connString, _ := params["conn_string"].(string)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log := logrus.WithField(trace.Component, teleport.Component("pgbk"))

	if azure, _ := params["azure"].(bool); azure {
		clientID, _ := params["azure_client_id"].(string)
		log.WithField("client_id", clientID).Warn("Using Azure authentication.")
		bc, err := azureBeforeConnect(clientID, log)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		poolConfig.BeforeConnect = bc
	}

	poolConfig.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET default_transaction_isolation TO serializable", pgx.QueryExecModeExec)
		return trace.Wrap(err)
	}

	log.Info("Setting up backend.")

	tryEnsureDatabase(ctx, poolConfig, log)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clock, _ := params["clock"].(clockwork.Clock)
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	ctx, cancel := context.WithCancel(ctx)
	b := &Backend{
		log:    log,
		pool:   pool,
		buf:    backend.NewCircularBuffer(),
		clock:  clock,
		cancel: cancel,
	}

	if err := b.setupAndMigrate(ctx); err != nil {
		b.Close()
		return nil, trace.Wrap(err)
	}

	b.wg.Add(1)
	go b.backgroundExpiry(ctx)

	b.wg.Add(1)
	go b.backgroundChangeFeed(ctx)

	return b, nil
}

type Backend struct {
	log   logrus.FieldLogger
	pool  *pgxpool.Pool
	buf   *backend.CircularBuffer
	clock clockwork.Clock

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (b *Backend) Close() error {
	b.cancel()
	b.wg.Wait()
	b.buf.Close()
	b.pool.Close()
	return nil
}

func (b *Backend) retry(ctx context.Context, f func(*pgxpool.Pool) error) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.NewHalfJitter(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for i := 1; i < 10; i++ {
		err = f(b.pool)
		if err == nil {
			return nil
		}

		if isCode(err, pgerrcode.SerializationFailure) || isCode(err, pgerrcode.DeadlockDetected) {
			b.log.WithError(err).WithField("attempt", i).Debug("Operation failed due to conflicts, retrying quickly.")
			retry.Reset()
		} else {
			b.log.WithError(err).WithField("attempt", i).Debug("Operation failed, retrying.")
			retry.Inc()
		}

		select {
		case <-retry.After():
		case <-ctx.Done():
			return ctx.Err()
		}

	}

	return trace.LimitExceeded("too many retries, last error: %v", err)
}

func (b *Backend) setupAndMigrate(ctx context.Context) error {
	const latestVersion = 2
	var version int
	var migrateErr error
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		return pgx.BeginFunc(ctx, b.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS migrate (
					version int PRIMARY KEY,
					created timestamptz NOT NULL DEFAULT now()
				)`, pgx.QueryExecModeExec,
			); err != nil && !isCode(err, pgerrcode.InsufficientPrivilege) {
				return trace.Wrap(err)
			}

			if err := tx.QueryRow(ctx,
				"SELECT COALESCE(max(version), 0) FROM migrate",
				pgx.QueryExecModeExec,
			).Scan(&version); err != nil {
				return trace.Wrap(err)
			}

			switch version {
			case 0:
				if _, err := tx.Exec(ctx, `
					CREATE TABLE kv (
						key bytea PRIMARY KEY,
						value bytea NOT NULL,
						expires timestamp,
						rev uuid NOT NULL
					);
					CREATE INDEX kv_expires ON kv (expires) WHERE expires IS NOT NULL;
					INSERT INTO migrate (version) VALUES (2);`,
					pgx.QueryExecModeExec,
				); err != nil {
					return trace.Wrap(err)
				}
			case latestVersion:
				// nothing to do
			default:
				migrateErr = trace.BadParameter("unsupported schema version %v", version)
			}

			return nil
		})
	}); err != nil {
		return trace.Wrap(err)
	}

	if migrateErr != nil {
		return migrateErr
	}

	if version != latestVersion {
		b.log.WithFields(logrus.Fields{
			"prev_version": version,
			"cur_version":  latestVersion,
		}).Info("Migrated database schema.")
	}

	return nil
}

var _ backend.Backend = (*Backend)(nil)

// Create writes item if key doesn't exist
func (b *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	var r int64
	rev := newRev()
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		tag, err := p.Exec(ctx,
			"INSERT INTO kv (key, value, expires, rev) VALUES ($1, $2, $3, $4) "+
				"ON CONFLICT (key) DO UPDATE SET "+
				"value = EXCLUDED.value, expires = EXCLUDED.expires, rev = EXCLUDED.rev "+
				"WHERE kv.expires IS NOT NULL AND kv.expires <= $5",
			i.Key, i.Value, zeronull.Timestamp(i.Expires.UTC()), rev, b.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}
		r = tag.RowsAffected()
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if r < 1 {
		return nil, trace.AlreadyExists("key %q already exists", i.Key)
	}
	return newLease(i), nil
}

// Put writes item
func (b *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	rev := newRev()
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		_, err := p.Exec(ctx,
			"INSERT INTO kv (key, value, expires, rev) VALUES ($1, $2, $3, $4) "+
				"ON CONFLICT (key) DO UPDATE SET "+
				"value = EXCLUDED.value, expires = EXCLUDED.expires, rev = EXCLUDED.rev",
			i.Key, i.Value, zeronull.Timestamp(i.Expires.UTC()), rev)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return newLease(i), nil
}

// CompareAndSwap
func (b *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}
	var r int64
	rev := newRev()
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		tag, err := p.Exec(ctx,
			"UPDATE kv SET value = $2, expires = $3, rev = $4 WHERE key = $1 AND value = $5 AND (expires IS NULL OR expires > $6)",
			replaceWith.Key, replaceWith.Value, zeronull.Timestamp(replaceWith.Expires.UTC()), rev, expected.Value, b.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}
		r = tag.RowsAffected()
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if r < 1 {
		return nil, trace.CompareFailed("key %q does not exist or does not match expected", replaceWith.Key)
	}
	return newLease(replaceWith), nil
}

// Update writes item if key exists
func (b *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	var r int64
	rev := newRev()
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		tag, err := p.Exec(ctx,
			"UPDATE kv SET value = $2, expires = $3, rev = $4 WHERE key = $1 AND (expires IS NULL OR expires > $5)",
			i.Key, i.Value, zeronull.Timestamp(i.Expires.UTC()), rev, b.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}
		r = tag.RowsAffected()
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if r < 1 {
		return nil, trace.NotFound("key %q does not exist", i.Key)
	}
	return newLease(i), nil
}

// Get implements backend.Backend
func (b *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	var item *backend.Item
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		var value []byte
		var expires zeronull.Timestamp
		var rev pgtype.UUID
		err := p.QueryRow(ctx,
			"SELECT value, expires, rev FROM kv "+
				"WHERE key = $1 AND (expires IS NULL OR expires > $2)",
			key, b.clock.Now().UTC()).Scan(&value, &expires, &rev)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return trace.Wrap(err)
		}

		item = &backend.Item{
			Key:     key,
			Value:   value,
			Expires: time.Time(expires),
		}
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if item == nil {
		return nil, trace.NotFound("key %q does not exist", key)
	}

	return item, nil
}

// GetRange implements backend.Backend
func (b *Backend) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}
	r := backend.GetResult{}
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		r.Items = nil
		rows, _ := p.Query(ctx,
			"SELECT key, value, expires, rev FROM kv "+
				"WHERE key BETWEEN $1 AND $2 AND (expires IS NULL OR expires > $3) "+
				"LIMIT $4",
			startKey, endKey, b.clock.Now().UTC(), limit,
		)
		var key, value []byte
		var expires zeronull.Timestamp
		var rev pgtype.UUID
		_, err := pgx.ForEachRow(rows, []any{&key, &value, &expires, &rev},
			func() error {
				r.Items = append(r.Items, backend.Item{
					Key:     key,
					Value:   value,
					Expires: time.Time(expires),
				})
				return nil
			},
		)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &r, nil
}

// Delete implements backend.Backend
func (b *Backend) Delete(ctx context.Context, key []byte) error {
	var r int64
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		tag, err := p.Exec(ctx,
			"DELETE FROM kv WHERE key = $1 AND (expires IS NULL OR expires > $2)",
			key, b.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}
		r = tag.RowsAffected()
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if r < 1 {
		return trace.NotFound("key %q does not exist", key)
	}
	return nil
}

// DeleteRange implements backend.Backend
func (b *Backend) DeleteRange(ctx context.Context, startKey []byte, endKey []byte) error {
	// "DELETE FROM kv WHERE key BETWEEN $1 AND $2" but more complicated:
	// logical decoding can become really really slow if a transaction is big
	// enough to spill on disk - max_changes_in_memory (4096) changes before
	// Postgres 13, or logical_decoding_work_mem (64MiB) bytes of total size in
	// Postgres 13 and later; thankfully, we can just limit our transactions to
	// a small-ish number of affected rows (1000 seems to work ok) as we don't
	// need atomicity here or in backgroundExpiry, which are the only two places
	// in which we do transactions that affect more than one row
	for i := 0; i < backend.DefaultRangeLimit/deleteBatchSize; i++ {
		var r int64
		if err := b.retry(ctx, func(p *pgxpool.Pool) error {
			tag, err := p.Exec(ctx,
				"DELETE FROM kv WHERE key = ANY(ARRAY(SELECT key FROM kv WHERE key BETWEEN $1 AND $2 LIMIT $3))",
				startKey, endKey, deleteBatchSize)
			if err != nil {
				return trace.Wrap(err)
			}
			r = tag.RowsAffected()
			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		if r < deleteBatchSize {
			return nil
		}
	}

	return trace.LimitExceeded("too many iterations")
}

// KeepAlive implements backend.Backend
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	var r int64
	rev := newRev()
	if err := b.retry(ctx, func(p *pgxpool.Pool) error {
		tag, err := p.Exec(ctx,
			"UPDATE kv SET expires = $2, rev = $3 WHERE key = $1 AND (expires IS NULL OR expires > $4)",
			lease.Key, zeronull.Timestamp(expires.UTC()), rev, b.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}
		r = tag.RowsAffected()
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if r < 1 {
		return trace.NotFound("key %q does not exist", lease.Key)
	}
	return nil
}

// NewWatcher implements backend.Backend
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// CloseWatchers implements backend.Backend
func (b *Backend) CloseWatchers() { b.buf.Clear() }

// Clock implements backend.Backend
func (b *Backend) Clock() clockwork.Clock { return b.clock }
