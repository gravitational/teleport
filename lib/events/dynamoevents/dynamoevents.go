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

package dynamoevents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	awsmetrics "github.com/gravitational/teleport/lib/observability/metrics/aws"
	dynamometrics "github.com/gravitational/teleport/lib/observability/metrics/dynamo"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws/dynamodbutils"
	"github.com/gravitational/teleport/lib/utils/aws/endpoint"
)

const (
	// iso8601DateFormat is the time format used by the date attribute on events.
	iso8601DateFormat = "2006-01-02"

	// ErrValidationException for service response error code
	// "ValidationException".
	//
	//  Indicates about invalid item for example max DynamoDB item length was exceeded.
	ErrValidationException = "ValidationException"

	// maxItemSize is the maximum size of a DynamoDB item.
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ServiceQuotas.html
	maxItemSize = 400 * 1024 // 400KB
)

// Defines the attribute schema for the DynamoDB event table and index.
var tableSchema = []dynamodbtypes.AttributeDefinition{
	// Existing attributes pre RFD 24.
	{
		AttributeName: aws.String(keySessionID),
		AttributeType: dynamodbtypes.ScalarAttributeTypeS,
	},
	{
		AttributeName: aws.String(keyEventIndex),
		AttributeType: dynamodbtypes.ScalarAttributeTypeN,
	},
	{
		AttributeName: aws.String(keyCreatedAt),
		AttributeType: dynamodbtypes.ScalarAttributeTypeN,
	},
	// New attribute in RFD 24.
	{
		AttributeName: aws.String(keyDate),
		AttributeType: dynamodbtypes.ScalarAttributeTypeS,
	},
}

// Config structure represents DynamoDB configuration as appears in `storage` section
// of Teleport YAML
type Config struct {
	// Region is where DynamoDB Table will be used to store k/v
	Region string
	// Tablename where to store K/V in DynamoDB
	Tablename string
	// ReadCapacityUnits is Dynamodb read capacity units
	ReadCapacityUnits int64
	// WriteCapacityUnits is Dynamodb write capacity units
	WriteCapacityUnits int64
	// RetentionPeriod is a default retention period for events.
	RetentionPeriod *types.Duration
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator
	UIDGenerator utils.UID
	// Endpoint is an optional non-AWS endpoint
	Endpoint string
	// DisableConflictCheck disables conflict checks when emitting an event.
	// Disabling it can cause events to be lost due to them being overwritten.
	DisableConflictCheck bool

	// ReadMaxCapacity is the maximum provisioned read capacity.
	ReadMaxCapacity int32
	// ReadMinCapacity is the minimum provisioned read capacity.
	ReadMinCapacity int32
	// ReadTargetValue is the ratio of consumed read to provisioned capacity.
	ReadTargetValue float64
	// WriteMaxCapacity is the maximum provisioned write capacity.
	WriteMaxCapacity int32
	// WriteMinCapacity is the minimum provisioned write capacity.
	WriteMinCapacity int32
	// WriteTargetValue is the ratio of consumed write to provisioned capacity.
	WriteTargetValue float64

	// UseFIPSEndpoint uses AWS FedRAMP/FIPS 140-2 mode endpoints.
	// to determine its behavior:
	// Unset - allows environment variables or AWS config to set the value
	// Enabled - explicitly enabled
	// Disabled - explicitly disabled
	UseFIPSEndpoint types.ClusterAuditConfigSpecV2_FIPSEndpointState

	// EnableContinuousBackups is used to enable PITR (Point-In-Time Recovery).
	EnableContinuousBackups bool

	// EnableAutoScaling is used to enable auto scaling policy.
	EnableAutoScaling bool

	// CredentialsProvider if supplied is used to override the credentials source.
	CredentialsProvider aws.CredentialsProvider

	// Insecure is an optional switch to opt out of https connections
	Insecure bool
}

// SetFromURL sets values on the Config from the supplied URI
func (cfg *Config) SetFromURL(in *url.URL) error {
	if endpoint := in.Query().Get(teleport.Endpoint); endpoint != "" {
		cfg.Endpoint = endpoint
	}

	if disableConflictCheck := in.Query().Get("disable_conflict_check"); disableConflictCheck != "" {
		cfg.DisableConflictCheck = true
	}

	const boolErrorTemplate = "failed to parse URI %q flag %q - %q, supported values are 'true', 'false', or any other" +
		"supported boolean in https://pkg.go.dev/strconv#ParseBool"
	if val := in.Query().Get(events.UseFIPSQueryParam); val != "" {
		useFips, err := strconv.ParseBool(val)
		if err != nil {
			return trace.BadParameter(boolErrorTemplate, in.String(), events.UseFIPSQueryParam, val)
		}
		if useFips {
			cfg.UseFIPSEndpoint = types.ClusterAuditConfigSpecV2_FIPS_ENABLED
		} else {
			cfg.UseFIPSEndpoint = types.ClusterAuditConfigSpecV2_FIPS_DISABLED
		}
	}

	return nil
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to DynamoDB
func (cfg *Config) CheckAndSetDefaults() error {
	// Table name is required.
	if cfg.Tablename == "" {
		return trace.BadParameter("DynamoDB: table_name is not specified")
	}

	if cfg.ReadCapacityUnits == 0 {
		cfg.ReadCapacityUnits = DefaultReadCapacityUnits
	}
	if cfg.WriteCapacityUnits == 0 {
		cfg.WriteCapacityUnits = DefaultWriteCapacityUnits
	}
	if cfg.RetentionPeriod == nil || cfg.RetentionPeriod.Duration() == 0 {
		duration := DefaultRetentionPeriod
		cfg.RetentionPeriod = &duration
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UIDGenerator == nil {
		cfg.UIDGenerator = utils.NewRealUID()
	}

	if cfg.Endpoint != "" {
		cfg.Endpoint = endpoint.CreateURI(cfg.Endpoint, cfg.Insecure)
	}

	return nil
}

// Log is a dynamo-db backed storage of events
type Log struct {
	// logger is emits log messages
	logger *slog.Logger
	// Config is a backend configuration
	Config
	svc *dynamodb.Client
}

type event struct {
	SessionID      string
	EventIndex     int64
	EventType      string
	CreatedAt      int64
	Expires        *int64 `json:"Expires,omitempty" dynamodbav:",omitempty"`
	FieldsMap      events.EventFields
	EventNamespace string
	CreatedAtDate  string
}

const (
	// keyExpires is a key used for TTL specification
	keyExpires = "Expires"

	// keySessionID is event SessionID dynamodb key
	keySessionID = "SessionID"

	// keyEventIndex is EventIndex key
	keyEventIndex = "EventIndex"

	// keyCreatedAt identifies created at key
	keyCreatedAt = "CreatedAt"

	// keyDate identifies the date the event was created at in UTC.
	// The date takes the format `yyyy-mm-dd` as a string.
	// Specified in RFD 24.
	keyDate = "CreatedAtDate"

	// indexTimeSearchV2 is the new secondary global index proposed in RFD 24.
	// Allows searching events by time.
	indexTimeSearchV2 = "timesearchV2"

	// DefaultReadCapacityUnits specifies default value for read capacity units
	DefaultReadCapacityUnits = 10

	// DefaultWriteCapacityUnits specifies default value for write capacity units
	DefaultWriteCapacityUnits = 10

	// DefaultRetentionPeriod is a default data retention period in events table.
	// The default is 1 year.
	DefaultRetentionPeriod = types.Duration(365 * 24 * time.Hour)
)

// New returns new instance of DynamoDB backend.
// It's an implementation of backend API's NewFunc
func New(ctx context.Context, cfg Config) (*Log, error) {
	l := slog.With(teleport.ComponentKey, teleport.ComponentDynamoDB)
	l.InfoContext(ctx, "Initializing event backend")

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	if cfg.CredentialsProvider != nil {
		opts = append(opts, config.WithCredentialsProvider(cfg.CredentialsProvider))
	}

	resolver, err := endpoint.NewLoggingResolver(
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
		dynamodb.WithEndpointResolverV2(resolver),
		func(o *dynamodb.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		},
	}

	// Override the service endpoint using the "endpoint" query parameter from
	// "audit_events_uri". This is for non-AWS DynamoDB-compatible backends.
	if cfg.Endpoint != "" {
		if _, err := url.Parse(cfg.Endpoint); err != nil {
			return nil, trace.BadParameter("configured DynamoDB events endpoint is invalid: %s", err.Error())
		}

		opts = append(opts, config.WithBaseEndpoint(cfg.Endpoint))
	}

	// FIPS settings are applied on the individual service instead of the aws config,
	// as DynamoDB Streams and Application Auto Scaling do not yet have FIPS endpoints in non-GovCloud.
	// See also: https://aws.amazon.com/compliance/fips/#FIPS_Endpoints_by_Service
	if dynamodbutils.IsFIPSEnabled() &&
		cfg.UseFIPSEndpoint == types.ClusterAuditConfigSpecV2_FIPS_ENABLED {
		dynamoOpts = append(dynamoOpts, func(o *dynamodb.Options) {
			o.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		})
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := dynamodb.NewFromConfig(awsConfig, dynamoOpts...)
	b := &Log{
		logger: l,
		Config: cfg,
		svc:    client,
	}

	if err := b.configureTable(ctx, applicationautoscaling.NewFromConfig(awsConfig)); err != nil {
		return nil, trace.Wrap(err)
	}

	l.InfoContext(ctx, "Connection established to DynamoDB events database",
		"table", cfg.Tablename,
		"region", cfg.Region,
	)

	return b, nil
}

type tableStatus int

const (
	tableStatusError = iota
	tableStatusMissing
	tableStatusNeedsMigration
	tableStatusOK
)

func (l *Log) configureTable(ctx context.Context, svc *applicationautoscaling.Client) error {
	// check if the table exists?
	ts, err := l.getTableStatus(ctx, l.Tablename)
	if err != nil {
		return trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		break
	case tableStatusMissing:
		err = l.createTable(ctx, l.Tablename)
	case tableStatusNeedsMigration:
		return trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return trace.Wrap(err)
	}
	tableName := aws.String(l.Tablename)
	ttlStatus, err := l.svc.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: tableName,
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	switch ttlStatus.TimeToLiveDescription.TimeToLiveStatus {
	case dynamodbtypes.TimeToLiveStatusEnabled, dynamodbtypes.TimeToLiveStatusEnabling:
	default:
		_, err = l.svc.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: tableName,
			TimeToLiveSpecification: &dynamodbtypes.TimeToLiveSpecification{
				AttributeName: aws.String(keyExpires),
				Enabled:       aws.Bool(true),
			},
		})
		if err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	// Enable continuous backups if requested.
	if l.Config.EnableContinuousBackups {
		// Make request to AWS to update continuous backups settings.
		_, err := l.svc.UpdateContinuousBackups(ctx, &dynamodb.UpdateContinuousBackupsInput{
			PointInTimeRecoverySpecification: &dynamodbtypes.PointInTimeRecoverySpecification{
				PointInTimeRecoveryEnabled: aws.Bool(true),
			},
			TableName: tableName,
		})
		if err != nil {
			return trace.Wrap(convertError(err))
		}
	}

	// Enable auto scaling if requested.
	if l.Config.EnableAutoScaling {
		type autoscalingParams struct {
			readDimension  autoscalingtypes.ScalableDimension
			writeDimension autoscalingtypes.ScalableDimension
			resourceID     string
			readPolicy     string
			writePolicy    string
		}

		params := []autoscalingParams{
			{
				readDimension:  autoscalingtypes.ScalableDimensionDynamoDBTableReadCapacityUnits,
				writeDimension: autoscalingtypes.ScalableDimensionDynamoDBTableWriteCapacityUnits,
				resourceID:     fmt.Sprintf("table/%s", l.Tablename),
				readPolicy:     fmt.Sprintf("%s-read-target-tracking-scaling-policy", l.Tablename),
				writePolicy:    fmt.Sprintf("%s-write-target-tracking-scaling-policy", l.Tablename),
			},
			{
				readDimension:  autoscalingtypes.ScalableDimensionDynamoDBIndexReadCapacityUnits,
				writeDimension: autoscalingtypes.ScalableDimensionDynamoDBIndexWriteCapacityUnits,
				resourceID:     fmt.Sprintf("table/%s/index/%s", l.Tablename, indexTimeSearchV2),
				readPolicy:     fmt.Sprintf("%s/index/%s-read-target-tracking-scaling-policy", l.Tablename, indexTimeSearchV2),
				writePolicy:    fmt.Sprintf("%s/index/%s-write-target-tracking-scaling-policy", l.Tablename, indexTimeSearchV2),
			},
		}

		for _, p := range params {
			// Define scaling targets. Defines minimum and maximum {read,write} capacity.
			if _, err := svc.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
				MinCapacity:       aws.Int32(l.ReadMinCapacity),
				MaxCapacity:       aws.Int32(l.ReadMaxCapacity),
				ResourceId:        aws.String(p.resourceID),
				ScalableDimension: p.readDimension,
				ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
			}); err != nil {
				return trace.Wrap(convertError(err))
			}
			if _, err := svc.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
				MinCapacity:       aws.Int32(l.WriteMinCapacity),
				MaxCapacity:       aws.Int32(l.WriteMaxCapacity),
				ResourceId:        aws.String(p.resourceID),
				ScalableDimension: p.writeDimension,
				ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
			}); err != nil {
				return trace.Wrap(convertError(err))
			}

			// Define scaling policy. Defines the ratio of {read,write} consumed capacity to
			// provisioned capacity DynamoDB will try and maintain.
			for i := 0; i < 2; i++ {
				if _, err := svc.PutScalingPolicy(ctx, &applicationautoscaling.PutScalingPolicyInput{
					PolicyName:        aws.String(p.readPolicy),
					PolicyType:        autoscalingtypes.PolicyTypeTargetTrackingScaling,
					ResourceId:        aws.String(p.resourceID),
					ScalableDimension: p.readDimension,
					ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
					TargetTrackingScalingPolicyConfiguration: &autoscalingtypes.TargetTrackingScalingPolicyConfiguration{
						PredefinedMetricSpecification: &autoscalingtypes.PredefinedMetricSpecification{
							PredefinedMetricType: autoscalingtypes.MetricTypeDynamoDBReadCapacityUtilization,
						},
						TargetValue: aws.Float64(l.ReadTargetValue),
					},
				}); err != nil {
					// The read policy name was accidentally changed to match the write policy in 17.0.0-17.1.4. This
					// prevented upgrading a cluster with autoscaling enabled from v16 to v17. To resolve in
					// a backwards compatible way, the read policy name was restored, however, any new clusters that
					// were created between 17.0.0 and 17.1.4 need to have the misnamed policy deleted and recreated
					// with the correct name.
					if i == 1 || !strings.Contains(err.Error(), "ValidationException: Only one TargetTrackingScaling policy for a given metric specification is allowed.") {
						return trace.Wrap(convertError(err))
					}

					l.logger.DebugContext(ctx, "Fixing incorrectly named scaling policy")
					if _, err := svc.DeleteScalingPolicy(ctx, &applicationautoscaling.DeleteScalingPolicyInput{
						PolicyName:        aws.String(p.writePolicy),
						ResourceId:        aws.String(p.resourceID),
						ScalableDimension: p.readDimension,
						ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
					}); err != nil {
						return trace.Wrap(convertError(err))
					}
				}
			}

			if _, err := svc.PutScalingPolicy(ctx, &applicationautoscaling.PutScalingPolicyInput{
				PolicyName:        aws.String(p.writePolicy),
				PolicyType:        autoscalingtypes.PolicyTypeTargetTrackingScaling,
				ResourceId:        aws.String(p.resourceID),
				ScalableDimension: p.writeDimension,
				ServiceNamespace:  autoscalingtypes.ServiceNamespaceDynamodb,
				TargetTrackingScalingPolicyConfiguration: &autoscalingtypes.TargetTrackingScalingPolicyConfiguration{
					PredefinedMetricSpecification: &autoscalingtypes.PredefinedMetricSpecification{
						PredefinedMetricType: autoscalingtypes.MetricTypeDynamoDBWriteCapacityUtilization,
					},
					TargetValue: aws.Float64(l.WriteTargetValue),
				},
			}); err != nil {
				return trace.Wrap(convertError(err))
			}
		}
	}

	return nil
}

// EmitAuditEvent emits audit event
func (l *Log) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	sessionID := getSessionID(in)
	return trace.Wrap(l.putAuditEvent(ctx, sessionID, in))
}

func (l *Log) handleAWSValidationError(ctx context.Context, err error, sessionID string, in apievents.AuditEvent) error {
	if alreadyTrimmed := ctx.Value(largeEventHandledContextKey); alreadyTrimmed != nil {
		return err
	}

	se, ok := trimEventSize(in)
	if !ok {
		return trace.BadParameter("%s", err)
	}
	if err := l.putAuditEvent(context.WithValue(ctx, largeEventHandledContextKey, true), sessionID, se); err != nil {
		return trace.BadParameter("%s", err)
	}
	l.logger.InfoContext(ctx, "Uploaded trimmed event to DynamoDB backend.", "event_id", in.GetID(), "event_type", in.GetType())
	events.MetricStoredTrimmedEvents.Inc()
	return nil
}

func (l *Log) handleConditionError(ctx context.Context, err error, sessionID string, in apievents.AuditEvent) error {
	if alreadyUpdated := ctx.Value(conflictHandledContextKey); alreadyUpdated != nil {
		return err
	}

	// Update index using the current system time instead of event time to
	// ensure the value is always set.
	in.SetIndex(l.Clock.Now().UnixNano())

	if err := l.putAuditEvent(context.WithValue(ctx, conflictHandledContextKey, true), sessionID, in); err != nil {
		return trace.Wrap(err)
	}
	l.logger.InfoContext(ctx, "Event index overwritten.", "event_id", in.GetID(), "event_type", in.GetType())
	return nil
}

// getSessionID if set returns event ID obtained from metadata or generates a new one.
func getSessionID(in apievents.AuditEvent) string {
	s, ok := in.(events.SessionMetadataGetter)
	if ok && s.GetSessionID() != "" {
		return s.GetSessionID()
	}
	// no session id - global event gets a random uuid to get a good partition
	// key distribution
	return uuid.New().String()
}

func isAWSValidationError(err error) bool {
	return errors.Is(trace.Unwrap(err), errAWSValidation)
}

func trimEventSize(event apievents.AuditEvent) (apievents.AuditEvent, bool) {
	trimmedEvent := event.TrimToMaxSize(maxItemSize)
	if trimmedEvent.Size() >= maxItemSize {
		return trimmedEvent, false
	}
	return trimmedEvent, true
}

// putAuditEventContextKey represents context keys of putAuditEvent.
type putAuditEventContextKey int

const (
	// conflictHandledContextKey if present on the context, the conflict error
	// was already handled.
	conflictHandledContextKey putAuditEventContextKey = iota
	// largeEventHandledContextKey if present on the context, the large event
	// error was already handled.
	largeEventHandledContextKey
)

func (l *Log) putAuditEvent(ctx context.Context, sessionID string, in apievents.AuditEvent) error {
	input, err := l.createPutItem(sessionID, in)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = l.svc.PutItem(ctx, input); err != nil {
		err = convertError(err)

		switch {
		case isAWSValidationError(err):
			// In case of ValidationException: Item size has exceeded the maximum allowed size
			// sanitize event length and retry upload operation.
			return trace.Wrap(l.handleAWSValidationError(ctx, err, sessionID, in))
		case trace.IsAlreadyExists(err):
			// Condition errors are directly related to the uniqueness of the
			// item event index/session id. Since we can't change the session
			// id, update the event index with a new value and retry the put
			// item.
			if err2 := l.handleConditionError(ctx, err, sessionID, in); err2 != nil {
				// Only log about the original conflict if updating
				// the session information fails.
				l.logger.ErrorContext(ctx, "Conflict on event session_id and event_index",
					"error", err,
					"event_type", in.GetType(),
					"session_id", sessionID,
					"event_index", in.GetIndex())
				return trace.Wrap(err2)
			}

			return nil
		}

		return err
	}

	return nil
}

func (l *Log) createPutItem(sessionID string, in apievents.AuditEvent) (*dynamodb.PutItemInput, error) {
	fieldsMap, err := events.ToEventFields(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e := event{
		SessionID:      sessionID,
		EventIndex:     in.GetIndex(),
		EventType:      in.GetType(),
		EventNamespace: apidefaults.Namespace,
		CreatedAt:      in.GetTime().Unix(),
		FieldsMap:      fieldsMap,
		CreatedAtDate:  in.GetTime().Format(iso8601DateFormat),
	}
	l.setExpiry(&e)
	av, err := attributevalue.MarshalMap(e)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(l.Tablename),
	}

	if !l.Config.DisableConflictCheck {
		input.ConditionExpression = aws.String("attribute_not_exists(SessionID) AND attribute_not_exists(EventIndex)")
	}

	return input, nil
}

func (l *Log) setExpiry(e *event) {
	if l.RetentionPeriod.Value() == 0 {
		return
	}

	e.Expires = aws.Int64(l.Clock.Now().UTC().Add(l.RetentionPeriod.Value()).Unix())
}

func daysSinceEpoch(timestamp time.Time) int64 {
	return timestamp.Unix() / (60 * 60 * 24)
}

// daysBetween returns a list of all dates between `start` and `end` in the format `yyyy-mm-dd`.
func daysBetween(start, end time.Time) []string {
	var days []string
	oneDay := time.Hour * time.Duration(24)
	startDay := daysSinceEpoch(start)
	endDay := daysSinceEpoch(end)

	for startDay <= endDay {
		days = append(days, start.Format(iso8601DateFormat))
		startDay++
		start = start.Add(oneDay)
	}

	return days
}

type checkpointKey struct {
	// The date that the Dynamo iterator corresponds to.
	Date string `json:"date,omitempty"`

	// A DynamoDB query iterator. Allows us to resume a partial query.
	Iterator string `json:"iterator,omitempty"`

	// EventKey is a derived identifier for an event used for resuming
	// sub-page breaks due to size constraints.
	EventKey string `json:"event_key,omitempty"`
}

// legacyCheckpointKey is the old checkpoint key returned by older auth versions. Used to decode
// checkpoints originating from old auths. Commonly we don't bother supporting pagination/cursors
// across teleport versions since the benefit of doing so is usually minimal, but this value is used
// as on-disk state by long running event export operations, and so must be supported.
//
// DELETE IN: 19.0.0
type legacyCheckpointKey struct {
	// The date that the Dynamo iterator corresponds to.
	Date string `json:"date,omitempty"`

	// A DynamoDB query iterator. Allows us to resume a partial query.
	Iterator map[string]*LegacyAttributeValue `json:"iterator,omitempty"`

	// EventKey is a derived identifier for an event used for resuming
	// sub-page breaks due to size constraints.
	EventKey string `json:"event_key,omitempty"`
}

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC).
//
// This function may never return more than 1 MiB of event data.
func (l *Log) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	values, next, err := l.searchEventsWithFilter(ctx, req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, searchEventsFilter{eventTypes: req.EventTypes}, "")
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	evts, err := events.FromEventFieldsSlice(values)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return evts, next, nil
}

func (l *Log) SearchUnstructuredEvents(ctx context.Context, req events.SearchEventsRequest) ([]*auditlogpb.EventUnstructured, string, error) {
	values, next, err := l.searchEventsWithFilter(ctx, req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, searchEventsFilter{eventTypes: req.EventTypes}, "")
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	evts, err := events.FromEventFieldsSliceToUnstructured(values)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return evts, next, nil
}

func (l *Log) searchEventsWithFilter(ctx context.Context, fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startKey string, filter searchEventsFilter, sessionID string) ([]events.EventFields, string, error) {
	rawEvents, lastKey, err := l.searchEventsRaw(ctx, fromUTC, toUTC, namespace, limit, order, startKey, filter, sessionID)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	eventArr := make([]events.EventFields, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		eventArr = append(eventArr, rawEvent.FieldsMap)
	}

	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = events.ByTimeAndIndex(eventArr)
	case types.EventOrderDescending:
		toSort = sort.Reverse(events.ByTimeAndIndex(eventArr))
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}

	sort.Sort(toSort)
	return eventArr, lastKey, nil
}

func (l *Log) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotImplemented("dynamoevents backend does not support streaming export"))
}

func (l *Log) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	return stream.Fail[*auditlogpb.EventExportChunk](trace.NotImplemented("dynamoevents backend does not support streaming export"))
}

// eventFilterList constructs a string of the form
// "(:eventTypeN, :eventTypeN, ...)" where N is a succession of integers
// starting from 0. The substrings :eventTypeN are automatically generated
// variable names that are valid with in the DynamoDB query language.
// The function generates a list of amount of these :eventTypeN variables that is a valid
// list literal in the DynamoDB query language. In order for this list to work the request
// needs to be supplied with the variable values for the event types you wish to fill the list with.
//
// The reason that this doesn't fill in the values as literals within the list is to prevent injection attacks.
func eventFilterList(amount int) string {
	var eventTypes []string
	for i := 0; i < amount; i++ {
		eventTypes = append(eventTypes, fmt.Sprintf(":eventType%d", i))
	}
	return "(" + strings.Join(eventTypes, ", ") + ")"
}

func reverseStrings(slice []string) []string {
	newSlice := make([]string, 0, len(slice))
	for i := len(slice) - 1; i >= 0; i-- {
		newSlice = append(newSlice, slice[i])
	}
	return newSlice
}

// searchEventsRaw is a low level function for searching for events. This is kept
// separate from the SearchEvents function in order to allow tests to grab more metadata.
func (l *Log) searchEventsRaw(ctx context.Context, fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startKey string, filter searchEventsFilter, sessionID string) ([]event, string, error) {
	if fromUTC.After(toUTC) {
		return nil, "", trace.BadParameter("from date is after to date")
	}
	checkpoint, err := getCheckpointFromStartKey(startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	l.logger.DebugContext(ctx, "search events", "from", fromUTC, "to", toUTC, "filter", filter, "limit", limit, "start_key", startKey, "order", order, "checkpoint", checkpoint)

	if startKey != "" {
		createdAt, err := GetCreatedAtFromStartKey(startKey)
		if err == nil {
			// we compare the cursor unix time to the from unix in order to drop the nanoseconds
			// that are not present in the cursor.
			if fromUTC.Unix() > createdAt.Unix() {
				l.logger.WarnContext(ctx, "cursor is from before window start time, resetting cursor", "created_at", createdAt, "from", fromUTC)
				// if fromUTC is after than the cursor, we changed the window and need to reset the cursor.
				// This is a guard check when iterating over the events using sliding window
				// and the previous cursor no longer fits the new window.
				checkpoint = checkpointKey{}
			}
			if createdAt.After(toUTC) {
				l.logger.DebugContext(ctx, "cursor is after the end of the window, skipping search", "created_at", createdAt, "to", toUTC)
				// if the cursor is after the end of the window, we can return early since we
				// won't find any events.
				return nil, "", nil
			}
		} else {
			l.logger.WarnContext(ctx, "failed to get creation time from start key", "start_key", startKey, "error", err)
		}
	}

	totalSize := 0
	dates := daysBetween(fromUTC, toUTC)
	if order == types.EventOrderDescending {
		dates = reverseStrings(dates)
	}

	indexName := aws.String(indexTimeSearchV2)
	var left int32
	if limit != 0 {
		left = int32(limit)
	} else {
		left = math.MaxInt32
	}

	// Resume scanning at the correct date. We need to do this because we send individual queries per date
	// and you can't resume a query with the wrong iterator checkpoint.
	//
	// We need to perform a guard check on the length of `dates` here in case a query is submitted with
	// `toUTC` occurring before `fromUTC`.
	if checkpoint.Date != "" && len(dates) > 0 {
		for len(dates) > 0 && dates[0] != checkpoint.Date {
			dates = dates[1:]
		}
		// if the initial data wasn't found in [fromUTC,toUTC]
		// dates will be empty and we can return early since we
		// won't find any events.
		if len(dates) == 0 {
			return nil, "", nil
		}
	}

	foundStart := checkpoint.EventKey == ""

	var forward bool
	switch order {
	case types.EventOrderAscending:
		forward = true
	case types.EventOrderDescending:
		forward = false
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}

	logger := l.logger.With(
		"from", fromUTC,
		"to", toUTC,
		"namespace", namespace,
		"filter", filter,
		"limit", limit,
		"start_key", startKey,
		"order", order,
	)

	ef := eventsFetcher{
		log:        logger,
		totalSize:  totalSize,
		checkpoint: &checkpoint,
		foundStart: foundStart,
		dates:      dates,
		left:       left,
		fromUTC:    fromUTC,
		toUTC:      toUTC,
		tableName:  l.Tablename,
		api:        l.svc,
		forward:    forward,
		indexName:  indexName,
		filter:     filter,
	}

	filterExpr := getExprFilter(filter)

	var values []event
	if fromUTC.IsZero() && sessionID != "" {
		values, err = ef.QueryBySessionIDIndex(ctx, sessionID, filterExpr)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	} else {
		values, err = ef.QueryByDateIndex(ctx, filterExpr)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var lastKey []byte
	if ef.hasLeft {
		lastKey, err = json.Marshal(&checkpoint)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return values, string(lastKey), nil
}

func GetCreatedAtFromStartKey(startKey string) (time.Time, error) {
	checkpoint, err := getCheckpointFromStartKey(startKey)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	if checkpoint.Iterator == "" {
		return time.Time{}, errors.New("missing iterator")
	}
	var e event
	if err := json.Unmarshal([]byte(checkpoint.Iterator), &e); err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	if e.CreatedAt <= 0 {
		// Value <= 0 means that either createdAt was not returned or
		// it has 0 values, either way, we can't use that value.
		return time.Time{}, errors.New("createdAt is invalid")
	}

	return time.Unix(e.CreatedAt, 0).UTC(), nil
}

func getCheckpointFromStartKey(startKey string) (checkpointKey, error) {
	var checkpoint checkpointKey
	if startKey == "" {
		return checkpoint, nil
	}
	// If a checkpoint key is provided, unmarshal it so we can work with its parts.
	if err := json.Unmarshal([]byte(startKey), &checkpoint); err != nil {
		// attempt to decode as legacy format.
		if checkpoint, err = getCheckpointFromLegacyStartKey(startKey); err == nil {
			return checkpoint, nil
		}
		return checkpointKey{}, trace.Wrap(err)
	}
	return checkpoint, nil
}

// getCheckpointFromLegacyStartKey is a helper function that decodes a legacy checkpoint key
// into the new format. The old format used raw dynamo attribute values for the iterator, where
// the new format uses a json-serialized map with bare values.
//
// DELETE IN: 19.0.0
func getCheckpointFromLegacyStartKey(startKey string) (checkpointKey, error) {
	var checkpoint legacyCheckpointKey
	if startKey == "" {
		return checkpointKey{}, nil
	}
	// If a checkpoint key is provided, unmarshal it so we can work with its parts.
	if err := json.Unmarshal([]byte(startKey), &checkpoint); err != nil {
		return checkpointKey{}, trace.Wrap(err)
	}

	convertedAttrMap, err := convertLegacyAttributesMap(checkpoint.Iterator)
	if err != nil {
		return checkpointKey{}, trace.Wrap(err)
	}

	// decode the dynamo attrs into the go map repr common to the old and new formats.
	m := make(map[string]any)
	if err := attributevalue.UnmarshalMap(convertedAttrMap, &m); err != nil {
		return checkpointKey{}, trace.Wrap(err)
	}

	// encode the map into json, making it equivalent to the new format.
	iterator, err := json.Marshal(m)
	if err != nil {
		return checkpointKey{}, trace.Wrap(err)
	}

	return checkpointKey{
		Date:     checkpoint.Date,
		Iterator: string(iterator),
		EventKey: checkpoint.EventKey,
	}, nil
}

func getExprFilter(filter searchEventsFilter) *string {
	var filterConds []string
	if len(filter.eventTypes) > 0 {
		typeList := eventFilterList(len(filter.eventTypes))
		filterConds = append(filterConds, fmt.Sprintf("EventType IN %s", typeList))
	}
	if filter.condExpr != "" {
		filterConds = append(filterConds, filter.condExpr)
	}
	var filterExpr *string
	if len(filterConds) > 0 {
		filterExpr = aws.String(strings.Join(filterConds, " AND "))
	}
	return filterExpr
}

func getSubPageCheckpoint(e *event) (string, error) {
	data, err := utils.FastMarshal(e)
	if err != nil {
		return "", trace.Wrap(err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// SearchSessionEvents returns session related events only. This is used to
// find completed session.
func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	filter := searchEventsFilter{eventTypes: events.SessionRecordingEvents}
	if req.Cond != nil {
		params := condFilterParams{attrValues: make(map[string]interface{}), attrNames: make(map[string]string)}
		expr, err := fromWhereExpr(req.Cond, &params)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condExpr = expr
		filter.condParams = params
	}
	values, next, err := l.searchEventsWithFilter(ctx, req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, filter, req.SessionID)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	evts, err := events.FromEventFieldsSlice(values)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return evts, next, nil
}

type searchEventsFilter struct {
	eventTypes []string
	condExpr   string
	condParams condFilterParams
}

type condFilterParams struct {
	attrValues map[string]interface{}
	attrNames  map[string]string
}

func fromWhereExpr(cond *types.WhereExpr, params *condFilterParams) (string, error) {
	if cond == nil {
		return "", trace.BadParameter("cond is nil")
	}

	binOp := func(e types.WhereExpr2, format func(a, b string) string) (string, error) {
		left, err := fromWhereExpr(e.L, params)
		if err != nil {
			return "", trace.Wrap(err)
		}
		right, err := fromWhereExpr(e.R, params)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return format(left, right), nil
	}
	if expr, err := binOp(cond.And, func(a, b string) string { return fmt.Sprintf("(%s) AND (%s)", a, b) }); err == nil {
		return expr, nil
	}
	if expr, err := binOp(cond.Or, func(a, b string) string { return fmt.Sprintf("(%s) OR (%s)", a, b) }); err == nil {
		return expr, nil
	}
	if inner, err := fromWhereExpr(cond.Not, params); err == nil {
		return fmt.Sprintf("NOT (%s)", inner), nil
	}

	addAttrValue := func(in interface{}) string {
		for k, v := range params.attrValues {
			if in == v {
				return k
			}
		}
		k := fmt.Sprintf(":condValue%d", len(params.attrValues))
		params.attrValues[k] = in
		return k
	}
	addAttrName := func(n string) string {
		for k, v := range params.attrNames {
			if n == v {
				return fmt.Sprintf("FieldsMap.%s", k)
			}
		}
		k := fmt.Sprintf("#condName%d", len(params.attrNames))
		params.attrNames[k] = n
		return fmt.Sprintf("FieldsMap.%s", k)
	}
	binPred := func(e types.WhereExpr2, format func(a, b string) string) (string, error) {
		left, right := e.L, e.R
		switch {
		case left.Field != "" && right.Field != "":
			return format(addAttrName(left.Field), addAttrName(right.Field)), nil
		case left.Literal != nil && right.Field != "":
			return format(addAttrValue(left.Literal), addAttrName(right.Field)), nil
		case left.Field != "" && right.Literal != nil:
			return format(addAttrName(left.Field), addAttrValue(right.Literal)), nil
		}
		return "", trace.BadParameter("failed to handle binary predicate with arguments %q and %q", left, right)
	}
	if cond.Equals.L != nil && cond.Equals.R != nil {
		if expr, err := binPred(cond.Equals, func(a, b string) string { return fmt.Sprintf("%s = %s", a, b) }); err == nil {
			return expr, nil
		}
	}
	if cond.Contains.L != nil && cond.Contains.R != nil {
		if expr, err := binPred(cond.Contains, func(a, b string) string { return fmt.Sprintf("contains(%s, %s)", a, b) }); err == nil {
			return expr, nil
		}
	}
	return "", trace.BadParameter("failed to convert WhereExpr %q to DynamoDB filter expression", cond)
}

// getTableStatus checks if a given table exists
func (l *Log) getTableStatus(ctx context.Context, tableName string) (tableStatus, error) {
	_, err := l.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	err = convertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return tableStatusMissing, nil
		}
		return tableStatusError, trace.Wrap(err)
	}
	return tableStatusOK, nil
}

// indexExists checks if a given index exists on a given table and that it is active or updating.
func (l *Log) indexExists(ctx context.Context, tableName, indexName string) (bool, error) {
	tableDescription, err := l.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return false, trace.Wrap(convertError(err))
	}

	for _, gsi := range tableDescription.Table.GlobalSecondaryIndexes {
		if *gsi.IndexName == indexName && (gsi.IndexStatus == dynamodbtypes.IndexStatusActive || gsi.IndexStatus == dynamodbtypes.IndexStatusUpdating) {
			return true, nil
		}
	}
	return false, nil
}

// createTable creates a DynamoDB table with a requested name and applies
// the back-end schema to it. The table must not exist.
//
// rangeKey is the name of the 'range key' the schema requires.
// currently is always set to "FullPath" (used to be something else, that's
// why it's a parameter for migration purposes)
func (l *Log) createTable(ctx context.Context, tableName string) error {
	provisionedThroughput := dynamodbtypes.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(l.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(l.WriteCapacityUnits),
	}
	elems := []dynamodbtypes.KeySchemaElement{
		{
			AttributeName: aws.String(keySessionID),
			KeyType:       dynamodbtypes.KeyTypeHash,
		},
		{
			AttributeName: aws.String(keyEventIndex),
			KeyType:       dynamodbtypes.KeyTypeRange,
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             aws.String(tableName),
		AttributeDefinitions:  tableSchema,
		KeySchema:             elems,
		ProvisionedThroughput: &provisionedThroughput,
		GlobalSecondaryIndexes: []dynamodbtypes.GlobalSecondaryIndex{
			{
				IndexName: aws.String(indexTimeSearchV2),
				KeySchema: []dynamodbtypes.KeySchemaElement{
					{
						// Partition by date instead of namespace.
						AttributeName: aws.String(keyDate),
						KeyType:       dynamodbtypes.KeyTypeHash,
					},
					{
						AttributeName: aws.String(keyCreatedAt),
						KeyType:       dynamodbtypes.KeyTypeRange,
					},
				},
				Projection: &dynamodbtypes.Projection{
					ProjectionType: dynamodbtypes.ProjectionTypeAll,
				},
				ProvisionedThroughput: &provisionedThroughput,
			},
		},
	}
	_, err := l.svc.CreateTable(ctx, &c)
	if err != nil {
		return trace.Wrap(err)
	}
	l.logger.InfoContext(ctx, "Waiting until table is created", "table", tableName)
	waiter := dynamodb.NewTableExistsWaiter(l.svc)
	err = waiter.Wait(ctx,
		&dynamodb.DescribeTableInput{TableName: aws.String(tableName)},
		10*time.Minute,
	)
	if err == nil {
		l.logger.InfoContext(ctx, "Table has been created", "table", tableName)
	}

	return trace.Wrap(err)
}

// Close the DynamoDB driver
func (l *Log) Close() error {
	return nil
}

// deleteAllItems deletes all items from the database, used in tests
func (l *Log) deleteAllItems(ctx context.Context) error {
	out, err := l.svc.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(l.Tablename)})
	if err != nil {
		return trace.Wrap(err)
	}
	var requests []dynamodbtypes.WriteRequest
	for _, item := range out.Items {
		requests = append(requests, dynamodbtypes.WriteRequest{
			DeleteRequest: &dynamodbtypes.DeleteRequest{
				Key: map[string]dynamodbtypes.AttributeValue{
					keySessionID:  item[keySessionID],
					keyEventIndex: item[keyEventIndex],
				},
			},
		})
	}

	for len(requests) > 0 {
		top := 25
		if top > len(requests) {
			top = len(requests)
		}
		chunk := requests[:top]
		requests = requests[top:]

		_, err := l.svc.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]dynamodbtypes.WriteRequest{
				l.Tablename: chunk,
			},
		})
		err = convertError(err)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// deleteTable deletes DynamoDB table with a given name
func (l *Log) deleteTable(ctx context.Context, tableName string, wait bool) error {
	tn := aws.String(tableName)
	_, err := l.svc.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: tn})
	if err != nil {
		return trace.Wrap(err)
	}
	if !wait {
		return nil
	}

	waiter := dynamodb.NewTableNotExistsWaiter(l.svc)

	return trace.Wrap(waiter.Wait(ctx,
		&dynamodb.DescribeTableInput{TableName: tn},
		10*time.Minute,
	))
}

var errAWSValidation = errors.New("aws validation error")

func convertError(err error) error {
	if err == nil {
		return nil
	}

	var conditionalCheckFailedError *dynamodbtypes.ConditionalCheckFailedException
	if errors.As(err, &conditionalCheckFailedError) {
		return trace.AlreadyExists("%s", conditionalCheckFailedError.ErrorMessage())
	}

	var throughputExceededError *dynamodbtypes.ProvisionedThroughputExceededException
	if errors.As(err, &throughputExceededError) {
		return trace.ConnectionProblem(throughputExceededError, "%s", throughputExceededError.ErrorMessage())
	}

	var notFoundError *dynamodbtypes.ResourceNotFoundException
	if errors.As(err, &notFoundError) {
		return trace.NotFound("%s", notFoundError.ErrorMessage())
	}

	var collectionLimitExceededError *dynamodbtypes.ItemCollectionSizeLimitExceededException
	if errors.As(err, &notFoundError) {
		return trace.BadParameter("%s", collectionLimitExceededError.ErrorMessage())
	}

	var internalError *dynamodbtypes.InternalServerError
	if errors.As(err, &internalError) {
		return trace.BadParameter("%s", internalError.ErrorMessage())
	}

	var ae smithy.APIError
	if errors.As(err, &ae) {
		if ae.ErrorCode() == ErrValidationException {
			// A ValidationException  type is missing from AWS SDK.
			// Use errAWSValidation that for most cases will contain:
			// "Item size has exceeded the maximum allowed size" AWS validation error.
			return trace.Wrap(errAWSValidation, ae.Error())
		}
	}

	return err
}

type query interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

type eventsFetcher struct {
	log *slog.Logger
	api query

	totalSize  int
	hasLeft    bool
	checkpoint *checkpointKey
	foundStart bool
	dates      []string
	left       int32

	fromUTC   time.Time
	toUTC     time.Time
	tableName string
	forward   bool
	indexName *string
	filter    searchEventsFilter
}

func (l *eventsFetcher) processQueryOutput(output *dynamodb.QueryOutput, hasLeftFun func() bool) ([]event, bool, error) {
	oldIterator := l.checkpoint.Iterator
	l.checkpoint.Iterator = ""

	if output.LastEvaluatedKey != nil {
		m := make(map[string]any)
		if err := attributevalue.UnmarshalMap(output.LastEvaluatedKey, &m); err != nil {
			return nil, false, trace.Wrap(err)
		}

		iter, err := json.Marshal(&m)
		if err != nil {
			return nil, false, err
		}
		l.log.DebugContext(context.Background(), "updating iterator for events fetcher", "iterator", string(iter))
		l.checkpoint.Iterator = string(iter)
	}

	var out []event
	for _, item := range output.Items {
		var e event
		if err := attributevalue.UnmarshalMap(item, &e); err != nil {
			return nil, false, trace.WrapWithMessage(err, "failed to unmarshal event")
		}
		data, err := json.Marshal(e.FieldsMap)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		if !l.foundStart {
			key, err := getSubPageCheckpoint(&e)
			if err != nil {
				return nil, false, trace.Wrap(err)
			}

			if key != l.checkpoint.EventKey {
				continue
			}
			l.foundStart = true
		}
		// Because this may break on non page boundaries an additional
		// checkpoint is needed for sub-page breaks.
		if l.totalSize+len(data) >= events.MaxEventBytesInResponse {
			key, err := getSubPageCheckpoint(&e)
			if err != nil {
				return nil, false, trace.Wrap(err)
			}
			l.log.DebugContext(context.Background(), "breaking up sub-page due to event size", "key", key)
			l.checkpoint.EventKey = key

			// We need to reset the iterator so we get the previous page again.
			l.checkpoint.Iterator = oldIterator

			// If we stopped because of the size limit, we know that at least one event has to be fetched from the
			// current date and old iterator, so we must set it to true independently of the hasLeftFun or
			// the new iterator being empty.
			l.hasLeft = true

			return out, true, nil
		}
		l.totalSize += len(data)
		out = append(out, e)
		l.left--
		if l.left == 0 {
			hf := false
			if hasLeftFun != nil {
				hf = hasLeftFun()
			}
			l.hasLeft = hf || l.checkpoint.Iterator != ""
			l.checkpoint.EventKey = ""
			l.log.DebugContext(context.Background(), "resetting checkpoint event-key due to full page", "has_left", l.hasLeft, "checkpoint", l.checkpoint)
			return out, true, nil
		}
	}
	return out, false, nil
}

func (l *eventsFetcher) QueryByDateIndex(ctx context.Context, filterExpr *string) (values []event, err error) {
	query := "CreatedAtDate = :date AND CreatedAt BETWEEN :start and :end"

dateLoop:
	for i, date := range l.dates {
		l.checkpoint.Date = date

		attributes := map[string]interface{}{
			":date":  date,
			":start": l.fromUTC.Unix(),
			":end":   l.toUTC.Unix(),
		}
		for i, eventType := range l.filter.eventTypes {
			attributes[fmt.Sprintf(":eventType%d", i)] = eventType
		}
		maps.Copy(attributes, l.filter.condParams.attrValues)
		attributeValues, err := attributevalue.MarshalMap(attributes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for {
			input := dynamodb.QueryInput{
				KeyConditionExpression:    aws.String(query),
				TableName:                 aws.String(l.tableName),
				ExpressionAttributeNames:  l.filter.condParams.attrNames,
				ExpressionAttributeValues: attributeValues,
				IndexName:                 aws.String(indexTimeSearchV2),
				Limit:                     aws.Int32(l.left),
				FilterExpression:          filterExpr,
				ScanIndexForward:          aws.Bool(l.forward),
			}

			if l.checkpoint.Iterator != "" {
				m := make(map[string]any)
				err = json.Unmarshal([]byte(l.checkpoint.Iterator), &m)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				input.ExclusiveStartKey, err = attributevalue.MarshalMap(&m)
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}

			start := time.Now()
			out, err := l.api.Query(ctx, &input)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			l.log.DebugContext(ctx, "Query completed.",
				"duration", time.Since(start),
				"items", len(out.Items),
				"forward", l.forward,
				"iterator", l.checkpoint.Iterator,
			)

			hasLeft := func() bool {
				return i+1 != len(l.dates)
			}
			result, limitReached, err := l.processQueryOutput(out, hasLeft)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			values = append(values, result...)
			if limitReached {
				// If we've reached the limit, we need to determine whether there are more events to fetch from the current date
				// or if we need to move the cursor to the next date.
				// To do this, we check if the iterator is empty and if the EventKey is empty.
				// DynamoDB returns an empty iterator if all events from the current date have been consumed.
				// We need to check if the EventKey is empty because it indicates that we left the page midway
				// due to reaching the maximum response size. In this case, we need to resume the query
				// from the same date and the request's iterator to fetch the remainder of the page.
				// If the input iterator is empty but the EventKey is not, we need to resume the query from the same date
				// and we shouldn't move to the next date.
				if i < len(l.dates)-1 && l.checkpoint.Iterator == "" && l.checkpoint.EventKey == "" {
					l.checkpoint.Date = l.dates[i+1]
				}
				return values, nil
			}
			if l.checkpoint.Iterator == "" {
				continue dateLoop
			}
		}
	}
	return values, nil
}

func (l *eventsFetcher) QueryBySessionIDIndex(ctx context.Context, sessionID string, filterExpr *string) (values []event, err error) {
	query := "SessionID = :id"

	attributes := map[string]interface{}{
		":id": sessionID,
	}
	for i, eventType := range l.filter.eventTypes {
		attributes[fmt.Sprintf(":eventType%d", i)] = eventType
	}
	maps.Copy(attributes, l.filter.condParams.attrValues)

	attributeValues, err := attributevalue.MarshalMap(attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 aws.String(l.tableName),
		ExpressionAttributeNames:  l.filter.condParams.attrNames,
		ExpressionAttributeValues: attributeValues,
		IndexName:                 nil, // Use primary SessionID index.
		Limit:                     aws.Int32(l.left),
		FilterExpression:          filterExpr,
		ScanIndexForward:          aws.Bool(l.forward),
	}

	if l.checkpoint.Iterator != "" {
		m := make(map[string]string)
		if err = json.Unmarshal([]byte(l.checkpoint.Iterator), &m); err != nil {
			return nil, trace.Wrap(err)
		}

		input.ExclusiveStartKey, err = attributevalue.MarshalMap(&m)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	start := time.Now()
	out, err := l.api.Query(ctx, &input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	l.log.DebugContext(ctx, "Query completed.",
		"duration", time.Since(start),
		"items", len(out.Items),
		"forward", l.forward,
		"iterator", l.checkpoint.Iterator,
	)

	result, limitReached, err := l.processQueryOutput(out, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values = append(values, result...)
	if limitReached {
		return values, nil
	}
	return values, nil
}
