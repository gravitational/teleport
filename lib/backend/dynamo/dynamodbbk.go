/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package dynamo

import (
	"bytes"
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams/dynamodbstreamsiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	dynamometrics "github.com/gravitational/teleport/lib/observability/metrics/dynamo"
)

func init() {
	backend.MustRegister(GetName(), func(ctx context.Context, p backend.Params) (backend.Backend, error) {
		return New(ctx, p)
	})
}

// Config structure represents DynamoDB configuration as appears in `storage` section
// of Teleport YAML
type Config struct {
	// Region is where DynamoDB Table will be used to store k/v
	Region string `json:"region,omitempty"`
	// AWS AccessKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	AccessKey string `json:"access_key,omitempty"`
	// AWS SecretKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	SecretKey string `json:"secret_key,omitempty"`
	// TableName where to store K/V in DynamoDB
	TableName string `json:"table_name,omitempty"`
	// ReadCapacityUnits is Dynamodb read capacity units
	ReadCapacityUnits int64 `json:"read_capacity_units"`
	// WriteCapacityUnits is Dynamodb write capacity units
	WriteCapacityUnits int64 `json:"write_capacity_units"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// PollStreamPeriod is a polling period for event stream
	PollStreamPeriod time.Duration `json:"poll_stream_period,omitempty"`
	// RetryPeriod is a period between dynamo backend retries on failures
	RetryPeriod time.Duration `json:"retry_period"`

	// EnableContinuousBackups is used to enables PITR (Point-In-Time Recovery).
	EnableContinuousBackups bool `json:"continuous_backups,omitempty"`

	// EnableAutoScaling is used to enable auto scaling policy.
	EnableAutoScaling bool `json:"auto_scaling,omitempty"`
	// ReadMaxCapacity is the maximum provisioned read capacity. Required to be
	// set if auto scaling is enabled.
	ReadMaxCapacity int64 `json:"read_max_capacity,omitempty"`
	// ReadMinCapacity is the minimum provisioned read capacity. Required to be
	// set if auto scaling is enabled.
	ReadMinCapacity int64 `json:"read_min_capacity,omitempty"`
	// ReadTargetValue is the ratio of consumed read capacity to provisioned
	// capacity. Required to be set if auto scaling is enabled.
	ReadTargetValue float64 `json:"read_target_value,omitempty"`
	// WriteMaxCapacity is the maximum provisioned write capacity. Required to
	// be set if auto scaling is enabled.
	WriteMaxCapacity int64 `json:"write_max_capacity,omitempty"`
	// WriteMinCapacity is the minimum provisioned write capacity. Required to
	// be set if auto scaling is enabled.
	WriteMinCapacity int64 `json:"write_min_capacity,omitempty"`
	// WriteTargetValue is the ratio of consumed write capacity to provisioned
	// capacity. Required to be set if auto scaling is enabled.
	WriteTargetValue float64 `json:"write_target_value,omitempty"`

	// BillingMode sets on-demand capacity to the DynamoDB tables
	BillingMode billingMode `json:"billing_mode,omitempty"`
}

type billingMode string

const (
	billingModeProvisioned   billingMode = "provisioned"
	billingModePayPerRequest billingMode = "pay_per_request"
)

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to DynamoDB
func (cfg *Config) CheckAndSetDefaults() error {
	// Table name is required.
	if cfg.TableName == "" {
		return trace.BadParameter("DynamoDB: table_name is not specified")
	}

	if cfg.BillingMode == "" {
		cfg.BillingMode = billingModePayPerRequest
	}

	if cfg.ReadCapacityUnits == 0 {
		cfg.ReadCapacityUnits = DefaultReadCapacityUnits
	}
	if cfg.WriteCapacityUnits == 0 {
		cfg.WriteCapacityUnits = DefaultWriteCapacityUnits
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	if cfg.PollStreamPeriod == 0 {
		cfg.PollStreamPeriod = backend.DefaultPollStreamPeriod
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	}

	return nil
}

// Backend is a DynamoDB-backed key value backend implementation.
type Backend struct {
	*log.Entry
	Config
	svc     dynamodbiface.DynamoDBAPI
	streams dynamodbstreamsiface.DynamoDBStreamsAPI
	clock   clockwork.Clock
	buf     *backend.CircularBuffer
	// closedFlag is set to indicate that the database is closed
	closedFlag int32

	// session holds the AWS client.
	session *session.Session
}

type record struct {
	HashKey   string
	FullPath  string
	Value     []byte
	Timestamp int64
	Expires   *int64 `json:"Expires,omitempty"`
	ID        int64
	Revision  string
}

type keyLookup struct {
	HashKey  string
	FullPath string
}

const (
	// hashKey is actually the name of the partition. This backend
	// places all objects in the same DynamoDB partition
	hashKey = "teleport"

	// obsolete schema key. if a table contains "Key" column it means
	// such table needs to be migrated
	oldPathAttr = "Key"

	// BackendName is the name of this backend
	BackendName = "dynamodb"

	// ttlKey is a key used for TTL specification
	ttlKey = "Expires"

	// DefaultReadCapacityUnits specifies default value for read capacity units
	DefaultReadCapacityUnits = 10

	// DefaultWriteCapacityUnits specifies default value for write capacity units
	DefaultWriteCapacityUnits = 10

	// fullPathKey is a name of the full path key
	fullPathKey = "FullPath"

	// hashKeyKey is a name of the hash key
	hashKeyKey = "HashKey"

	// keyPrefix is a prefix that is added to every dynamodb key
	// for backwards compatibility
	keyPrefix = "teleport"
)

// GetName is a part of backend API and it returns DynamoDB backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

// keep this here to test interface conformance
var _ backend.Backend = &Backend{}

// New returns new instance of DynamoDB backend.
// It's an implementation of backend API's NewFunc
func New(ctx context.Context, params backend.Params) (*Backend, error) {
	l := log.WithFields(log.Fields{teleport.ComponentKey: BackendName})

	var cfg *Config
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("DynamoDB configuration is invalid: %v", err)
	}

	defer l.Debug("AWS session is created.")

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	l.Infof("Initializing backend. Table: %q, poll streams every %v.", cfg.TableName, cfg.PollStreamPeriod)

	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)
	b := &Backend{
		Entry:  l,
		Config: *cfg,
		clock:  clockwork.NewRealClock(),
		buf:    buf,
	}

	// determine if the FIPS endpoints should be used
	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if modules.GetModules().IsBoringBinary() {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	awsConfig := aws.Config{}
	if cfg.Region != "" {
		awsConfig.Region = aws.String(cfg.Region)
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, "")
	}

	b.session, err = session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            awsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Increase the size of the connection pool. This substantially improves the
	// performance of Teleport under load as it reduces the number of TLS
	// handshakes performed.
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        defaults.HTTPMaxIdleConns,
			MaxIdleConnsPerHost: defaults.HTTPMaxIdleConnsPerHost,
		},
	}
	b.session.Config.HTTPClient = httpClient

	// Create DynamoDB service.
	svc, err := dynamometrics.NewAPIMetrics(dynamometrics.Backend, dynamodb.New(b.session, &aws.Config{
		// Setting this on the individual service instead of the session, as DynamoDB Streams
		// and Application Auto Scaling do not yet have FIPS endpoints in non-GovCloud.
		// See also: https://aws.amazon.com/compliance/fips/#FIPS_Endpoints_by_Service
		UseFIPSEndpoint: useFIPSEndpoint,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.svc = svc
	streams, err := dynamometrics.NewStreamsMetricsAPI(dynamometrics.Backend, dynamodbstreams.New(b.session))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.streams = streams

	// check if the table exists?
	ts, tableBillingMode, err := b.getTableStatus(ctx, b.TableName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		if tableBillingMode == dynamodb.BillingModePayPerRequest {
			cfg.EnableAutoScaling = false
			l.Info("Ignoring auto_scaling setting as table is in on-demand mode.")
		}
	case tableStatusMissing:
		if cfg.BillingMode == billingModePayPerRequest {
			cfg.EnableAutoScaling = false
			l.Info("Ignoring auto_scaling setting as table is being created in on-demand mode.")
		}
		err = b.createTable(ctx, b.TableName, fullPathKey)
	case tableStatusNeedsMigration:
		return nil, trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Enable TTL on table.
	err = TurnOnTimeToLive(ctx, b.svc, b.TableName, ttlKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Turn on DynamoDB streams, needed to implement events.
	err = TurnOnStreams(ctx, b.svc, b.TableName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Enable continuous backups if requested.
	if b.Config.EnableContinuousBackups {
		if err := SetContinuousBackups(ctx, b.svc, b.TableName); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Enable auto scaling if requested.
	if b.Config.EnableAutoScaling {
		if err := SetAutoScaling(ctx, applicationautoscaling.New(b.session), GetTableID(b.TableName), AutoScalingParams{
			ReadMinCapacity:  b.Config.ReadMinCapacity,
			ReadMaxCapacity:  b.Config.ReadMaxCapacity,
			ReadTargetValue:  b.Config.ReadTargetValue,
			WriteMinCapacity: b.Config.WriteMinCapacity,
			WriteMaxCapacity: b.Config.WriteMaxCapacity,
			WriteTargetValue: b.Config.WriteTargetValue,
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	go func() {
		if err := b.asyncPollStreams(ctx); err != nil {
			b.Errorf("Stream polling loop exited: %v", err)
		}
	}()

	// Wrap backend in a input sanitizer and return it.
	return b, nil
}

func (b *Backend) GetName() string {
	return GetName()
}

// Create creates item if it does not exist
func (b *Backend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	rev, err := b.create(ctx, item, modeCreate)
	if trace.IsCompareFailed(err) {
		err = trace.AlreadyExists(err.Error())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item.Revision = rev
	return backend.NewLease(item), nil
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (b *Backend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	rev, err := b.create(ctx, item, modePut)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item.Revision = rev
	return backend.NewLease(item), nil
}

// Update updates value in the backend
func (b *Backend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	rev, err := b.create(ctx, item, modeUpdate)
	if trace.IsCompareFailed(err) {
		err = trace.NotFound(err.Error())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item.Revision = rev
	return backend.NewLease(item), nil
}

// GetRange returns range of elements
func (b *Backend) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}

	result, err := b.getAllRecords(ctx, startKey, endKey, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(records(result.records))
	values := make([]backend.Item, len(result.records))
	for i, r := range result.records {
		values[i] = backend.Item{
			Key:      trimPrefix(r.FullPath),
			Value:    r.Value,
			ID:       r.ID,
			Revision: r.Revision,
		}
		if r.Expires != nil {
			values[i].Expires = time.Unix(*r.Expires, 0).UTC()
		}
		if values[i].Revision == "" {
			values[i].Revision = backend.BlankRevision
		}
	}
	return &backend.GetResult{Items: values}, nil
}

func (b *Backend) getAllRecords(ctx context.Context, startKey []byte, endKey []byte, limit int) (*getResult, error) {
	var result getResult

	// this code is being extra careful here not to introduce endless loop
	// by some unfortunate series of events
	for i := 0; i < backend.DefaultRangeLimit/100; i++ {
		re, err := b.getRecords(ctx, prependPrefix(startKey), prependPrefix(endKey), limit, result.lastEvaluatedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.records = append(result.records, re.records...)
		// If the limit was exceeded or there are no more records to fetch return the current result
		// otherwise updated lastEvaluatedKey and proceed with obtaining new records.
		if (limit != 0 && len(result.records) >= limit) || len(re.lastEvaluatedKey) == 0 {
			if len(result.records) == backend.DefaultRangeLimit {
				b.Warnf("Range query hit backend limit. (this is a bug!) startKey=%q,limit=%d", startKey, backend.DefaultRangeLimit)
			}
			result.lastEvaluatedKey = nil
			return &result, nil
		}
		result.lastEvaluatedKey = re.lastEvaluatedKey
	}
	return nil, trace.BadParameter("backend entered endless loop")
}

const (
	// batchOperationItemsLimit is the maximum number of items that can be put or deleted in a single batch operation.
	// From https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Limits.html:
	// A single call to BatchWriteItem can transmit up to 16MB of data over the network,
	// consisting of up to 25 item put or delete operations.
	batchOperationItemsLimit = 25
)

// DeleteRange deletes range of items with keys between startKey and endKey
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}
	// keep fetching and deleting until no records left,
	// keep the very large limit, just in case if someone else
	// keeps adding records
	for i := 0; i < backend.DefaultRangeLimit/100; i++ {
		result, err := b.getRecords(ctx, prependPrefix(startKey), prependPrefix(endKey), batchOperationItemsLimit, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(result.records) == 0 {
			return nil
		}
		requests := make([]*dynamodb.WriteRequest, 0, len(result.records))
		for _, record := range result.records {
			requests = append(requests, &dynamodb.WriteRequest{
				DeleteRequest: &dynamodb.DeleteRequest{
					Key: map[string]*dynamodb.AttributeValue{
						hashKeyKey: {
							S: aws.String(hashKey),
						},
						fullPathKey: {
							S: aws.String(record.FullPath),
						},
					},
				},
			})
		}
		input := dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]*dynamodb.WriteRequest{
				b.TableName: requests,
			},
		}

		if _, err = b.svc.BatchWriteItemWithContext(ctx, &input); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.ConnectionProblem(nil, "not all items deleted, too many requests")
}

// Get returns a single item or not found error
func (b *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	r, err := b.getKey(ctx, key)
	if err != nil {
		return nil, err
	}

	item := &backend.Item{
		Key:      trimPrefix(r.FullPath),
		Value:    r.Value,
		ID:       r.ID,
		Revision: r.Revision,
	}
	if r.Expires != nil {
		item.Expires = time.Unix(*r.Expires, 0)
	}

	if item.Revision == "" {
		item.Revision = backend.BlankRevision
	}
	return item, nil
}

// CompareAndSwap compares and swap values in atomic operation
// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (b *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	replaceWith.Revision = backend.CreateRevision()
	r := record{
		HashKey:   hashKey,
		FullPath:  prependPrefix(replaceWith.Key),
		Value:     replaceWith.Value,
		Timestamp: time.Now().UTC().Unix(),
		ID:        time.Now().UTC().UnixNano(),
		Revision:  replaceWith.Revision,
	}
	if !replaceWith.Expires.IsZero() {
		r.Expires = aws.Int64(replaceWith.Expires.UTC().Unix())
	}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.TableName),
	}
	input.SetConditionExpression("#v = :prev")
	input.SetExpressionAttributeNames(map[string]*string{
		"#v": aws.String("Value"),
	})
	input.SetExpressionAttributeValues(map[string]*dynamodb.AttributeValue{
		":prev": {
			B: expected.Value,
		},
	})
	_, err = b.svc.PutItemWithContext(ctx, &input)
	err = convertError(err)
	if err != nil {
		// in this case let's use more specific compare failed error
		if trace.IsAlreadyExists(err) {
			return nil, trace.CompareFailed(err.Error())
		}
		return nil, trace.Wrap(err)
	}
	return backend.NewLease(replaceWith), nil
}

// Delete deletes item by key
func (b *Backend) Delete(ctx context.Context, key []byte) error {
	if _, err := b.getKey(ctx, key); err != nil {
		return err
	}
	return b.deleteKey(ctx, key)
}

// ConditionalUpdate updates the matching item in Dynamo if the provided revision matches
// the revision of the item in Dynamo.
func (b *Backend) ConditionalUpdate(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	if item.Revision == "" {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	if item.Revision == backend.BlankRevision {
		item.Revision = ""
	}

	rev, err := b.create(ctx, item, modeConditionalUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item.Revision = rev
	return backend.NewLease(item), nil
}

// ConditionalDelete deletes item by key if the provided revision matches
// the revision of the item in Dynamo.
func (b *Backend) ConditionalDelete(ctx context.Context, key []byte, rev string) error {
	if rev == "" {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	av, err := dynamodbattribute.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: prependPrefix(key),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	input := dynamodb.DeleteItemInput{Key: av, TableName: aws.String(b.TableName)}

	if rev == backend.BlankRevision {
		input.SetConditionExpression("attribute_not_exists(Revision) AND attribute_exists(FullPath)")
	} else {
		input.SetExpressionAttributeValues(map[string]*dynamodb.AttributeValue{":rev": {S: aws.String(rev)}})
		input.SetConditionExpression("Revision = :rev AND attribute_exists(FullPath)")
	}

	if _, err = b.svc.DeleteItemWithContext(ctx, &input); err != nil {
		err = convertError(err)
		if trace.IsCompareFailed(err) {
			return trace.Wrap(backend.ErrIncorrectRevision)
		}
		return trace.Wrap(err)
	}
	return nil
}

// NewWatcher returns a new event watcher
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if len(lease.Key) == 0 {
		return trace.BadParameter("lease is missing key")
	}
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":expires": {
				N: aws.String(strconv.FormatInt(expires.UTC().Unix(), 10)),
			},
			":timestamp": {
				N: aws.String(strconv.FormatInt(b.clock.Now().UTC().Unix(), 10)),
			},
		},
		TableName: aws.String(b.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			hashKeyKey: {
				S: aws.String(hashKey),
			},
			fullPathKey: {
				S: aws.String(prependPrefix(lease.Key)),
			},
		},
		UpdateExpression: aws.String("SET Expires = :expires"),
	}
	input.SetConditionExpression("attribute_exists(FullPath) AND (attribute_not_exists(Expires) OR Expires >= :timestamp)")
	_, err := b.svc.UpdateItemWithContext(ctx, input)
	err = convertError(err)
	if trace.IsCompareFailed(err) {
		err = trace.NotFound(err.Error())
	}
	return err
}

func (b *Backend) isClosed() bool {
	return atomic.LoadInt32(&b.closedFlag) == 1
}

func (b *Backend) setClosed() {
	atomic.StoreInt32(&b.closedFlag, 1)
}

// Close closes the DynamoDB driver
// and releases associated resources
func (b *Backend) Close() error {
	b.setClosed()
	return b.buf.Close()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (b *Backend) CloseWatchers() {
	b.buf.Clear()
}

type tableStatus int

const (
	tableStatusError = iota
	tableStatusMissing
	tableStatusNeedsMigration
	tableStatusOK
)

// Clock returns wall clock
func (b *Backend) Clock() clockwork.Clock {
	return b.clock
}

// getTableStatus checks if a given table exists
func (b *Backend) getTableStatus(ctx context.Context, tableName string) (tableStatus, string, error) {
	td, err := b.svc.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	err = convertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return tableStatusMissing, "", nil
		}
		return tableStatusError, "", trace.Wrap(err)
	}
	for _, attr := range td.Table.AttributeDefinitions {
		if *attr.AttributeName == oldPathAttr {
			return tableStatusNeedsMigration, "", nil
		}
	}
	// the billing mode can be empty unless it was specified on the
	// initial create table request, and the default billing mode is
	// PROVISIONED, if unspecified.
	// https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BillingModeSummary.html
	if td.Table.BillingModeSummary == nil {
		return tableStatusOK, dynamodb.BillingModeProvisioned, nil
	}
	return tableStatusOK, aws.StringValue(td.Table.BillingModeSummary.BillingMode), nil
}

// createTable creates a DynamoDB table with a requested name and applies
// the back-end schema to it. The table must not exist.
//
// rangeKey is the name of the 'range key' the schema requires.
// currently is always set to "FullPath" (used to be something else, that's
// why it's a parameter for migration purposes)
//
// Note: If we change DynamoDB table schemas, we must also update the
// documentation in case users want to set up DynamoDB tables manually. Edit the
// following docs partial:
// docs/pages/includes/dynamodb-iam-policy.mdx
func (b *Backend) createTable(ctx context.Context, tableName string, rangeKey string) error {
	billingMode := aws.String(dynamodb.BillingModeProvisioned)
	pThroughput := &dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(b.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(b.WriteCapacityUnits),
	}
	if b.BillingMode == billingModePayPerRequest {
		billingMode = aws.String(dynamodb.BillingModePayPerRequest)
		pThroughput = nil
	}

	def := []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String(hashKeyKey),
			AttributeType: aws.String("S"),
		},
		{
			AttributeName: aws.String(rangeKey),
			AttributeType: aws.String("S"),
		},
	}
	elems := []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String(hashKeyKey),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String(rangeKey),
			KeyType:       aws.String("RANGE"),
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             aws.String(tableName),
		AttributeDefinitions:  def,
		KeySchema:             elems,
		ProvisionedThroughput: pThroughput,
		BillingMode:           billingMode,
	}
	_, err := b.svc.CreateTableWithContext(ctx, &c)
	if err != nil {
		return trace.Wrap(err)
	}
	b.Infof("Waiting until table %q is created.", tableName)
	err = b.svc.WaitUntilTableExistsWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		b.Infof("Table %q has been created.", tableName)
	}
	return trace.Wrap(err)
}

type getResult struct {
	records []record
	// lastEvaluatedKey is the primary key of the item where the operation stopped, inclusive of the
	// previous result set. Use this value to start a new operation, excluding this
	// value in the new request.
	lastEvaluatedKey map[string]*dynamodb.AttributeValue
}

// getRecords retrieves all keys by path
func (b *Backend) getRecords(ctx context.Context, startKey, endKey string, limit int, lastEvaluatedKey map[string]*dynamodb.AttributeValue) (*getResult, error) {
	query := "HashKey = :hashKey AND FullPath BETWEEN :fullPath AND :rangeEnd"
	attrV := map[string]interface{}{
		":fullPath":  startKey,
		":hashKey":   hashKey,
		":timestamp": b.clock.Now().UTC().Unix(),
		":rangeEnd":  endKey,
	}

	// filter out expired items, otherwise they might show up in the query
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/howitworks-ttl.html
	filter := "attribute_not_exists(Expires) OR Expires >= :timestamp"
	av, err := dynamodbattribute.MarshalMap(attrV)
	if err != nil {
		return nil, convertError(err)
	}
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 &b.TableName,
		ExpressionAttributeValues: av,
		FilterExpression:          aws.String(filter),
		ConsistentRead:            aws.Bool(true),
		ExclusiveStartKey:         lastEvaluatedKey,
	}
	if limit > 0 {
		input.Limit = aws.Int64(int64(limit))
	}
	out, err := b.svc.QueryWithContext(ctx, &input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result getResult
	for _, item := range out.Items {
		var r record
		if err := dynamodbattribute.UnmarshalMap(item, &r); err != nil {
			return nil, trace.Wrap(err)
		}
		result.records = append(result.records, r)
	}
	sort.Sort(records(result.records))
	result.records = removeDuplicates(result.records)
	result.lastEvaluatedKey = out.LastEvaluatedKey
	return &result, nil
}

// isExpired returns 'true' if the given object (record) has a TTL and
// it's due.
func (r *record) isExpired(now time.Time) bool {
	if r.Expires == nil {
		return false
	}
	expiryDateUTC := time.Unix(*r.Expires, 0).UTC()
	return now.UTC().After(expiryDateUTC)
}

func removeDuplicates(elements []record) []record {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []record{}

	for v := range elements {
		if encountered[elements[v].FullPath] {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v].FullPath] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

const (
	modeCreate = iota
	modePut
	modeUpdate
	modeConditionalUpdate
)

// prependPrefix adds leading 'teleport/' to the key for backwards compatibility
// with previous implementation of DynamoDB backend
func prependPrefix(key []byte) string {
	return keyPrefix + string(key)
}

// trimPrefix removes leading 'teleport' from the key
func trimPrefix(key string) []byte {
	return []byte(strings.TrimPrefix(key, keyPrefix))
}

// create is a helper that writes a key/value pair in Dynamo with a given expiration.
// Depending on the mode provided, the item will must various conditions met before
// the item is persisted in the database. On a successful write the revision of the
// item is returned.
func (b *Backend) create(ctx context.Context, item backend.Item, mode int) (string, error) {
	r := record{
		HashKey:   hashKey,
		FullPath:  prependPrefix(item.Key),
		Value:     item.Value,
		Timestamp: time.Now().UTC().Unix(),
		ID:        time.Now().UTC().UnixNano(),
		Revision:  backend.CreateRevision(),
	}
	if !item.Expires.IsZero() {
		r.Expires = aws.Int64(item.Expires.UTC().Unix())
	}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return "", trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.TableName),
	}

	switch mode {
	case modeCreate:
		input.SetConditionExpression("attribute_not_exists(FullPath)")
	case modeUpdate:
		input.SetConditionExpression("attribute_exists(FullPath)")
	case modePut:
	case modeConditionalUpdate:
		// If the revision is empty, then the resource existed prior to revision support. Instead of validating that
		// the revisions match, validate that the revision attribute does not exist. Otherwise, validate that the revision
		// attribute matches the item revision.
		if item.Revision == "" {
			input.SetConditionExpression("attribute_not_exists(Revision) AND attribute_exists(FullPath)")
		} else {
			input.SetExpressionAttributeValues(map[string]*dynamodb.AttributeValue{":rev": {S: aws.String(item.Revision)}})
			input.SetConditionExpression("Revision = :rev AND attribute_exists(FullPath)")
		}
	default:
		return "", trace.BadParameter("unrecognized mode")
	}
	_, err = b.svc.PutItemWithContext(ctx, &input)
	err = convertError(err)
	if err != nil {
		if mode == modeConditionalUpdate && trace.IsCompareFailed(err) {
			return "", trace.Wrap(backend.ErrIncorrectRevision)
		}

		return "", trace.Wrap(err)
	}

	return r.Revision, nil
}

func (b *Backend) deleteKey(ctx context.Context, key []byte) error {
	av, err := dynamodbattribute.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: prependPrefix(key),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.DeleteItemInput{Key: av, TableName: aws.String(b.TableName)}
	if _, err = b.svc.DeleteItemWithContext(ctx, &input); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *Backend) deleteKeyIfExpired(ctx context.Context, key []byte) error {
	_, err := b.svc.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(b.TableName),
		Key:       keyToAttributeValueMap(key),

		// succeed if the item no longer exists
		ConditionExpression: aws.String(
			"attribute_not_exists(FullPath) OR (attribute_exists(Expires) AND Expires <= :timestamp)",
		),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":timestamp": timeToAttributeValue(b.clock.Now()),
		},
	})
	return trace.Wrap(err)
}

func (b *Backend) getKey(ctx context.Context, key []byte) (*record, error) {
	av, err := dynamodbattribute.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: prependPrefix(key),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.GetItemInput{
		Key:            av,
		TableName:      aws.String(b.TableName),
		ConsistentRead: aws.Bool(true),
	}
	out, err := b.svc.GetItemWithContext(ctx, &input)
	if err != nil {
		// we deliberately use a "generic" trace error here, since we don't want
		// callers to make assumptions about the nature of the failure.
		return nil, trace.WrapWithMessage(err, "failed to get %q (dynamo error)", string(key))
	}
	if len(out.Item) == 0 {
		return nil, trace.NotFound("%q is not found", string(key))
	}
	var r record
	if err := dynamodbattribute.UnmarshalMap(out.Item, &r); err != nil {
		return nil, trace.WrapWithMessage(err, "failed to unmarshal dynamo item %q", string(key))
	}
	// Check if key expired, if expired delete it
	if r.isExpired(b.clock.Now()) {
		if err := b.deleteKeyIfExpired(ctx, key); err != nil {
			b.Warnf("Failed deleting expired key %q: %v", key, err)
		}
		return nil, trace.NotFound("%q is not found", key)
	}
	return &r, nil
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return err
	}
	switch aerr.Code() {
	case dynamodb.ErrCodeConditionalCheckFailedException:
		return trace.CompareFailed(aerr.Error())
	case dynamodb.ErrCodeProvisionedThroughputExceededException:
		return trace.ConnectionProblem(aerr, aerr.Error())
	case dynamodb.ErrCodeResourceNotFoundException, applicationautoscaling.ErrCodeObjectNotFoundException:
		return trace.NotFound(aerr.Error())
	case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
		return trace.BadParameter(aerr.Error())
	case dynamodb.ErrCodeInternalServerError:
		return trace.BadParameter(aerr.Error())
	case dynamodbstreams.ErrCodeExpiredIteratorException, dynamodbstreams.ErrCodeLimitExceededException, dynamodbstreams.ErrCodeTrimmedDataAccessException:
		return trace.ConnectionProblem(aerr, aerr.Error())
	default:
		return err
	}
}

type records []record

// Len is part of sort.Interface.
func (r records) Len() int {
	return len(r)
}

// Swap is part of sort.Interface.
func (r records) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Less is part of sort.Interface.
func (r records) Less(i, j int) bool {
	return r[i].FullPath < r[j].FullPath
}

func fullPathToAttributeValueMap(fullPath string) map[string]*dynamodb.AttributeValue {
	return map[string]*dynamodb.AttributeValue{
		hashKeyKey:  {S: aws.String(hashKey)},
		fullPathKey: {S: aws.String(fullPath)},
	}
}

func keyToAttributeValueMap(key []byte) map[string]*dynamodb.AttributeValue {
	return fullPathToAttributeValueMap(prependPrefix(key))
}

func timeToAttributeValue(t time.Time) *dynamodb.AttributeValue {
	return &dynamodb.AttributeValue{
		N: aws.String(strconv.FormatInt(t.Unix(), 10)),
	}
}
