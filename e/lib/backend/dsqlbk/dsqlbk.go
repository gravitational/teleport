// Package dsqlbk implements a Teleport backend that uses AWS DSQL
// (https://docs.aws.amazon.com/aurora-dsql/latest/userguide/what-is-aurora-dsql.html)
// as a Teleport backend, using its Kinesis CDC stream functionality
// (https://docs.aws.amazon.com/aurora-dsql/latest/userguide/cdc-streams.html)
// to receive events for watchers. It supports DSQL clusters in both
// single-region and multi-region mode.
//
// Backend items are stored in a single "kv" table, defined by
//
//	CREATE TABLE kv IF NOT EXISTS (
//	    key text PRIMARY KEY,
//	    value bytea NOT NULL CHECK (length(value) <= 512000),
//	    expiry timestamptz NOT NULL,
//	    revision uuid NOT NULL
//	)
//
// and containing one row per item. Items with no expiry are recorded with an
// expiry of 'infinity' (unlike pgbk and crdbbk which use NULL for that,
// requiring much more annoying handling).
//
// Backend keys are stored as text because DSQL doesn't support indexing on
// bytea, but any real use of the backend goes through a [backend.Sanitizer]
// which enforces that backend keys are ascii printable with a handful of
// additional restrictions anyway.
//
// Values are enforced to be 500KiB or less, which is slightly more than what
// DynamoDB can store in a single item, helps with staying under transaction
// size limits and makes it so that the CDC record will never be larger than the
// cutoff for record fragmentation - even at maximum length, a json-encoded
// record for a row in the kv table should stay within the 1MiB of the default
// Kinesis maximum message size, let alone the hard limit of 10MiB, so we don't
// have to handle defragmentation at all.
//
// All backend operations (save for an edge case in AtomicWrite) pretend that
// expired items don't exist; expired items are solely handled by the background
// expiry deleter which runs periodically, getting a list of all items that are
// expired and then attempting to delete them serially in small chunks if
// they're still expired.
//
// Row manipulations are all fairly obvious, except for items in AtomicWrite
// that are asserted to not exist and are supposed to be left unwritten: the
// repeatable read locking model does not have a concept of locks on nonexistent
// rows, so what we have to do in that case is write a fake item (rendered as an
// item with an empty value and '-infinity' expiry) and delete it in the same
// transaction. Only the deletion will appear in the CDC stream, not the
// tombstone item, but there's unfortunately no way to filter it out, so any
// AtomicWrite with a [backend.NotExists] condition and [backend.Nop] action
// will generate a delete event in the event stream. An expired item asserted
// for nonexistence will likewise be deleted as a result of this limitation.
//
// The PutBatch, DeleteBatch and DeleteRange operations (DeleteRange is
// implemented by reading keys in the range and then calling the same
// implementation as DeleteBatch) use some internal parallelism to upsert or
// delete chunks of keys in parallel. While PutBatch splits up chunks based on
// total size and amount of items, DeleteBatch can only use the amount of items
// to delete at once to prepare chunks, which forces us to use a smaller maximum
// chunk size: the implementation of DSQL as of 2026/06/26 raises out of memory
// errors or fatal errors that kill connections if too much data is deleted in a
// single transaction. The default chunk size of 200 seems to work reliably, but
// tunables are available to change the limit at the backend config level.
//
// The CDC stream is parsed from Kinesis with the assumption that it is in
// UNORDERED ordering and JSON format (the only options at the time of writing).
// Since events are emitted to Kinesis without guarantees of ordering, the
// backend keeps track of the last event emitted for each key and will skip
// older events for the same key. To avoid unbound growth, a time-based
// threshold is applied during periodic clean-up operations, and events received
// from before this advancing threshold will cause the event stream to fault and
// reset. The time interval is configurable and defaults to 30 minutes, but it
// can be reduced to reduce the memory consumption of the reordering filter at
// the expense of less tolerance for out of order events.
//
// In multi-region mode, CDC streams created from either region will include the
// full set of changes from both regions, so each region can have its own
// regional Kinesis stream. The change feed reader does not support enhanced
// fan-out consumers, so each Kinesis stream is effectively limited to two
// consumers - similar to DynamoDB, the read throughput limit is twice the write
// throughput limit, so in on-demand mode there's only ever a guarantee that two
// consumers can read the same stream without throttling.
package dsqlbk

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"net"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dsqlauth "github.com/aws/aws-sdk-go-v2/feature/dsql/auth"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apitypes "github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	libcloudawsconfig "github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

func init() {
	backend.MustRegister(Name, NewFromParams)
}

var prepareMetricsOnce sync.Once

var eventEmitLatencySecondsHistogram prometheus.Histogram
var eventReceiveLatencySecondsHistogram prometheus.Histogram
var expiredItemsDeletedTotal prometheus.Counter
var expiredItemsSkippedTotal prometheus.Counter
var expiredItemsFailedTotal prometheus.Counter

func prepareMetrics() {
	prepareMetricsOnce.Do(func() {
		eventEmitLatencySecondsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "teleport_backend_dsql_event_emit_latency_seconds",
			Help:    "The latency between database write and event sent to Kinesis by DSQL",
			Buckets: prometheus.ExponentialBuckets(0.016, 2, 16),
		})
		eventReceiveLatencySecondsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "teleport_backend_dsql_event_receive_latency_seconds",
			Help:    "The latency between event sent to Kinesis by DSQL and event received by us",
			Buckets: prometheus.ExponentialBuckets(0.016, 2, 16),
		})
		expiredItemsDeletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "teleport_backend_dsql_expired_items_deleted_total",
			Help: "Expired items successfully deleted by background expiry",
		})
		expiredItemsSkippedTotal = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "teleport_backend_dsql_expired_items_skipped_total",
			Help: "Items identified as candidates for background expiry but skipped by being already gone or no longer expired",
		})
		expiredItemsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "teleport_backend_dsql_expired_items_failed_total",
			Help: "Items identified as candidates for background expiry but not deleted due to an error",
		})

		metrics.RegisterPrometheusCollectors(
			eventEmitLatencySecondsHistogram,
			eventReceiveLatencySecondsHistogram,
			expiredItemsDeletedTotal,
			expiredItemsSkippedTotal,
			expiredItemsFailedTotal,
		)
	})
}

const (
	Name = "dsql"

	// componentName is the component name used for logging.
	componentName = "dsqlbk"
)

// maxValueSize is the limit of a value that we're willing to write; other
// backends have slightly tighter limits (dynamodb is limited to 400KB per
// document, which leaves slightly less than 400 for the actual item itself),
// the row limit in DSQL is 2MiB (including some not well documented overhead),
// the column size limit for bytea is 1MiB, and the transaction data limit is
// 10MiB; by limiting ourselves to 500KiB (not 512) we are able to write 20
// max-size items in a batch without errors and deleting up to 200 items at once
// seems to work well. Even considering base64 encoding for bytea, a max-size
// row upsert should fit well within the default 1MiB kinesis message size limit
// without needing to burst higher, which simplifies the inner workings of the
// DSQL CDC with regards to kinesis and oversized messages.
const maxValueSize = 500 * 1024

// Config is the configuration struct for [Backend]; outside of tests or custom
// code, it's usually generated by converting the [backend.Params] from the
// Teleport configuration file.
type Config struct {
	DSQLIdentifier string `json:"dsql_identifier"`
	DSQLEndpoint   string `json:"dsql_endpoint"`
	DSQLRegion     string `json:"dsql_region"`
	DSQLAWSProfile string `json:"dsql_aws_profile"`

	KinesisStreamName   string `json:"kinesis_stream_name"`
	KinesisStreamRegion string `json:"kinesis_stream_region"`
	KinesisAWSProfile   string `json:"kinesis_aws_profile"`

	AWSConfigFilePath string `json:"aws_config_file_path"`

	DatabaseUser       string `json:"database_user"`
	DatabaseName       string `json:"database_name"`
	DatabaseSchema     string `json:"database_schema"`
	ConnStringOverride string `json:"database_conn_string_override"`

	ConnectTimeout apitypes.Duration `json:"connect_timeout"`
	QueryTimeout   apitypes.Duration `json:"query_timeout"`

	MaxConns              int32             `json:"max_conns"`
	MinConns              int32             `json:"min_conns"`
	MinIdleConns          int32             `json:"min_idle_conns"`
	MaxConnIdleTime       apitypes.Duration `json:"max_conn_idle_time"`
	MaxConnLifetime       apitypes.Duration `json:"max_conn_lifetime"`
	MaxConnLifetimeJitter apitypes.Duration `json:"max_conn_lifetime_jitter"`

	ItemsChunkMaxRows         int64 `json:"items_chunk_max_rows"`
	PutBatchChunkMaxRows      int   `json:"put_batch_chunk_max_rows"`
	PutBatchChunkMaxBytes     int64 `json:"put_batch_chunk_max_bytes"`
	PutBatchMaxParallelism    int   `json:"put_batch_max_parallelism"`
	DeleteBatchChunkMaxRows   int   `json:"delete_batch_chunk_max_items"`
	DeleteBatchMaxParallelism int   `json:"delete_batch_max_parallelism"`

	DisableExpiry       bool              `json:"disable_expiry"`
	ExpiryInterval      apitypes.Duration `json:"expiry_interval"`
	ExpiryChunkMaxRows  int               `json:"expiry_chunk_max_rows"`
	ExpiryGraceInterval apitypes.Duration `json:"expiry_grace_interval"`

	StreamPollInterval         apitypes.Duration `json:"stream_poll_interval"`
	StreamRetryInterval        apitypes.Duration `json:"stream_retry_interval"`
	StreamReorderGraceInterval apitypes.Duration `json:"stream_reorder_grace_interval"`

	DisableMetrics bool `json:"disable_metrics"`
}

func (c *Config) CheckAndSetDefaults() error {
	if c.DSQLIdentifier == "" {
		return trace.BadParameter("missing dsql_identifier")
	}
	if c.DSQLRegion == "" {
		return trace.BadParameter("missing dsql_region")
	}
	if c.DSQLEndpoint == "" {
		// https://github.com/awslabs/aurora-dsql-connectors/blob/761f856f9bb923e41680479fb7f5da5ec1099024/go/pgx/dsql/util.go#L40
		c.DSQLEndpoint = c.DSQLIdentifier + ".dsql." + c.DSQLRegion + ".on.aws"
	}

	if c.KinesisStreamName == "" {
		return trace.BadParameter("missing kinesis_stream_name")
	}
	if c.KinesisStreamRegion == "" {
		return trace.BadParameter("missing kinesis_stream_region")
	}

	if c.DatabaseUser == "" {
		return trace.BadParameter("missing database_user")
	}
	if c.DatabaseName == "" {
		// at the time of writing, DSQL only supports one database per cluster,
		// called "postgres", but the option is here because this might change
		// in the future and it's trivial to support an option for this
		c.DatabaseName = "postgres"
	}
	if c.DatabaseSchema == "" {
		return trace.BadParameter("missing database_schema")
	}

	if c.ConnectTimeout < 0 {
		return trace.BadParameter("connect_timeout must be non-negative")
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = apitypes.Duration(30 * time.Second)
	}

	if c.QueryTimeout < 0 {
		return trace.BadParameter("query_timeout must be non-negative")
	}
	if c.QueryTimeout == 0 {
		c.QueryTimeout = apitypes.Duration(30 * time.Second)
	}

	if c.MaxConns < 0 {
		return trace.BadParameter("max_conns must be non-negative")
	}
	if c.MinConns < 0 {
		return trace.BadParameter("min_conns must be non-negative")
	}
	if c.MinIdleConns < 0 {
		return trace.BadParameter("min_idle_conns must be non-negative")
	}

	// DSQL optimized settings, from
	// https://github.com/awslabs/aurora-dsql-connectors/blob/f0d7ead0e724164f24e1725fc6a74df7e07f2acc/go/pgx/dsql/config.go#L36-L40
	if c.MaxConnIdleTime < 0 {
		return trace.BadParameter("max_conn_idle_time must be non-negative")
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = apitypes.Duration(10 * time.Minute)
	}
	if c.MaxConnLifetime < 0 {
		return trace.BadParameter("max_conn_lifetime must be non-negative")
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = apitypes.Duration(50 * time.Minute)
	}
	if c.MaxConnLifetimeJitter < 0 {
		return trace.BadParameter("max_conn_lifetime_jitter must be non-negative")
	}
	if c.MaxConnLifetimeJitter == 0 {
		c.MaxConnLifetimeJitter = apitypes.Duration(5 * time.Minute)
	}

	if c.ItemsChunkMaxRows < 0 {
		return trace.BadParameter("items_chunk_max_rows must be non-negative")
	}
	if c.ItemsChunkMaxRows == 0 {
		c.ItemsChunkMaxRows = 1000
	}

	if c.PutBatchChunkMaxRows < 0 {
		return trace.BadParameter("put_batch_chunk_max_rows must be non-negative")
	}
	if c.PutBatchChunkMaxRows == 0 {
		c.PutBatchChunkMaxRows = 1000
	}

	if c.PutBatchChunkMaxBytes < 0 {
		return trace.BadParameter("put_batch_chunk_max_bytes must be non-negative")
	}
	if c.PutBatchChunkMaxBytes == 0 {
		c.PutBatchChunkMaxBytes = 5 * 1024 * 1024
	}

	if c.PutBatchMaxParallelism < 0 {
		return trace.BadParameter("put_batch_max_parallelism must be non-negative")
	}
	if c.PutBatchMaxParallelism == 0 {
		c.PutBatchMaxParallelism = 20
	}

	if c.DeleteBatchChunkMaxRows < 0 {
		return trace.BadParameter("delete_batch_chunk_max_items must be non-negative")
	}
	if c.DeleteBatchChunkMaxRows == 0 {
		c.DeleteBatchChunkMaxRows = 200
	}

	if c.DeleteBatchMaxParallelism < 0 {
		return trace.BadParameter("delete_batch_max_parallelism must be non-negative")
	}
	if c.DeleteBatchMaxParallelism == 0 {
		c.DeleteBatchMaxParallelism = 20
	}

	if c.ExpiryInterval < 0 {
		return trace.BadParameter("expiry_interval must be non-negative")
	}
	if c.ExpiryInterval == 0 {
		c.ExpiryInterval = apitypes.Duration(20 * time.Minute)
	}

	if c.ExpiryChunkMaxRows < 0 {
		return trace.BadParameter("expiry_chunk_max_rows must be non-negative")
	}
	if c.ExpiryChunkMaxRows == 0 {
		c.ExpiryChunkMaxRows = 200
	}

	if c.ExpiryGraceInterval < 0 {
		return trace.BadParameter("expiry_grace_interval must be non-negative")
	}
	if c.ExpiryGraceInterval == 0 {
		c.ExpiryGraceInterval = apitypes.Duration(5 * time.Second)
	}

	if c.StreamPollInterval < 0 {
		return trace.BadParameter("stream_poll_interval must be non-negative")
	}
	if c.StreamPollInterval == 0 {
		// Kinesis recommendation for the time between GetRecords calls
		// https://docs.aws.amazon.com/kinesis/latest/APIReference/API_GetRecords.html
		c.StreamPollInterval = apitypes.Duration(time.Second)
	}

	if c.StreamRetryInterval < 0 {
		return trace.BadParameter("stream_retry_interval must be non-negative")
	}
	if c.StreamRetryInterval == 0 {
		c.StreamRetryInterval = apitypes.Duration(10 * time.Second)
	}

	if c.StreamReorderGraceInterval < 0 {
		return trace.BadParameter("stream_reorder_grace_interval must be non-negative")
	}
	if c.StreamReorderGraceInterval == 0 {
		c.StreamReorderGraceInterval = apitypes.Duration(30 * time.Minute)
	}

	return nil
}

// NewFromParams starts and returns a [backend.Backend] (whose underlying type
// is [*Backend]) with the given params (generally read from the Teleport
// configuration file).
func NewFromParams(ctx context.Context, params backend.Params) (backend.Backend, error) {
	var cfg Config
	if err := apiutils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	bk, err := NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bk, nil
}

// NewWithConfig starts and returns a [*Backend] with the given [Config].
func NewWithConfig(ctx context.Context, cfg Config) (*Backend, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.DisableMetrics {
		prepareMetrics()
	}

	var sharedConfigFiles, sharedCredentialsFiles []string
	if cfg.AWSConfigFilePath != "" {
		sharedConfigFiles = []string{cfg.AWSConfigFilePath}
		sharedCredentialsFiles = []string{}
	}
	databaseAWSConfig, err := libcloudawsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.DSQLRegion),
		awsconfig.WithSharedConfigProfile(cfg.DSQLAWSProfile),
		awsconfig.WithSharedConfigFiles(sharedConfigFiles),
		awsconfig.WithSharedCredentialsFiles(sharedCredentialsFiles),
	)
	if err != nil {
		return nil, trace.Wrap(err, "loading DSQL config")
	}
	if _, err := databaseAWSConfig.Credentials.Retrieve(ctx); err != nil {
		return nil, trace.Wrap(err, "retrieving DSQL credentials")
	}

	var kinesisAWSConfig aws.Config
	if cfg.KinesisStreamRegion == cfg.DSQLRegion && cfg.KinesisAWSProfile == cfg.DSQLAWSProfile {
		kinesisAWSConfig = databaseAWSConfig
	} else {
		c, err := libcloudawsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(cfg.KinesisStreamRegion),
			awsconfig.WithSharedConfigProfile(cfg.KinesisAWSProfile),
			awsconfig.WithSharedConfigFiles(sharedConfigFiles),
			awsconfig.WithSharedCredentialsFiles(sharedCredentialsFiles),
		)
		if err != nil {
			return nil, trace.Wrap(err, "loading Kinesis config")
		}
		if _, err := c.Credentials.Retrieve(ctx); err != nil {
			return nil, trace.Wrap(err, "retrieving Kinesis credentials")
		}
		kinesisAWSConfig = c
	}

	connString := cfg.ConnStringOverride
	if connString == "" {
		connString = (&url.URL{
			Scheme: "postgresql",
			User:   url.User(cfg.DatabaseUser),
			Host:   cfg.DSQLEndpoint,
			Path:   cfg.DatabaseName,
			RawQuery: url.Values{
				"sslmode":        {"verify-full"},
				"sslnegotiation": {"direct"},
				"sslrootcert":    {"system"},
				"search_path":    {"''"},
			}.Encode(),
		}).String()
	}

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.DatabaseUser == "admin" {
		endpoint, region, creds := cfg.DSQLEndpoint, cfg.DSQLRegion, databaseAWSConfig.Credentials
		poolConfig.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
			token, err := dsqlauth.GenerateDBConnectAdminAuthToken(ctx, endpoint, region, creds)
			if err != nil {
				return trace.Wrap(err)
			}
			cc.Password = token
			return nil
		}
	} else {
		endpoint, region, creds := cfg.DSQLEndpoint, cfg.DSQLRegion, databaseAWSConfig.Credentials
		poolConfig.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
			token, err := dsqlauth.GenerateDbConnectAuthToken(ctx, endpoint, region, creds)
			if err != nil {
				return trace.Wrap(err)
			}
			cc.Password = token
			return nil
		}
	}

	poolConfig.ConnConfig.ConnectTimeout = time.Duration(cfg.ConnectTimeout)
	poolConfig.ConnConfig.DialFunc = (&net.Dialer{Timeout: time.Duration(cfg.ConnectTimeout)}).DialContext

	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	if cfg.MinIdleConns > 0 {
		poolConfig.MinIdleConns = cfg.MinIdleConns
	}
	poolConfig.MaxConnIdleTime = time.Duration(cfg.MaxConnIdleTime)
	poolConfig.MaxConnLifetime = time.Duration(cfg.MaxConnLifetime)
	poolConfig.MaxConnLifetimeJitter = time.Duration(cfg.MaxConnLifetimeJitter)

	log := slog.With(teleport.ComponentKey, componentName)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, trace.Wrap(err, "connecting to the database")
	}

	slog.InfoContext(ctx, "starting backend")

	ctx, cancel := context.WithCancel(ctx)
	b := &Backend{
		cfg: cfg,
		kv:  pgx.Identifier{cfg.DatabaseSchema, "kv"}.Sanitize(),

		log:    log,
		pool:   pool,
		buf:    backend.NewCircularBuffer(),
		cancel: cancel,
	}

	if _, err := b.execIdempotent(ctx,
		"CREATE TABLE IF NOT EXISTS "+b.kv+" (key text CONSTRAINT kv_pkey PRIMARY KEY, value bytea NOT NULL CONSTRAINT kv_value_check CHECK (length(value) <= 512000), expiry timestamptz NOT NULL, revision uuid NOT NULL)",
		pgx.QueryExecModeExec,
	); err != nil {
		slog.WarnContext(ctx, "failed to ensure the existence of the kv table, proceeding anyway", "error", err)
	}

	if !cfg.DisableExpiry {
		b.wg.Go(func() {
			b.runExpiry(ctx)
		})
	}

	b.wg.Go(func() {
		b.runChangeFeed(ctx, kinesisAWSConfig)
	})

	return b, nil
}

type Backend struct {
	cfg Config

	// kv is pgx.Identifier{cfg.DatabaseSchema, "kv"}.Sanitize() so we don't
	// have to repeat that over and over.
	kv string

	log  *slog.Logger
	pool *pgxpool.Pool
	buf  *backend.CircularBuffer

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

var _ backend.Backend = (*Backend)(nil)
var _ backend.BatchPutter = (*Backend)(nil)
var _ backend.BatchDeleter = (*Backend)(nil)

// GetName implements [backend.Backend].
func (*Backend) GetName() string {
	return Name
}

// exec is [pgx.Conn.Exec] with automatic retrying and a timeout. The query is
// assumed to not be idempotent when deciding which errors to retry.
func (b *Backend) exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	const isIdempotentFalse = false
	return retry(ctx, b.log, isIdempotentFalse, func() (pgconn.CommandTag, error) {
		conn, err := b.pool.Acquire(ctx)
		if err != nil {
			return pgconn.CommandTag{}, trace.Wrap(err)
		}
		defer conn.Release()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.QueryTimeout))
		defer cancel()

		t, err := conn.Exec(ctx, sql, arguments...)
		if err != nil {
			return pgconn.CommandTag{}, trace.Wrap(err)
		}
		return t, nil
	})
}

// execIdempotent is [pgx.Conn.Exec] with automatic retrying and a timeout. The
// query is assumed to be idempotent when deciding which errors to retry.
func (b *Backend) execIdempotent(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	const isIdempotentTrue = true
	return retry(ctx, b.log, isIdempotentTrue, func() (pgconn.CommandTag, error) {
		conn, err := b.pool.Acquire(ctx)
		if err != nil {
			return pgconn.CommandTag{}, trace.Wrap(err)
		}
		defer conn.Release()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.QueryTimeout))
		defer cancel()

		t, err := conn.Exec(ctx, sql, arguments...)
		if err != nil {
			return pgconn.CommandTag{}, trace.Wrap(err)
		}
		return t, nil
	})
}

// TODO(espadolini): these are free functions but should be turned into generic methods in go 1.27

// exec is [pgx.Conn.Query] followed by [pgx.CollectRows] with automatic
// retrying and a timeout. The query is assumed to be idempotent when deciding
// which errors to retry.
func collectRowsIdempotent[T any](b *Backend, ctx context.Context, sql string, arguments []any, rowToFunc pgx.RowToFunc[T]) ([]T, error) {
	const isIdempotentTrue = true
	return retry(ctx, b.log, isIdempotentTrue, func() ([]T, error) {
		conn, err := b.pool.Acquire(ctx)
		if err != nil {
			return nil, err
		}
		defer conn.Release()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.QueryTimeout))
		defer cancel()

		rows, _ := conn.Query(ctx, sql, arguments...)
		return pgx.CollectRows(rows, rowToFunc)
	})
}

// exec is [pgx.Conn.Query] followed by [pgx.CollectExactlyOneRow] with
// automatic retrying and a timeout. The query is assumed to be idempotent when
// deciding which errors to retry.
func collectExactlyOneRowIdempotent[T any](b *Backend, ctx context.Context, sql string, arguments []any, rowToFunc pgx.RowToFunc[T]) (T, error) {
	const isIdempotentTrue = true
	return retry(ctx, b.log, isIdempotentTrue, func() (T, error) {
		conn, err := b.pool.Acquire(ctx)
		if err != nil {
			return *new(T), err
		}
		defer conn.Release()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.QueryTimeout))
		defer cancel()

		rows, _ := conn.Query(ctx, sql, arguments...)
		return pgx.CollectExactlyOneRow(rows, rowToFunc)
	})
}

// Create implements [backend.Backend].
func (b *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Value) > maxValueSize {
		return nil, trace.BadParameter("backend item too large for writing")
	}

	newRevision := randomRevision()
	tag, err := b.exec(ctx,
		"INSERT INTO "+b.kv+" AS kv (key, value, expiry, revision) VALUES ($1, $2, $3, $4) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiry = EXCLUDED.expiry, revision = EXCLUDED.revision WHERE kv.expiry <= transaction_timestamp()",
		i.Key.String(),
		nonNil(i.Value),
		toExpiry(i.Expires),
		newRevision,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return nil, trace.AlreadyExists("item %+q already exists", i.Key.String())
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: revisionToString(newRevision),
	}, nil
}

// Put implements [backend.Backend].
func (b *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Value) > maxValueSize {
		return nil, trace.BadParameter("backend item too large for writing")
	}

	newRevision := randomRevision()
	if _, err := b.exec(ctx,
		"INSERT INTO "+b.kv+" AS kv (key, value, expiry, revision) VALUES ($1, $2, $3, $4) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiry = EXCLUDED.expiry, revision = EXCLUDED.revision",
		i.Key.String(),
		nonNil(i.Value),
		toExpiry(i.Expires),
		newRevision,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: revisionToString(newRevision),
	}, nil
}

// CompareAndSwap implements [backend.Backend].
func (b *Backend) CompareAndSwap(ctx context.Context, expected, replaceWith backend.Item) (*backend.Lease, error) {
	if len(replaceWith.Value) > maxValueSize {
		return nil, trace.BadParameter("backend item too large for writing")
	}

	if expected.Key.String() != replaceWith.Key.String() {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	newRevision := randomRevision()
	tag, err := b.exec(ctx,
		"UPDATE "+b.kv+" AS kv SET value = $1, expiry = $2, revision = $3 WHERE kv.key = $4 AND kv.value = $5 AND kv.expiry > transaction_timestamp()",
		nonNil(replaceWith.Value),
		toExpiry(replaceWith.Expires),
		newRevision,
		replaceWith.Key.String(),
		nonNil(expected.Value),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return nil, trace.CompareFailed("item %+q does not exist or does not match expected", replaceWith.Key.String())
	}

	return &backend.Lease{
		Key:      replaceWith.Key,
		Revision: revisionToString(newRevision),
	}, nil
}

// Update implements [backend.Backend].
func (b *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Value) > maxValueSize {
		return nil, trace.BadParameter("backend item too large for writing")
	}

	newRevision := randomRevision()
	tag, err := b.exec(ctx,
		"UPDATE "+b.kv+" AS kv SET value = $1, expiry = $2, revision = $3 WHERE kv.key = $4 AND kv.expiry > transaction_timestamp()",
		nonNil(i.Value),
		toExpiry(i.Expires),
		newRevision,
		i.Key.String(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return nil, trace.NotFound("item %+q does not exist", i.Key.String())
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: revisionToString(newRevision),
	}, nil
}

func (b *Backend) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if len(i.Value) > maxValueSize {
		return nil, trace.BadParameter("backend item too large for writing")
	}

	expectedRevision, ok := revisionFromString(i.Revision)
	if !ok {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	newRevision := randomRevision()
	tag, err := b.exec(ctx,
		"UPDATE "+b.kv+" AS kv SET value = $1, expiry = $2, revision = $3 WHERE kv.key = $4 AND kv.revision = $5 AND kv.expiry > transaction_timestamp()",
		nonNil(i.Value),
		toExpiry(i.Expires),
		newRevision,
		i.Key.String(),
		expectedRevision,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: revisionToString(newRevision),
	}, nil
}

// Get implements [backend.Backend].
func (b *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	item, err := collectExactlyOneRowIdempotent(b, ctx,
		"SELECT kv.value, kv.expiry, kv.revision FROM "+b.kv+" AS kv WHERE kv.key = $1 AND kv.expiry > transaction_timestamp()",
		[]any{key.String()},
		func(row pgx.CollectableRow) (*backend.Item, error) {
			var value []byte
			var expiry pgtype.Timestamptz
			var revision revision
			if err := row.Scan(&value, &expiry, &revision); err != nil {
				return nil, trace.Wrap(err)
			}
			return &backend.Item{
				Key:      key,
				Value:    value,
				Expires:  fromExpiry(expiry),
				Revision: revisionToString(revision),
			}, nil
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, trace.NotFound("item %+q does not exist", key.String())
		}
		return nil, trace.Wrap(err)
	}
	return item, nil
}

func (b *Backend) Items(ctx context.Context, params backend.ItemsParams) iter.Seq2[backend.Item, error] {
	if params.StartKey.IsZero() {
		err := trace.BadParameter("missing parameter startKey")
		return func(yield func(backend.Item, error) bool) { yield(backend.Item{}, err) }
	}
	if params.EndKey.IsZero() {
		err := trace.BadParameter("missing parameter endKey")
		return func(yield func(backend.Item, error) bool) { yield(backend.Item{}, err) }
	}

	limit := max(0, int64(params.Limit))

	var query string
	if !params.Descending {
		query = "SELECT kv.key, kv.value, kv.expiry, kv.revision FROM " + b.kv + " AS kv WHERE $1 <= kv.key AND kv.key <= $2 AND ($3::text IS NULL OR kv.key > $3) AND kv.expiry > transaction_timestamp() ORDER BY kv.key ASC LIMIT $4"
	} else {
		query = "SELECT kv.key, kv.value, kv.expiry, kv.revision FROM " + b.kv + " AS kv WHERE $1 <= kv.key AND kv.key <= $2 AND ($3::text IS NULL OR kv.key < $3) AND kv.expiry > transaction_timestamp() ORDER BY kv.key DESC LIMIT $4"
	}

	return func(yield func(backend.Item, error) bool) {
		var exclusiveStartKey pgtype.Text
		var totalCount int64
		for {
			pageLimit := b.cfg.ItemsChunkMaxRows
			if limit > 0 {
				pageLimit = min(limit-totalCount, pageLimit)
			}

			items, err := collectRowsIdempotent(b, ctx,
				query,
				[]any{params.StartKey.String(), params.EndKey.String(), exclusiveStartKey, pageLimit},
				func(row pgx.CollectableRow) (backend.Item, error) {
					var key string
					var value []byte
					var expiry pgtype.Timestamptz
					var revision revision
					if err := row.Scan(&key, &value, &expiry, &revision); err != nil {
						return backend.Item{}, trace.Wrap(err)
					}

					return backend.Item{
						Key:      backend.KeyFromString(key),
						Value:    value,
						Expires:  fromExpiry(expiry),
						Revision: revisionToString(revision),
					}, nil
				})
			if err != nil {
				yield(backend.Item{}, trace.Wrap(err))
				return
			}

			for _, item := range items {
				if !yield(item, nil) {
					return
				}

				totalCount++
				if limit > 0 && totalCount >= limit {
					return
				}
			}

			if int64(len(items)) < pageLimit {
				return
			}

			exclusiveStartKey = pgtype.Text{
				String: items[len(items)-1].Key.String(),
				Valid:  true,
			}
		}
	}
}

// GetRange implements [backend.Backend].
func (b *Backend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	var result backend.GetResult
	for item, err := range b.Items(ctx, backend.ItemsParams{StartKey: startKey, EndKey: endKey, Limit: limit}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		result.Items = append(result.Items, item)
	}

	return &result, nil
}

// Delete implements [backend.Backend].
func (b *Backend) Delete(ctx context.Context, key backend.Key) error {
	tag, err := b.exec(ctx,
		"DELETE FROM "+b.kv+" AS kv WHERE kv.key = $1 AND kv.expiry > transaction_timestamp()",
		key.String(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return trace.NotFound("item %+q does not exist", key.String())
	}
	return nil
}

// ConditionalDelete implements [backend.Backend].
func (b *Backend) ConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	expectedRevision, ok := revisionFromString(rev)
	if !ok {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	tag, err := b.exec(ctx,
		"DELETE FROM "+b.kv+" AS kv WHERE kv.key = $1 AND kv.revision = $2 AND kv.expiry > transaction_timestamp()",
		key.String(),
		expectedRevision,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}
	return nil
}

// DeleteRange implements [backend.Backend].
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	keys, err := collectRowsIdempotent(b, ctx,
		"SELECT kv.key FROM "+b.kv+" AS kv WHERE $1 <= kv.key AND kv.key <= $2 AND kv.expiry > transaction_timestamp()",
		[]any{startKey.String(), endKey.String()},
		pgx.RowTo[string],
	)
	if err != nil {
		return trace.Wrap(err)
	}
	shuffleSlice(keys)

	return b.deleteBatch(ctx, keys)
}

// KeepAlive implements [backend.Backend].
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expiry time.Time) error {
	newRevision := randomRevision()
	tag, err := b.exec(ctx,
		"UPDATE "+b.kv+" AS kv SET expiry = $1, revision = $2 WHERE kv.key = $3 AND kv.expiry > transaction_timestamp()",
		toExpiry(expiry), newRevision, lease.Key.String(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if tag.RowsAffected() <= 0 {
		return trace.NotFound("item %+q does not exist", lease.Key.String())
	}
	return nil
}

// PutBatch implements [backend.BatchPutter].
func (b *Backend) PutBatch(ctx context.Context, items []backend.Item) ([]string, error) {
	for _, item := range items {
		if len(item.Value) > maxValueSize {
			return nil, trace.BadParameter("backend item too large for writing")
		}
	}

	// our best bet for performance is to insert items in random order so we can
	// use all the physical partitions at once - in a traditional database we'd
	// probably want to insert in primary key order instead
	indices := make([]int, 0, len(items))
	for i := range items {
		indices = append(indices, i)
	}
	shuffleSlice(indices)

	revisions := make([]string, len(items))
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(b.cfg.PutBatchMaxParallelism)

	query := "INSERT INTO " + b.kv + " AS kv (key, value, expiry, revision) SELECT * FROM unnest($1::text[], $2::bytea[], $3::timestamptz[], $4::uuid[]) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiry = EXCLUDED.expiry, revision = EXCLUDED.revision"

	for indicesChunk := range sizedChunks(b.cfg.PutBatchChunkMaxBytes, b.cfg.PutBatchChunkMaxRows, indices, items) {
		if ctx.Err() != nil {
			if err := eg.Wait(); err != nil {
				return nil, trace.Wrap(err)
			}
			return nil, trace.Wrap(ctx.Err())
		}

		eg.Go(func() error {
			newRevision := randomRevision()

			keyChunk := make([]string, 0, len(indicesChunk))
			valueChunk := make([][]byte, 0, len(indicesChunk))
			expiryChunk := make([]pgtype.Timestamptz, 0, len(indicesChunk))
			revisionChunk := make([]revision, 0, len(indicesChunk))
			for _, i := range indicesChunk {
				item := &items[i]
				keyChunk = append(keyChunk, item.Key.String())
				valueChunk = append(valueChunk, nonNil(item.Value))
				expiryChunk = append(expiryChunk, toExpiry(item.Expires))
				revisionChunk = append(revisionChunk, newRevision)
			}

			if _, err := b.exec(ctx,
				query,
				keyChunk, valueChunk, expiryChunk, revisionChunk,
			); err != nil {
				return trace.Wrap(err)
			}

			r := revisionToString(newRevision)
			for _, i := range indicesChunk {
				revisions[i] = r
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return revisions, nil
}

// sizedChunks yields consecutive chunks of indices, stopping each chunk after
// reaching the specified maximum total item size (taking the indices to be
// indices in the slice of backend items) or after reaching the specified
// maximum count of items in the chunk. The chunks will always contain at least
// one item even if the maximum count is non-positive or if the size of the item
// exceeds the maximum total byte size.
func sizedChunks(maxBytes int64, maxCount int, indices []int, items []backend.Item) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		for len(indices) > 0 {
			size := int64(len(items[indices[0]].Value))
			count := 1
			for count < len(indices) && count < maxCount && size+int64(len(items[indices[count]].Value)) <= maxBytes {
				size += int64(len(items[indices[count]].Value))
				count += 1
			}
			if !yield(indices[:count]) {
				return
			}
			indices = indices[count:]
		}
	}
}

// DeleteBatch implements [backend.BatchDeleter].
func (b *Backend) DeleteBatch(ctx context.Context, keys []backend.Key) error {
	textKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		textKeys = append(textKeys, k.String())
	}
	shuffleSlice(textKeys)

	return b.deleteBatch(ctx, textKeys)
}

func (b *Backend) deleteBatch(ctx context.Context, keys []string) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(b.cfg.DeleteBatchMaxParallelism)

	query := "DELETE FROM " + b.kv + " AS kv WHERE kv.key = ANY ($1::text[]) AND kv.expiry > transaction_timestamp()"

	for chunk := range slices.Chunk(keys, b.cfg.DeleteBatchChunkMaxRows) {
		if ctx.Err() != nil {
			if err := eg.Wait(); err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(ctx.Err())
		}

		eg.Go(func() error {
			if _, err := b.exec(ctx,
				query,
				chunk,
			); err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return trace.Wrap(err)
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
	// we don't support a custom clock, because deciding which items still exist
	// in the backend depends on which items are still stored but expired, and
	// it's much cleaner to just rely on the server transaction time (which is
	// shared between all auth servers) for that
	return clockwork.NewRealClock()
}
