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
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// TODO(tobiaszheller): move to batcher.go in other PR.
	// maxWaitTimeOnReceiveMessageFromSQS defines how long single
	// receiveFromQueue will wait if there is no max events (10).
	maxWaitTimeOnReceiveMessageFromSQS = 5 * time.Second
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
	LargeEventsS3 string

	// Query settings.

	// Database is name of Glue Database that Athena will query against (required).
	Database string
	// TableName is name of Glue Table that Athena will query against (required).
	TableName string
	// LocationS3 is location on S3 where Parquet files partitioned by date are
	// stored (required).
	LocationS3 string
	// QueryResultsS3 is location on S3 where Athena stored query results (optional).
	// Default results path can be defined by in workgroup settings.
	QueryResultsS3 string
	// Workgroup is Glue workgroup where Athena queries are executed (optional).
	Workgroup string
	// GetQueryResultsInterval is used to define how long query will wait before
	// checking again for results status if previous status was not ready (optional).
	GetQueryResultsInterval time.Duration
	// LimiterRate defines rate at which search_event rate limiter is filled (optional).
	LimiterRate float64
	// LimiterBurst defines rate limit bucket capacity (optional).
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

	// TODO(tobiaszheller): add FIPS config in later phase.
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to setup Athena based audit log.
func (cfg *Config) CheckAndSetDefaults() error {
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
	if scheme, ok := isValidUrlWithScheme(cfg.LocationS3); !ok || scheme != "s3" {
		return trace.BadParameter("LocationS3 must be valid url and start with s3")
	}

	if cfg.LargeEventsS3 == "" {
		return trace.BadParameter("LargeEventsS3 is not specified")
	}
	if scheme, ok := isValidUrlWithScheme(cfg.LargeEventsS3); !ok || scheme != "s3" {
		return trace.BadParameter("LargeEventsS3 must be valid url and start with s3")
	}

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
		// 20000 items, per average 500KB event size = 10MB
		cfg.BatchMaxItems = 20000
	}

	if cfg.BatchMaxInterval == 0 {
		cfg.BatchMaxInterval = 1 * time.Minute
	}

	if cfg.BatchMaxInterval < maxWaitTimeOnReceiveMessageFromSQS {
		// If BatchMaxInterval is shorter it will mean we will cancel all
		// requests when there is less messages than 10 on queue.
		// This can be fixed by shortening timeout on read, but realisticly
		// no-one should use that short interval, so it's easier to check here.
		// For high load operation, BatchMaxItems will happen first.
		return trace.BadParameter("BatchMaxInterval too short, must be greater than 5s")
	}

	if cfg.LimiterRate < 0 {
		return trace.BadParameter("LimiterRate cannot be nagative")
	}
	if cfg.LimiterBurst < 0 {
		return trace.BadParameter("LimiterBurst cannot be nagative")
	}

	if cfg.LimiterRate > 0 && cfg.LimiterBurst == 0 {
		return trace.BadParameter("LimiterBurst must be greater than 0 if LimiterRate is used")
	}

	if cfg.LimiterBurst > 0 && math.Abs(cfg.LimiterRate) < 1e-9 {
		return trace.BadParameter("LimiterRate must be greater than 0 if LimiterBurst is used")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UIDGenerator == nil {
		cfg.UIDGenerator = utils.NewRealUID()
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
	rateInString := url.Query().Get("limiterRate")
	if rateInString != "" {
		rate, err := strconv.ParseFloat(rateInString, 32)
		if err != nil {
			return trace.BadParameter("invalid limiterRate value (it must be float32): %v", err)
		}
		cfg.LimiterRate = rate
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
	// Entry is a log entry
	*log.Entry
	// Config is a backend configuration
	Config

	awsConfig aws.Config
}

// New creates an instance of an Athena based audit log.
func New(ctx context.Context, cfg Config) (*Log, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logEntry := log.WithFields(log.Fields{
		trace.Component: teleport.ComponentAthena,
	})
	l := &Log{
		Entry:  logEntry,
		Config: cfg,
	}

	l.awsConfig, err = awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// override the default environment (region + credentials) with the values
	// from the config.
	if cfg.Region != "" {
		l.awsConfig.Region = cfg.Region
	}

	// TODO(tobiaszheller): initialize publisher
	// TODO(tobiaszheller): initialize batcher
	// TODO(tobiaszheller): initialize querier

	return l, nil
}

func (l *Log) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	return trace.NotImplemented("not implemented")
}

func (l *Log) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (l *Log) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (l *Log) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	return nil, "", trace.NotImplemented("not implemented")
}

func (l *Log) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr, sessionID string) ([]apievents.AuditEvent, string, error) {
	return nil, "", trace.NotImplemented("not implemented")
}

func (l *Log) Close() error {
	return nil
}

func (l *Log) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	e <- trace.NotImplemented("not implemented")
	return c, e
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
