// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package athena

import (
	"context"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// defaultBatchItems defines default value for batch items count.
	// 20000 items, per average 500KB event size = 10MB
	defaultBatchItems = 20000
	// defaultBatchInterval defines default batch interval.
	defaultBatchInterval = 1 * time.Minute
)

// Config structure represents Athena configuration.
// Right now the only way to set config is via url params.
type Config struct {
	// Region is where Athena, SQS and SNS lives (required).
	Region string

	// Publisher settings.

	// TopicARN where to emit events in SNS (required).
	TopicARN string
	// LargeEventsS3 is location on S3 where temporary large events (>256KB)
	// are stored before converting it to Parquet and moving to long term
	// storage (required).
	LargeEventsS3     string
	largeEventsBucket string
	largeEventsPrefix string

	// Query settings.

	// Database is name of Glue Database that Athena will query against (required).
	Database string
	// TableName is name of Glue Table that Athena will query against (required).
	TableName string
	// LocationS3 is location on S3 where Parquet files partitioned by date are
	// stored (required).
	LocationS3       string
	locationS3Bucket string
	locationS3Prefix string

	// QueryResultsS3 is location on S3 where Athena stored query results (optional).
	// Default results path can be defined by in workgroup settings.
	QueryResultsS3 string
	// Workgroup is Glue workgroup where Athena queries are executed (optional).
	Workgroup string
	// GetQueryResultsInterval is used to define how long query will wait before
	// checking again for results status if previous status was not ready (optional).
	GetQueryResultsInterval time.Duration

	// LimiterRefillTime determines the duration of time between the addition of tokens to the bucket (optional).
	LimiterRefillTime time.Duration
	// LimiterRefillAmount is the number of tokens that are added to the bucket during interval
	// specified by LimiterRefillTime (optional).
	LimiterRefillAmount int
	// Burst defines number of available tokens. It's initially full and refilled
	// based on LimiterRefillAmount and LimiterRefillTime (optional).
	LimiterBurst int

	// Batcher settings.

	// QueueURL is URL of SQS, which is set as subscriber to SNS topic (required).
	QueueURL string
	// BatchMaxItems defines how many items can be stored in single Parquet
	// batch (optional).
	// It's soft limit.
	BatchMaxItems int
	// BatchMaxInterval defined interval at which parquet files will be created (optional).
	BatchMaxInterval time.Duration

	// Clock is a clock interface, used in tests.
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator.
	UIDGenerator utils.UID
	// LogEntry is a log entry.
	LogEntry *log.Entry
	// AWSConfig is AWS config which can be used to construct varius AWS Clients
	// using aws-sdk-go-v2.
	AWSConfig *aws.Config

	Backend backend.Backend

	// Tracer is used to create spans
	Tracer oteltrace.Tracer

	// TODO(tobiaszheller): add FIPS config in later phase.
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to setup Athena based audit log.
func (cfg *Config) CheckAndSetDefaults(ctx context.Context) error {
	// AWS restrictions (https://docs.aws.amazon.com/athena/latest/ug/tables-databases-columns-names.html)
	const glueNameMaxLen = 255
	if cfg.Database == "" {
		return trace.BadParameter("Database is not specified")
	}
	if len(cfg.Database) > glueNameMaxLen {
		return trace.BadParameter("Database name too long")
	}
	if !isAlphanumericOrUnderscore(cfg.Database) {
		return trace.BadParameter("Database name can contains only alphanumeric or underscore characters")
	}

	if cfg.TableName == "" {
		return trace.BadParameter("TableName is not specified")
	}
	if len(cfg.TableName) > glueNameMaxLen {
		return trace.BadParameter("TableName too long")
	}
	// TableName is appended directly to athena query. That's why we put extra care
	// that no weird chars are passed here.
	if !isAlphanumericOrUnderscore(cfg.TableName) {
		return trace.BadParameter("TableName can contains only alphanumeric or underscore characters")
	}

	if cfg.TopicARN == "" {
		return trace.BadParameter("TopicARN is not specified")
	}

	if cfg.LocationS3 == "" {
		return trace.BadParameter("LocationS3 is not specified")
	}
	locationS3URL, err := url.Parse(cfg.LocationS3)
	if err != nil {
		return trace.BadParameter("LocationS3 must be valid url")
	}
	if locationS3URL.Scheme != "s3" {
		return trace.BadParameter("LocationS3 must starts with s3://")
	}
	cfg.locationS3Bucket = locationS3URL.Host
	cfg.locationS3Prefix = strings.TrimSuffix(strings.TrimPrefix(locationS3URL.Path, "/"), "/")

	if cfg.LargeEventsS3 == "" {
		return trace.BadParameter("LargeEventsS3 is not specified")
	}

	largeEventsS3URL, err := url.Parse(cfg.LargeEventsS3)
	if err != nil {
		return trace.BadParameter("LargeEventsS3 must be valid url")
	}
	if largeEventsS3URL.Scheme != "s3" {
		return trace.BadParameter("LargeEventsS3 must starts with s3://")
	}
	cfg.largeEventsBucket = largeEventsS3URL.Host
	cfg.largeEventsPrefix = strings.TrimSuffix(strings.TrimPrefix(largeEventsS3URL.Path, "/"), "/")

	if cfg.QueueURL == "" {
		return trace.BadParameter("QueueURL is not specified")
	}
	if scheme, ok := isValidUrlWithScheme(cfg.QueueURL); !ok || scheme != "https" {
		return trace.BadParameter("QueueURL must be valid url and start with https")
	}

	if cfg.GetQueryResultsInterval == 0 {
		cfg.GetQueryResultsInterval = 100 * time.Millisecond
	}

	if cfg.BatchMaxItems == 0 {
		cfg.BatchMaxItems = defaultBatchItems
	}

	if cfg.BatchMaxInterval == 0 {
		cfg.BatchMaxInterval = defaultBatchInterval
	}

	if cfg.BatchMaxInterval < maxWaitTimeOnReceiveMessageFromSQS {
		// If BatchMaxInterval is shorter it will mean we will cancel all
		// requests when there is less messages than 10 on queue.
		// This can be fixed by shortening timeout on read, but realisticly
		// no-one should use that short interval, so it's easier to check here.
		// For high load operation, BatchMaxItems will happen first.
		return trace.BadParameter("BatchMaxInterval too short, must be greater than 5s")
	}

	if cfg.LimiterRefillAmount < 0 {
		return trace.BadParameter("LimiterRefillAmount cannot be nagative")
	}
	if cfg.LimiterBurst < 0 {
		return trace.BadParameter("LimiterBurst cannot be negative")
	}

	if cfg.LimiterRefillAmount > 0 && cfg.LimiterBurst == 0 {
		return trace.BadParameter("LimiterBurst must be greater than 0 if LimiterRefillAmount is used")
	}

	if cfg.LimiterBurst > 0 && cfg.LimiterRefillAmount == 0 {
		return trace.BadParameter("LimiterRefillAmount must be greater than 0 if LimiterBurst is used")
	}

	if cfg.LimiterRefillAmount > 0 && cfg.LimiterRefillTime == 0 {
		cfg.LimiterRefillTime = time.Second
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UIDGenerator == nil {
		cfg.UIDGenerator = utils.NewRealUID()
	}

	if cfg.LogEntry == nil {
		cfg.LogEntry = log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAthena,
		})
	}

	if cfg.AWSConfig == nil {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		// override the default environment (region + credentials) with the values
		// from the config.
		if cfg.Region != "" {
			awsCfg.Region = cfg.Region
		}
		otelaws.AppendMiddlewares(&awsCfg.APIOptions)
		cfg.AWSConfig = &awsCfg
	}

	if cfg.Backend == nil {
		return trace.BadParameter("Backend cannot be nil")
	}

	if cfg.Tracer == nil {
		cfg.Tracer = tracing.NoopTracer(teleport.ComponentAthena)
	}

	return nil
}

// SetFromURL establishes values on an EventsConfig from the supplied URI
func (cfg *Config) SetFromURL(url *url.URL) error {
	splitted := strings.Split(url.Host, ".")
	if len(splitted) != 2 {
		return trace.BadParameter("invalid athena address, supported format is 'athena://database.table', got %q", url.Host)
	}
	cfg.Database, cfg.TableName = splitted[0], splitted[1]

	region := url.Query().Get("region")
	if region != "" {
		cfg.Region = region
	}
	topicARN := url.Query().Get("topicArn")
	if topicARN != "" {
		cfg.TopicARN = topicARN
	}
	largeEventsS3 := url.Query().Get("largeEventsS3")
	if largeEventsS3 != "" {
		cfg.LargeEventsS3 = largeEventsS3
	}

	locationS3 := url.Query().Get("locationS3")
	if locationS3 != "" {
		cfg.LocationS3 = locationS3
	}
	queryResultsS3 := url.Query().Get("queryResultsS3")
	if queryResultsS3 != "" {
		cfg.QueryResultsS3 = queryResultsS3
	}
	workgroup := url.Query().Get("workgroup")
	if workgroup != "" {
		cfg.Workgroup = workgroup
	}
	getQueryResultsInterval := url.Query().Get("getQueryResultsInterval")
	if getQueryResultsInterval != "" {
		dur, err := time.ParseDuration(getQueryResultsInterval)
		if err != nil {
			return trace.BadParameter("invalid getQueryResultsInterval value: %v", err)
		}
		cfg.GetQueryResultsInterval = dur
	}
	refillAmountInString := url.Query().Get("limiterRefillAmount")
	if refillAmountInString != "" {
		refillAmount, err := strconv.Atoi(refillAmountInString)
		if err != nil {
			return trace.BadParameter("invalid limiterRefillAmount value (it must be int): %v", err)
		}
		cfg.LimiterRefillAmount = refillAmount
	}
	refillTimeInString := url.Query().Get("limiterRefillTime")
	if refillTimeInString != "" {
		dur, err := time.ParseDuration(refillTimeInString)
		if err != nil {
			return trace.BadParameter("invalid limiterRefillTime value: %v", err)
		}
		cfg.LimiterRefillTime = dur
	}
	burstInString := url.Query().Get("limiterBurst")
	if burstInString != "" {
		burst, err := strconv.Atoi(burstInString)
		if err != nil {
			return trace.BadParameter("invalid limiterBurst value (it must be int): %v", err)
		}
		cfg.LimiterBurst = burst
	}

	queueURL := url.Query().Get("queueURL")
	if queueURL != "" {
		cfg.QueueURL = queueURL
	}
	batchMaxItems := url.Query().Get("batchMaxItems")
	if batchMaxItems != "" {
		intMaxItems, err := strconv.Atoi(batchMaxItems)
		if err != nil {
			return trace.BadParameter("invalid batchMaxItems value (it must be int): %v", err)
		}
		cfg.BatchMaxItems = intMaxItems
	}
	batchMaxInterval := url.Query().Get("batchMaxInterval")
	if batchMaxInterval != "" {
		dur, err := time.ParseDuration(batchMaxInterval)
		if err != nil {
			return trace.BadParameter("invalid batchMaxInterval value: %v", err)
		}
		cfg.BatchMaxInterval = dur
	}

	return nil
}

// Log is an events storage backend.
//
// It's using SNS for emitting events.
// SQS is used as subscriber for SNS topic.
// Consumer uses SQS to read multiple events, create batch, convert it to
// Parquet and send it to S3 for long term storage.
// Athena is used for quering Parquet files on S3.
type Log struct {
	publisher      *publisher
	querier        *querier
	consumerCloser io.Closer
}

// New creates an instance of an Athena based audit log.
func New(ctx context.Context, cfg Config) (*Log, error) {
	err := cfg.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// metricConsumerBatchProcessingDuration is defined after checking config, because
	// its bucket depends on batchMaxInterval.
	metricConsumerBatchProcessingDuration := metricConsumerBatchProcessingDuration(cfg.BatchMaxInterval)

	if err := metrics.RegisterPrometheusCollectors(append(prometheusCollectors, metricConsumerBatchProcessingDuration)...); err != nil {
		return nil, trace.Wrap(err)
	}

	querier, err := newQuerier(querierConfig{
		tablename:               cfg.TableName,
		database:                cfg.Database,
		workgroup:               cfg.Workgroup,
		queryResultsS3:          cfg.QueryResultsS3,
		getQueryResultsInterval: cfg.GetQueryResultsInterval,
		awsCfg:                  cfg.AWSConfig,
		logger:                  cfg.LogEntry,
		clock:                   cfg.Clock,
		tracer:                  cfg.Tracer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	consumerCtx, consumerCancel := context.WithCancel(ctx)

	consumer, err := newConsumer(cfg, consumerCancel, metricConsumerBatchProcessingDuration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := &Log{
		publisher:      newPublisherFromAthenaConfig(cfg),
		querier:        querier,
		consumerCloser: consumer,
	}

	go consumer.run(consumerCtx)

	return l, nil
}

func (l *Log) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	return trace.Wrap(l.publisher.EmitAuditEvent(ctx, in))
}

func (l *Log) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return l.querier.SearchEvents(ctx, req)
}

func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	return l.querier.SearchSessionEvents(ctx, req)
}

func (l *Log) Close() error {
	return trace.Wrap(l.consumerCloser.Close())
}

var isAlphanumericOrUnderscoreRe = regexp.MustCompile("^[a-zA-Z0-9_]+$")

func isAlphanumericOrUnderscore(s string) bool {
	return isAlphanumericOrUnderscoreRe.MatchString(s)
}

func isValidUrlWithScheme(s string) (string, bool) {
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	if u.Scheme == "" {
		return "", false
	}
	return u.Scheme, true
}

func metricConsumerBatchProcessingDuration(batchInterval time.Duration) prometheus.Histogram {
	batchSeconds := batchInterval.Seconds()
	return prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerBatchPorcessingDuration,
			Help:      "Duration of processing single batch of events in parquetlog",
			// For 60s batch interval it will look like:
			// 6.00, 12.00, 30.00, 45.00, 54.00, 59.01, 64.48, 70.47, 77.01, 84.15, 91.96, 100.49, 109.81, 120.00
			// We want some visibility if batch takes very small amount of time, but we are mostly interested
			// in range from 0.9*batch to 2*batch.
			Buckets: append([]float64{0.1 * batchSeconds, 0.2 * batchSeconds, 0.5 * batchSeconds, 0.75 * batchSeconds}, prometheus.ExponentialBucketsRange(0.9*batchSeconds, 2*batchSeconds, 10)...),
		},
	)
}

var (
	consumerS3parquetFlushDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerS3FlushDuration,
			Help:      "Duration of flush and close of s3 parquet files in parquetlog",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)

	consumerDeleteMessageDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerDeleteEventsDuration,
			Help:      "Duration of delation of events on SQS in parquetlog",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)

	consumerBatchSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerBatchSize,
			Help:      "Size of single batch of events in parquetlog",
			Buckets:   prometheus.ExponentialBucketsRange(200, 100*1024*1024 /* 100 MB*/, 10),
		},
	)

	consumerBatchCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerBatchCount,
			Help:      "Number of events in single batch in parquetlog",
		},
	)

	consumerLastProcessedTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerLastProcessedTimestamp,
			Help:      "Timestamp of last finished consumer execution",
		},
	)

	consumerAgeOfOldestProcessedMessage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerOldestProcessedMessage,
			Help:      "Age of oldest processed message in seconds",
		},
	)

	consumerNumberOfErrorsFromSQSCollect = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricParquetlogConsumerCollectFailed,
			Help:      "Number of errors received from sqs collect",
		},
	)

	prometheusCollectors = []prometheus.Collector{
		consumerS3parquetFlushDuration, consumerDeleteMessageDuration,
		consumerBatchSize, consumerBatchCount,
		consumerLastProcessedTimestamp, consumerAgeOfOldestProcessedMessage,
		consumerNumberOfErrorsFromSQSCollect,
	}
)
