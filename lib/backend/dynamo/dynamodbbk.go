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
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	awsmetrics "github.com/gravitational/teleport/lib/observability/metrics/aws"
	dynamometrics "github.com/gravitational/teleport/lib/observability/metrics/dynamo"
	"github.com/gravitational/teleport/lib/utils/aws/endpoint"
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
	// BillingMode sets on-demand capacity to the DynamoDB tables
	BillingMode billingMode `json:"billing_mode,omitempty"`
	// RetryPeriod is a period between dynamo backend retries on failures
	RetryPeriod time.Duration `json:"retry_period"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// PollStreamPeriod is a polling period for event stream
	PollStreamPeriod time.Duration `json:"poll_stream_period,omitempty"`
	// WriteCapacityUnits is Dynamodb write capacity units
	WriteCapacityUnits int64 `json:"write_capacity_units"`
	// ReadTargetValue is the ratio of consumed read capacity to provisioned
	// capacity. Required to be set if auto scaling is enabled.
	ReadTargetValue float64 `json:"read_target_value,omitempty"`
	// WriteTargetValue is the ratio of consumed write capacity to provisioned
	// capacity. Required to be set if auto scaling is enabled.
	WriteTargetValue float64 `json:"write_target_value,omitempty"`
	// ReadCapacityUnits is Dynamodb read capacity units
	ReadCapacityUnits int64 `json:"read_capacity_units"`
	// ReadMaxCapacity is the maximum provisioned read capacity. Required to be
	// set if auto scaling is enabled.
	ReadMaxCapacity int32 `json:"read_max_capacity,omitempty"`
	// ReadMinCapacity is the minimum provisioned read capacity. Required to be
	// set if auto scaling is enabled.
	ReadMinCapacity int32 `json:"read_min_capacity,omitempty"`
	// WriteMaxCapacity is the maximum provisioned write capacity. Required to
	// be set if auto scaling is enabled.
	WriteMaxCapacity int32 `json:"write_max_capacity,omitempty"`
	// WriteMinCapacity is the minimum provisioned write capacity. Required to
	// be set if auto scaling is enabled.
	WriteMinCapacity int32 `json:"write_min_capacity,omitempty"`
	// EnableContinuousBackups is used to enables PITR (Point-In-Time Recovery).
	EnableContinuousBackups bool `json:"continuous_backups,omitempty"`
	// EnableAutoScaling is used to enable auto scaling policy.
	EnableAutoScaling bool `json:"auto_scaling,omitempty"`
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

type dynamoClient interface {
	DescribeTimeToLive(ctx context.Context, params *dynamodb.DescribeTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error)
	UpdateTimeToLive(ctx context.Context, params *dynamodb.UpdateTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTimeToLiveOutput, error)
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	UpdateTable(ctx context.Context, params *dynamodb.UpdateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTableOutput, error)
	DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error)
	UpdateContinuousBackups(ctx context.Context, params *dynamodb.UpdateContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateContinuousBackupsOutput, error)
	DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

// Backend is a DynamoDB-backed key value backend implementation.
type Backend struct {
	svc     dynamoClient
	clock   clockwork.Clock
	logger  *slog.Logger
	streams *dynamodbstreams.Client
	buf     *backend.CircularBuffer
	Config
	// closedFlag is set to indicate that the database is closed
	closedFlag int32
}

type record struct {
	Expires   *int64 `json:"Expires,omitempty" dynamodbav:",omitempty"`
	HashKey   string
	FullPath  string
	Revision  string
	Value     []byte
	Timestamp int64
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
)

const (
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
	l := slog.With(teleport.ComponentKey, BackendName)

	var cfg *Config
	if err := utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("DynamoDB configuration is invalid: %v", err)
	}

	defer l.DebugContext(ctx, "AWS session is created.")

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	l.InfoContext(ctx, "Initializing backend", "table", cfg.TableName, "poll_stream_period", cfg.PollStreamPeriod)

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        defaults.HTTPMaxIdleConns,
				MaxIdleConnsPerHost: defaults.HTTPMaxIdleConnsPerHost,
			},
		}),
		config.WithAPIOptions(awsmetrics.MetricsMiddleware()),
		config.WithAPIOptions(dynamometrics.MetricsMiddleware(dynamometrics.Backend)),
	}

	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dynamoResolver, err := endpoint.NewLoggingResolver(
		dynamodb.NewDefaultEndpointResolverV2(),
		l.With(slog.Group("service",
			"id", dynamodb.ServiceID,
			"api_version", dynamodb.ServiceAPIVersion,
		)),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dynamoOpts := []func(*dynamodb.Options){
		dynamodb.WithEndpointResolverV2(dynamoResolver),
		func(o *dynamodb.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		},
	}

	// FIPS settings are applied on the individual service instead of the aws config,
	// as DynamoDB Streams and Application Auto Scaling do not yet have FIPS endpoints in non-GovCloud.
	// See also: https://aws.amazon.com/compliance/fips/#FIPS_Endpoints_by_Service
	if modules.GetModules().IsBoringBinary() {
		dynamoOpts = append(dynamoOpts, func(o *dynamodb.Options) {
			o.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		})
	}

	dynamoClient := dynamodb.NewFromConfig(awsConfig, dynamoOpts...)

	streamsResolver, err := endpoint.NewLoggingResolver(
		dynamodbstreams.NewDefaultEndpointResolverV2(),
		l.With(slog.Group("service",
			"id", dynamodbstreams.ServiceID,
			"api_version", dynamodbstreams.ServiceAPIVersion,
		)),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	streamsClient := dynamodbstreams.NewFromConfig(awsConfig, dynamodbstreams.WithEndpointResolverV2(streamsResolver))
	b := &Backend{
		logger:  l,
		Config:  *cfg,
		clock:   clockwork.NewRealClock(),
		buf:     backend.NewCircularBuffer(backend.BufferCapacity(cfg.BufferSize)),
		svc:     dynamoClient,
		streams: streamsClient,
	}

	if err := b.configureTable(ctx, applicationautoscaling.NewFromConfig(awsConfig)); err != nil {
		return nil, trace.Wrap(err)
	}

	l.InfoContext(ctx, "Connection established to DynamoDB state database",
		"table", cfg.TableName,
		"region", cfg.Region,
	)

	go func() {
		if err := b.asyncPollStreams(ctx); err != nil {
			b.logger.ErrorContext(ctx, "Stream polling loop exited", "error", err)
		}
	}()

	return b, nil
}

func (b *Backend) configureTable(ctx context.Context, svc *applicationautoscaling.Client) error {
	tableName := aws.String(b.TableName)
	// check if the table exists?
	ts, tableBillingMode, err := b.getTableStatus(ctx, tableName)
	if err != nil {
		return trace.Wrap(err)
	}

	switch ts {
	case tableStatusOK:
		if tableBillingMode == types.BillingModePayPerRequest {
			b.Config.EnableAutoScaling = false
			b.logger.InfoContext(ctx, "Ignoring auto_scaling setting as table is in on-demand mode.")
		}
	case tableStatusMissing:
		if b.Config.BillingMode == billingModePayPerRequest {
			b.Config.EnableAutoScaling = false
			b.logger.InfoContext(ctx, "Ignoring auto_scaling setting as table is being created in on-demand mode.")
		}
		err = b.createTable(ctx, tableName, fullPathKey)
	case tableStatusNeedsMigration:
		return trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Enable TTL on table.
	ttlStatus, err := b.svc.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: tableName,
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	switch ttlStatus.TimeToLiveDescription.TimeToLiveStatus {
	case types.TimeToLiveStatusEnabled, types.TimeToLiveStatusEnabling:
	default:
		_, err = b.svc.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: tableName,
			TimeToLiveSpecification: &types.TimeToLiveSpecification{
				AttributeName: aws.String(ttlKey),
				Enabled:       aws.Bool(true),
			},
		})
		if err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	// Turn on DynamoDB streams, needed to implement events.
	tableStatus, err := b.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: tableName,
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}

	if tableStatus.Table.StreamSpecification == nil || (tableStatus.Table.StreamSpecification != nil && !aws.ToBool(tableStatus.Table.StreamSpecification.StreamEnabled)) {
		_, err = b.svc.UpdateTable(ctx, &dynamodb.UpdateTableInput{
			TableName: tableName,
			StreamSpecification: &types.StreamSpecification{
				StreamEnabled:  aws.Bool(true),
				StreamViewType: types.StreamViewTypeNewImage,
			},
		})
		if err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	// Enable continuous backups if requested.
	if b.Config.EnableContinuousBackups {
		// Make request to AWS to update continuous backups settings.
		_, err := b.svc.UpdateContinuousBackups(ctx, &dynamodb.UpdateContinuousBackupsInput{
			PointInTimeRecoverySpecification: &types.PointInTimeRecoverySpecification{
				PointInTimeRecoveryEnabled: aws.Bool(true),
			},
			TableName: tableName,
		})
		if err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	// Enable auto scaling if requested.
	if b.Config.EnableAutoScaling {
		readDimension := autoscalingtypes.ScalableDimensionDynamoDBTableReadCapacityUnits
		writeDimension := autoscalingtypes.ScalableDimensionDynamoDBTableWriteCapacityUnits
		resourceID := "table/" + b.TableName

		// Define scaling targets. Defines minimum and maximum {read,write} capacity.
		if _, err := svc.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
			MinCapacity:       aws.Int32(b.ReadMinCapacity),
			MaxCapacity:       aws.Int32(b.ReadMaxCapacity),
			ResourceId:        aws.String(resourceID),
			ScalableDimension: readDimension,
			ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
		}); err != nil {
			return trace.Wrap(convertError(err))
		}
		if _, err := svc.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
			MinCapacity:       aws.Int32(b.WriteMinCapacity),
			MaxCapacity:       aws.Int32(b.WriteMaxCapacity),
			ResourceId:        aws.String(resourceID),
			ScalableDimension: writeDimension,
			ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
		}); err != nil {
			return trace.Wrap(convertError(err))
		}

		// Define scaling policy. Defines the ratio of {read,write} consumed capacity to
		// provisioned capacity DynamoDB will try and maintain.
		readPolicy := b.TableName + "-read-target-tracking-scaling-policy"
		if _, err := svc.PutScalingPolicy(ctx, &applicationautoscaling.PutScalingPolicyInput{
			PolicyName:        aws.String(readPolicy),
			PolicyType:        autoscalingtypes.PolicyTypeTargetTrackingScaling,
			ResourceId:        aws.String(resourceID),
			ScalableDimension: readDimension,
			ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
			TargetTrackingScalingPolicyConfiguration: &autoscalingtypes.TargetTrackingScalingPolicyConfiguration{
				PredefinedMetricSpecification: &autoscalingtypes.PredefinedMetricSpecification{
					PredefinedMetricType: autoscalingtypes.MetricTypeDynamoDBReadCapacityUtilization,
				},
				TargetValue: aws.Float64(b.ReadTargetValue),
			},
		}); err != nil {
			return trace.Wrap(convertError(err))
		}

		writePolicy := b.TableName + "-write-target-tracking-scaling-policy"
		if _, err := svc.PutScalingPolicy(ctx, &applicationautoscaling.PutScalingPolicyInput{
			PolicyName:        aws.String(writePolicy),
			PolicyType:        autoscalingtypes.PolicyTypeTargetTrackingScaling,
			ResourceId:        aws.String(resourceID),
			ScalableDimension: writeDimension,
			ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
			TargetTrackingScalingPolicyConfiguration: &autoscalingtypes.TargetTrackingScalingPolicyConfiguration{
				PredefinedMetricSpecification: &autoscalingtypes.PredefinedMetricSpecification{
					PredefinedMetricType: autoscalingtypes.MetricTypeDynamoDBWriteCapacityUtilization,
				},
				TargetValue: aws.Float64(b.WriteTargetValue),
			},
		}); err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	return nil
}

func (b *Backend) GetName() string {
	return GetName()
}

// Create creates item if it does not exist
func (b *Backend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	rev, err := b.create(ctx, item, modeCreate)
	if trace.IsCompareFailed(err) {
		err = trace.AlreadyExists("%s", err)
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
		err = trace.NotFound("%s", err)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item.Revision = rev
	return backend.NewLease(item), nil
}

// GetRange returns range of elements
func (b *Backend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	if startKey.IsZero() {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
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

func (b *Backend) getAllRecords(ctx context.Context, startKey, endKey backend.Key, limit int) (*getResult, error) {
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
				b.logger.WarnContext(ctx, "Range query hit backend limit. (this is a bug!)", "start_key", startKey, "limit", backend.DefaultRangeLimit)
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
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	if startKey.IsZero() {
		return trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
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
		requests := make([]types.WriteRequest, 0, len(result.records))
		for _, record := range result.records {
			requests = append(requests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: map[string]types.AttributeValue{
						hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
						fullPathKey: &types.AttributeValueMemberS{Value: record.FullPath},
					},
				},
			})
		}
		input := dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				b.TableName: requests,
			},
		}

		if _, err = b.svc.BatchWriteItem(ctx, &input); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.ConnectionProblem(nil, "not all items deleted, too many requests")
}

// Get returns a single item or not found error
func (b *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	r, err := b.getKey(ctx, key)
	if err != nil {
		return nil, err
	}

	item := &backend.Item{
		Key:      trimPrefix(r.FullPath),
		Value:    r.Value,
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
	if expected.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if replaceWith.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if expected.Key.Compare(replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	replaceWith.Revision = backend.CreateRevision()
	r := record{
		HashKey:   hashKey,
		FullPath:  prependPrefix(replaceWith.Key),
		Value:     replaceWith.Value,
		Timestamp: time.Now().UTC().Unix(),
		Revision:  replaceWith.Revision,
	}
	if !replaceWith.Expires.IsZero() {
		r.Expires = aws.Int64(replaceWith.Expires.UTC().Unix())
	}
	av, err := attributevalue.MarshalMap(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(b.TableName),
		ConditionExpression: aws.String("#v = :prev"),
		ExpressionAttributeNames: map[string]string{
			"#v": "Value",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prev": &types.AttributeValueMemberB{Value: expected.Value},
		},
	}

	_, err = b.svc.PutItem(ctx, &input)
	err = convertError(err)
	if err != nil {
		// in this case let's use more specific compare failed error
		if trace.IsAlreadyExists(err) {
			return nil, trace.CompareFailed("%s", err)
		}
		return nil, trace.Wrap(err)
	}
	return backend.NewLease(replaceWith), nil
}

// Delete deletes item by key
func (b *Backend) Delete(ctx context.Context, key backend.Key) error {
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
func (b *Backend) ConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	if rev == "" {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	av, err := attributevalue.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: prependPrefix(key),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	input := dynamodb.DeleteItemInput{
		Key:       av,
		TableName: aws.String(b.TableName),
	}

	if rev == backend.BlankRevision {
		input.ConditionExpression = aws.String("attribute_not_exists(Revision) AND attribute_exists(FullPath)")
	} else {
		input.ExpressionAttributeValues = map[string]types.AttributeValue{":rev": &types.AttributeValueMemberS{Value: rev}}
		input.ConditionExpression = aws.String("Revision = :rev AND attribute_exists(FullPath)")
	}

	if _, err = b.svc.DeleteItem(ctx, &input); err != nil {
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
	if lease.Key.IsZero() {
		return trace.BadParameter("lease is missing key")
	}
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expires":   &types.AttributeValueMemberN{Value: strconv.FormatInt(expires.UTC().Unix(), 10)},
			":timestamp": &types.AttributeValueMemberN{Value: strconv.FormatInt(b.clock.Now().UTC().Unix(), 10)},
		},
		TableName: aws.String(b.TableName),
		Key: map[string]types.AttributeValue{
			hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
			fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(lease.Key)},
		},
		UpdateExpression:    aws.String("SET Expires = :expires"),
		ConditionExpression: aws.String("attribute_exists(FullPath) AND (attribute_not_exists(Expires) OR Expires >= :timestamp)"),
	}
	_, err := b.svc.UpdateItem(ctx, input)
	err = convertError(err)
	if trace.IsCompareFailed(err) {
		err = trace.NotFound("%s", err)
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
func (b *Backend) getTableStatus(ctx context.Context, tableName *string) (tableStatus, types.BillingMode, error) {
	td, err := b.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: tableName,
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
		return tableStatusOK, types.BillingModeProvisioned, nil
	}
	return tableStatusOK, td.Table.BillingModeSummary.BillingMode, nil
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
func (b *Backend) createTable(ctx context.Context, tableName *string, rangeKey string) error {
	billingMode := types.BillingModeProvisioned
	pThroughput := &types.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(b.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(b.WriteCapacityUnits),
	}
	if b.BillingMode == billingModePayPerRequest {
		billingMode = types.BillingModePayPerRequest
		pThroughput = nil
	}

	def := []types.AttributeDefinition{
		{
			AttributeName: aws.String(hashKeyKey),
			AttributeType: types.ScalarAttributeTypeS,
		},
		{
			AttributeName: aws.String(rangeKey),
			AttributeType: types.ScalarAttributeTypeS,
		},
	}
	elems := []types.KeySchemaElement{
		{
			AttributeName: aws.String(hashKeyKey),
			KeyType:       types.KeyTypeHash,
		},
		{
			AttributeName: aws.String(rangeKey),
			KeyType:       types.KeyTypeRange,
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             tableName,
		AttributeDefinitions:  def,
		KeySchema:             elems,
		ProvisionedThroughput: pThroughput,
		BillingMode:           billingMode,
	}
	_, err := b.svc.CreateTable(ctx, &c)
	if err != nil {
		return trace.Wrap(err)
	}
	b.logger.InfoContext(ctx, "Waiting until table is created.", "table", aws.ToString(tableName))
	waiter := dynamodb.NewTableExistsWaiter(b.svc)

	err = waiter.Wait(ctx,
		&dynamodb.DescribeTableInput{TableName: tableName},
		10*time.Minute,
	)
	if err == nil {
		b.logger.InfoContext(ctx, "Table has been created.", "table", aws.ToString(tableName))
	}

	return trace.Wrap(err)
}

type getResult struct {
	// lastEvaluatedKey is the primary key of the item where the operation stopped, inclusive of the
	// previous result set. Use this value to start a new operation, excluding this
	// value in the new request.
	lastEvaluatedKey map[string]types.AttributeValue
	records          []record
}

// getRecords retrieves all keys by path
func (b *Backend) getRecords(ctx context.Context, startKey, endKey string, limit int, lastEvaluatedKey map[string]types.AttributeValue) (*getResult, error) {
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
	av, err := attributevalue.MarshalMap(attrV)
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
		input.Limit = aws.Int32(int32(limit))
	}
	out, err := b.svc.Query(ctx, &input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result getResult
	for _, item := range out.Items {
		var r record
		if err := attributevalue.UnmarshalMap(item, &r); err != nil {
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
	var result []record

	for v := range elements {
		if !encountered[elements[v].FullPath] {
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
func prependPrefix(key backend.Key) string {
	return keyPrefix + key.String()
}

// trimPrefix removes leading 'teleport' from the key
func trimPrefix(key string) backend.Key {
	return backend.KeyFromString(key).TrimPrefix(backend.KeyFromString(keyPrefix))
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
		Revision:  backend.CreateRevision(),
	}
	if !item.Expires.IsZero() {
		r.Expires = aws.Int64(item.Expires.UTC().Unix())
	}
	av, err := attributevalue.MarshalMap(r)
	if err != nil {
		return "", trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.TableName),
	}

	switch mode {
	case modeCreate:
		input.ConditionExpression = aws.String("attribute_not_exists(FullPath)")
	case modeUpdate:
		input.ConditionExpression = aws.String("attribute_exists(FullPath)")
	case modePut:
	case modeConditionalUpdate:
		// If the revision is empty, then the resource existed prior to revision support. Instead of validating that
		// the revisions match, validate that the revision attribute does not exist. Otherwise, validate that the revision
		// attribute matches the item revision.
		if item.Revision == "" {
			input.ConditionExpression = aws.String("attribute_not_exists(Revision) AND attribute_exists(FullPath)")
		} else {
			input.ExpressionAttributeValues = map[string]types.AttributeValue{":rev": &types.AttributeValueMemberS{Value: item.Revision}}
			input.ConditionExpression = aws.String("Revision = :rev AND attribute_exists(FullPath)")
		}
	default:
		return "", trace.BadParameter("unrecognized mode")
	}
	_, err = b.svc.PutItem(ctx, &input)
	err = convertError(err)
	if err != nil {
		if mode == modeConditionalUpdate && trace.IsCompareFailed(err) {
			return "", trace.Wrap(backend.ErrIncorrectRevision)
		}

		return "", trace.Wrap(err)
	}

	return r.Revision, nil
}

func (b *Backend) deleteKey(ctx context.Context, key backend.Key) error {
	av, err := attributevalue.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: prependPrefix(key),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.DeleteItemInput{Key: av, TableName: aws.String(b.TableName)}
	if _, err = b.svc.DeleteItem(ctx, &input); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *Backend) deleteKeyIfExpired(ctx context.Context, key backend.Key) error {
	_, err := b.svc.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(b.TableName),
		Key:       keyToAttributeValueMap(key),

		// succeed if the item no longer exists
		ConditionExpression: aws.String(
			"attribute_not_exists(FullPath) OR (attribute_exists(Expires) AND Expires <= :timestamp)",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":timestamp": timeToAttributeValue(b.clock.Now()),
		},
	})
	return trace.Wrap(err)
}

func (b *Backend) getKey(ctx context.Context, key backend.Key) (*record, error) {
	av, err := attributevalue.MarshalMap(keyLookup{
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
	out, err := b.svc.GetItem(ctx, &input)
	if err != nil {
		// we deliberately use a "generic" trace error here, since we don't want
		// callers to make assumptions about the nature of the failure.
		return nil, trace.WrapWithMessage(err, "failed to get %q (dynamo error)", key.String())
	}
	if len(out.Item) == 0 {
		return nil, trace.NotFound("%q is not found", key.String())
	}
	var r record
	if err := attributevalue.UnmarshalMap(out.Item, &r); err != nil {
		return nil, trace.WrapWithMessage(err, "failed to unmarshal dynamo item %q", key.String())
	}
	// Check if key expired, if expired delete it
	if r.isExpired(b.clock.Now()) {
		if err := b.deleteKeyIfExpired(ctx, key); err != nil {
			b.logger.WarnContext(ctx, "Failed deleting expired key", "key", key, "error", err)
		}
		return nil, trace.NotFound("%q is not found", key.String())
	}
	return &r, nil
}

func convertError(err error) error {
	if err == nil {
		return nil
	}

	var conditionalCheckFailedError *types.ConditionalCheckFailedException
	if errors.As(err, &conditionalCheckFailedError) {
		return trace.CompareFailed("%s", conditionalCheckFailedError.ErrorMessage())
	}

	var throughputExceededError *types.ProvisionedThroughputExceededException
	if errors.As(err, &throughputExceededError) {
		return trace.ConnectionProblem(throughputExceededError, "%s", throughputExceededError.ErrorMessage())
	}

	var notFoundError *types.ResourceNotFoundException
	if errors.As(err, &notFoundError) {
		return trace.NotFound("%s", notFoundError.ErrorMessage())
	}

	var collectionLimitExceededError *types.ItemCollectionSizeLimitExceededException
	if errors.As(err, &notFoundError) {
		return trace.BadParameter("%s", collectionLimitExceededError.ErrorMessage())
	}

	var internalError *types.InternalServerError
	if errors.As(err, &internalError) {
		return trace.BadParameter("%s", internalError.ErrorMessage())
	}

	var expiredIteratorError *streamtypes.ExpiredIteratorException
	if errors.As(err, &expiredIteratorError) {
		return trace.ConnectionProblem(expiredIteratorError, "%s", expiredIteratorError.ErrorMessage())
	}

	var limitExceededError *streamtypes.LimitExceededException
	if errors.As(err, &limitExceededError) {
		return trace.ConnectionProblem(limitExceededError, "%s", limitExceededError.ErrorMessage())
	}
	var trimmedAccessError *streamtypes.TrimmedDataAccessException
	if errors.As(err, &trimmedAccessError) {
		return trace.ConnectionProblem(trimmedAccessError, "%s", trimmedAccessError.ErrorMessage())
	}

	var scalingObjectNotFoundError *autoscalingtypes.ObjectNotFoundException
	if errors.As(err, &scalingObjectNotFoundError) {
		return trace.NotFound("%s", scalingObjectNotFoundError.ErrorMessage())
	}

	return err
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

func fullPathToAttributeValueMap(fullPath string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
		fullPathKey: &types.AttributeValueMemberS{Value: fullPath},
	}
}

func keyToAttributeValueMap(key backend.Key) map[string]types.AttributeValue {
	return fullPathToAttributeValueMap(prependPrefix(key))
}

func timeToAttributeValue(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{
		Value: strconv.FormatInt(t.Unix(), 10),
	}
}
