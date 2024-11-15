// Package crdb implements the [backend.Backend] interface for cockroachdb.
// cockroachdb is a highly distributed and fault-taulerant database capable of
// surviving many failure scenarios.
package crdb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
)

func init() {
	backend.MustRegister(Name, func(ctx context.Context, p backend.Params) (backend.Backend, error) {
		return NewFromParams(ctx, p)
	})
	prometheus.MustRegister(metricCertExpiry, metricChangefeedCertExpiry)
}

const (
	// Name is the name of the backend used for configuration.
	Name      = "cockroachdb"
	component = "crdb"
)

var (

	// defaultPageSize is the page size used for GetRange queries by default.
	// This was chosen based on load testing at 150k ssh nodes. At scale range queries
	// over many rows fail due to ReadWithinUncertaintyIntervalError.
	defaultPageSize = 1000
	// defaultQueryTimeout is the context timeout set for all queries by default.
	defaultQueryTimeout = time.Second * 30

	metricCertExpiry = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "crdb_backend_certificate_expiry",
		Help:      "The expiration timestamp of the CockroachDB client certificates in seconds. A value of 0 indicates client certificates are not being used or an error occurred.",
	})
	metricChangefeedCertExpiry = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "crdb_backend_changefeed_certificate_expiry",
		Help:      "The expiration timestamp of the CockroachDB changefeed certificates in seconds. A value of 0 indicates client certificates are not being used or an error occurred.",
	})
)

var schemas = []string{
	`CREATE TABLE kv (
		key bytea NOT NULL,
		value bytea NOT NULL,
		expires timestamptz,
		revision uuid NOT NULL,
		CONSTRAINT kv_pkey PRIMARY KEY (key)
	) WITH (ttl_expiration_expression = 'expires', ttl_job_cron = '*/20 * * * *')`,
}

// New creates a new cockroachdb backend instance.
func NewFromParams(ctx context.Context, p backend.Params) (*Backend, error) {
	var cfg Config
	if err := utils.ObjectToStruct(p, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	bk, err := newFromConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bk, nil
}

func newFromConfig(ctx context.Context, cfg Config) (*Backend, error) {
	if cfg.ConnString == "" {
		return nil, trace.BadParameter("conn string must not be empty")
	}
	if cfg.ChangeFeedConnString == "" {
		cfg.ChangeFeedConnString = cfg.ConnString
	}
	if cfg.RangePageSize <= 0 {
		cfg.RangePageSize = defaultPageSize
	}

	log := slog.With(teleport.ComponentKey, component)
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tls := poolConfig.ConnConfig.TLSConfig; tls != nil {
		expiry, err := getCertExpiry(tls)
		if err != nil {
			log.WarnContext(ctx, "Failed to report client cert expiry", "error", err)
		}
		metricCertExpiry.Set(float64(expiry.Unix()))
	}

	feedConfig, err := pgxpool.ParseConfig(cfg.ChangeFeedConnString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tls := feedConfig.ConnConfig.TLSConfig; tls != nil {
		expiry, err := getCertExpiry(tls)
		if err != nil {
			log.WarnContext(ctx, "Failed to report changefeed cert expiry", "error", err)
		}
		metricChangefeedCertExpiry.Set(float64(expiry.Unix()))
	}

	log.InfoContext(ctx, "Setting up backend.")
	pgcommon.TryEnsureDatabase(ctx, poolConfig, log)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := pgcommon.SetupAndMigrate(ctx, log, pool, "backend_version", schemas); err != nil {
		pool.Close()
		return nil, trace.Wrap(err)
	}

	if cfg.TTLJobCron != "" {
		escapedCron := pgx.Identifier{cfg.TTLJobCron}.Sanitize()
		_, err := pool.Exec(ctx, fmt.Sprintf("ALTER TABLE kv SET (ttl_job_cron = %s)", escapedCron), pgx.QueryExecModeExec)
		if err != nil {
			pool.Close()
			return nil, trace.Wrap(err)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	bk := &Backend{
		cfg:        cfg,
		log:        log,
		feedConfig: feedConfig,
		buf:        backend.NewCircularBuffer(),
		pool:       pool,
		cancel:     cancel,
	}

	bk.wg.Add(1)
	go func() {
		defer bk.wg.Done()
		bk.backgroundChangeFeed(ctx)
	}()

	return bk, nil
}

// getCertExpiry returns the [tls.Certificate.NotAfter] for the certificate
// expiring the soonest.
func getCertExpiry(c *tls.Config) (time.Time, error) {
	var expiry time.Time
	for _, cert := range c.Certificates {
		if cert.Leaf == nil {
			parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return time.Time{}, trace.Wrap(err)
			}
			cert.Leaf = parsedCert
		}

		// Get the NotAfter timestamp (valid until) for the certificate
		if expiry.IsZero() || cert.Leaf.NotAfter.Before(expiry) {
			expiry = cert.Leaf.NotAfter
		}
	}
	return expiry, nil
}

// Config specifies parameters for configuring a cockroachdb backend.
type Config struct {
	// ConnString is the connection string used for connection pooling.
	ConnString string `json:"conn_string"`
	// ChangeFeedConnString is the connection string use for establishing change
	// feed connections.
	ChangeFeedConnString string `json:"change_feed_conn_string"`
	// TTLJobCron is a cron expression to specify the frequency at which
	// rows will be deleted based on their expiry.
	TTLJobCron string `json:"ttl_job_cron"`
	// RangePageSize is maximum number of rows queried by GetRange in a single request.
	// Queries with more rows will be broken up into multiple requests.
	RangePageSize int `json:"range_page_size"`
}

// Backend implements [backend.Backend] for cockroachdb.
type Backend struct {
	cfg    Config
	buf    *backend.CircularBuffer
	wg     sync.WaitGroup
	cancel context.CancelFunc
	log    *slog.Logger

	feedConfig *pgxpool.Config
	pool       *pgxpool.Pool
}

// Close implements [backend.Backend].
func (b *Backend) Close() error {
	b.cancel()
	b.wg.Wait()
	b.buf.Close()
	b.pool.Close()
	return nil
}

// Put implements [backend.Backend].
func (b *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	revision := newRevision()
	i.Expires = i.Expires.UTC()
	if _, err := pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			_, err := c.Exec(ctx,
				// Upsert is cockroachdb-specific.
				"UPSERT INTO kv (key, value, expires, revision) VALUES ($1, $2, $3, $4)",
				nonNilKey(i.Key), nonNil(i.Value), zeronull.Timestamptz(i.Expires), revision)
			return trace.Wrap(err)
		})
		return struct{}{}, trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	i.Revision = revisionToString(revision)
	return backend.NewLease(i), nil
}

var _ backend.Backend = (*Backend)(nil)

// GetName implements [backend.Backend].
func (*Backend) GetName() string {
	return Name
}

// Create implements [backend.Backend].
func (b *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	revision := newRevision()
	i.Expires = i.Expires.UTC()
	created, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var created bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()

			tag, err := c.Exec(ctx,
				"INSERT INTO kv (key, value, expires, revision) VALUES ($1, $2, $3, $4)"+
					" ON CONFLICT (key) DO UPDATE SET"+
					" value = excluded.value, expires = excluded.expires, revision = excluded.revision"+
					" WHERE kv.expires IS NOT NULL AND kv.expires <= now()",
				nonNilKey(i.Key), nonNil(i.Value), zeronull.Timestamptz(i.Expires), revision)
			if err != nil {
				return trace.Wrap(err)
			}
			created = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return created, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !created {
		return nil, trace.AlreadyExists("key %q already exists", i.Key)
	}

	i.Revision = revisionToString(revision)
	return backend.NewLease(i), nil
}

// CompareAndSwap implements [backend.Backend].
func (b *Backend) CompareAndSwap(ctx context.Context, expected, replaceWith backend.Item) (*backend.Lease, error) {
	if expected.Key.Compare(replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	revision := newRevision()
	replaceWith.Expires = replaceWith.Expires.UTC()
	swapped, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var swapped bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"UPDATE kv SET value = $1, expires = $2, revision = $3"+
					" WHERE kv.key = $4 AND kv.value = $5 AND (kv.expires IS NULL OR kv.expires > now())",
				nonNil(replaceWith.Value), zeronull.Timestamptz(replaceWith.Expires), revision,
				nonNilKey(replaceWith.Key), nonNil(expected.Value))
			if err != nil {
				return trace.Wrap(err)
			}
			swapped = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return swapped, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !swapped {
		return nil, trace.CompareFailed("key %q does not exist or does not match expected", replaceWith.Key)
	}

	replaceWith.Revision = revisionToString(revision)
	return backend.NewLease(replaceWith), nil
}

// Update implements [backend.Backend].
func (b *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	revision := newRevision()
	i.Expires = i.Expires.UTC()
	updated, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var updated bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"UPDATE kv SET value = $1, expires = $2, revision = $3"+
					" WHERE kv.key = $4 AND (kv.expires IS NULL OR kv.expires > now())",
				nonNil(i.Value), zeronull.Timestamptz(i.Expires), revision, nonNilKey(i.Key))
			if err != nil {
				return trace.Wrap(err)
			}
			updated = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return updated, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !updated {
		return nil, trace.NotFound("key %q does not exist", i.Key)
	}

	i.Revision = revisionToString(revision)
	return backend.NewLease(i), nil
}

func (b *Backend) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	expectedRevision, ok := revisionFromString(i.Revision)
	if !ok {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	newRevision := newRevision()
	i.Expires = i.Expires.UTC()
	updated, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var updated bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"UPDATE kv SET value = $1, expires = $2, revision = $3 "+
					"WHERE kv.key = $4 AND kv.revision = $5 AND "+
					"(kv.expires IS NULL OR kv.expires > now())",
				nonNil(i.Value), zeronull.Timestamptz(i.Expires), newRevision,
				nonNilKey(i.Key), expectedRevision)
			if err != nil {
				return trace.Wrap(err)
			}
			updated = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return updated, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !updated {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	i.Revision = revisionToString(newRevision)
	return backend.NewLease(i), nil
}

// Get implements [backend.Backend].
func (b *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	item, err := pgcommon.RetryIdempotent(ctx, b.log, func() (*backend.Item, error) {
		var item *backend.Item
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			batch := new(pgx.Batch)
			// batches run in an implicit transaction
			batch.Queue("SET transaction_read_only TO on")

			batch.Queue("SELECT kv.value, kv.expires, kv.revision FROM kv"+
				" WHERE kv.key = $1 AND (kv.expires IS NULL OR kv.expires > now())", nonNilKey(key),
			).QueryRow(func(row pgx.Row) error {
				var value []byte
				var expires time.Time
				var revision revision
				if err := row.Scan(&value, (*zeronull.Timestamptz)(&expires), &revision); err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return nil
					}
					return trace.Wrap(err)
				}

				item = &backend.Item{
					Key:      key,
					Value:    value,
					Expires:  expires.UTC(),
					Revision: revisionToString(revision),
				}
				return nil
			})
			if err := c.SendBatch(ctx, batch).Close(); err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return item, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if item == nil {
		return nil, trace.NotFound("key %q does not exist", key)
	}
	return item, nil
}

// GetRange implements [backend.Backend].
func (b *Backend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}
	var exclusiveStartKey []byte
	results := &backend.GetResult{}

	for {
		pageLimit := min(limit-len(results.Items), defaultPageSize)
		items, err := pgcommon.RetryIdempotent(ctx, b.log, func() ([]backend.Item, error) {
			var items []backend.Item
			err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				// Timers can be expensive at scale. To mitigate this allocate the
				// timeout context after a connection has been acquired. This limits
				// the number of timers to at most the size of the connection pool.
				ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
				defer cancel()

				batch := new(pgx.Batch)
				// batches run in an implicit transaction
				batch.Queue("SET transaction_read_only TO on")
				// TODO(espadolini): figure out if we want transaction_deferred enabled
				// for GetRange

				batch.Queue(
					"SELECT kv.key, kv.value, kv.expires, kv.revision FROM kv"+
						" WHERE kv.key BETWEEN $1 AND $2 AND ($3::bytea is NULL or kv.key > $3) AND (kv.expires IS NULL OR kv.expires > now())"+
						" ORDER BY kv.key LIMIT $4",
					nonNilKey(startKey), nonNilKey(endKey), exclusiveStartKey, pageLimit,
				).Query(func(rows pgx.Rows) error {
					var err error
					items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (backend.Item, error) {
						var key backend.Key
						var value []byte
						var expires time.Time
						var revision revision
						if err := row.Scan(&key, &value, (*zeronull.Timestamptz)(&expires), &revision); err != nil {
							return backend.Item{}, err
						}
						return backend.Item{
							Key:      key,
							Value:    value,
							Expires:  expires.UTC(),
							Revision: revisionToString(revision),
						}, nil
					})
					return trace.Wrap(err)
				})

				if err := c.SendBatch(ctx, batch).Close(); err != nil {
					return trace.Wrap(err)
				}
				return nil
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return items, nil
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results.Items = append(results.Items, items...)
		if len(items) < pageLimit || len(results.Items) >= limit {
			break
		}
		exclusiveStartKey = nonNilKey(items[len(items)-1].Key)
	}

	return results, nil
}

// Delete implements [backend.Backend].
func (b *Backend) Delete(ctx context.Context, key backend.Key) error {
	deleted, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var deleted bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"DELETE FROM kv WHERE kv.key = $1 AND (kv.expires IS NULL OR kv.expires > now())", nonNilKey(key))
			if err != nil {
				return trace.Wrap(err)
			}
			deleted = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return deleted, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !deleted {
		return trace.NotFound("key %q does not exist", key)
	}
	return nil
}

func (b *Backend) ConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	expectedRevision, ok := revisionFromString(rev)
	if !ok {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	deleted, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var deleted bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"DELETE FROM kv WHERE kv.key = $1 AND kv.revision = $2 AND "+
					"(kv.expires IS NULL OR kv.expires > now())",
				nonNilKey(key), expectedRevision)
			if err != nil {
				return trace.Wrap(err)
			}
			deleted = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return deleted, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !deleted {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}
	return nil
}

// DeleteRange implements [backend.Backend].
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	// this is the only backend operation that might affect a disproportionate
	// amount of rows at the same time; in actual operation, DeleteRange hardly
	// ever deletes more than dozens of items at once, so we're good here.
	if _, err := pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			_, err := c.Exec(ctx,
				"DELETE FROM kv WHERE kv.key BETWEEN $1 AND $2",
				nonNilKey(startKey), nonNilKey(endKey),
			)
			return trace.Wrap(err)
		})
		if err != nil {
			return struct{}{}, trace.Wrap(err)
		}
		return struct{}{}, nil
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// KeepAlive implements [backend.Backend].
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	revision := newRevision()
	updated, err := pgcommon.Retry(ctx, b.log, func() (bool, error) {
		var updated bool
		err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
			// Timers can be expensive at scale. To mitigate this allocate the
			// timeout context after a connection has been acquired. This limits
			// the number of timers to at most the size of the connection pool.
			ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
			defer cancel()
			tag, err := c.Exec(ctx,
				"UPDATE kv SET expires = $1, revision = $2"+
					" WHERE kv.key = $3 AND (kv.expires IS NULL OR kv.expires > now())",
				zeronull.Timestamptz(expires.UTC()), revision, nonNilKey(lease.Key))
			if err != nil {
				return trace.Wrap(err)
			}
			updated = tag.RowsAffected() > 0
			return nil
		})
		if err != nil {
			return false, trace.Wrap(err)
		}
		return updated, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !updated {
		return trace.NotFound("key %q does not exist", lease.Key)
	}
	return nil
}

// NewWatcher implements [backend.Backend].
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// CloseWatchers implements [backend.Backend].
func (b *Backend) CloseWatchers() { b.buf.Clear() }

// Clock implements [backend.Backend].
func (b *Backend) Clock() clockwork.Clock {
	return clockwork.NewRealClock()
}
