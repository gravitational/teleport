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
	"maps"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend/dynamo"
	"github.com/gravitational/teleport/lib/events"
	dynamometrics "github.com/gravitational/teleport/lib/observability/metrics/dynamo"
	"github.com/gravitational/teleport/lib/utils"
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
var tableSchema = []*dynamodb.AttributeDefinition{
	// Existing attributes pre RFD 24.
	{
		AttributeName: aws.String(keySessionID),
		AttributeType: aws.String("S"),
	},
	{
		AttributeName: aws.String(keyEventIndex),
		AttributeType: aws.String("N"),
	},
	{
		AttributeName: aws.String(keyCreatedAt),
		AttributeType: aws.String("N"),
	},
	// New attribute in RFD 24.
	{
		AttributeName: aws.String(keyDate),
		AttributeType: aws.String("S"),
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
	ReadMaxCapacity int64
	// ReadMinCapacity is the minimum provisioned read capacity.
	ReadMinCapacity int64
	// ReadTargetValue is the ratio of consumed read to provisioned capacity.
	ReadTargetValue float64
	// WriteMaxCapacity is the maximum provisioned write capacity.
	WriteMaxCapacity int64
	// WriteMinCapacity is the minimum provisioned write capacity.
	WriteMinCapacity int64
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

	return nil
}

// Log is a dynamo-db backed storage of events
type Log struct {
	// Entry is a log entry
	*log.Entry
	// Config is a backend configuration
	Config
	svc dynamodbiface.DynamoDBAPI

	// session holds the AWS client.
	session *awssession.Session
}

type event struct {
	SessionID      string
	EventIndex     int64
	EventType      string
	CreatedAt      int64
	Expires        *int64 `json:"Expires,omitempty"`
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
	l := log.WithFields(log.Fields{
		teleport.ComponentKey: teleport.Component(teleport.ComponentDynamoDB),
	})
	l.Info("Initializing event backend.")

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b := &Log{
		Entry:  l,
		Config: cfg,
	}

	awsConfig := aws.Config{}

	// Override the default environment's region if value set in YAML file:
	if cfg.Region != "" {
		awsConfig.Region = aws.String(cfg.Region)
	}

	// Override the service endpoint using the "endpoint" query parameter from
	// "audit_events_uri". This is for non-AWS DynamoDB-compatible backends.
	if cfg.Endpoint != "" {
		awsConfig.Endpoint = aws.String(cfg.Endpoint)
	}

	b.session, err = awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
		Config:            awsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create DynamoDB service.
	svc, err := dynamometrics.NewAPIMetrics(dynamometrics.Events, dynamodb.New(b.session, &aws.Config{
		// Setting this on the individual service instead of the session, as DynamoDB Streams
		// and Application Auto Scaling do not yet have FIPS endpoints in non-GovCloud.
		// See also: https://aws.amazon.com/compliance/fips/#FIPS_Endpoints_by_Service
		UseFIPSEndpoint: events.FIPSProtoStateToAWSState(cfg.UseFIPSEndpoint),
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.svc = svc

	// check if the table exists?
	ts, err := b.getTableStatus(ctx, b.Tablename)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		break
	case tableStatusMissing:
		err = b.createTable(ctx, b.Tablename)
	case tableStatusNeedsMigration:
		return nil, trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = dynamo.TurnOnTimeToLive(ctx, b.svc, b.Tablename, keyExpires)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Enable continuous backups if requested.
	if b.Config.EnableContinuousBackups {
		if err := dynamo.SetContinuousBackups(ctx, b.svc, b.Tablename); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Enable auto scaling if requested.
	if b.Config.EnableAutoScaling {
		if err := dynamo.SetAutoScaling(ctx, applicationautoscaling.New(b.session), dynamo.GetTableID(b.Tablename), dynamo.AutoScalingParams{
			ReadMinCapacity:  b.Config.ReadMinCapacity,
			ReadMaxCapacity:  b.Config.ReadMaxCapacity,
			ReadTargetValue:  b.Config.ReadTargetValue,
			WriteMinCapacity: b.Config.WriteMinCapacity,
			WriteMaxCapacity: b.Config.WriteMaxCapacity,
			WriteTargetValue: b.Config.WriteTargetValue,
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := dynamo.SetAutoScaling(ctx, applicationautoscaling.New(b.session), dynamo.GetIndexID(b.Tablename, indexTimeSearchV2), dynamo.AutoScalingParams{
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

	return b, nil
}

type tableStatus int

const (
	tableStatusError = iota
	tableStatusMissing
	tableStatusNeedsMigration
	tableStatusOK
)

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
		return trace.BadParameter(err.Error())
	}
	if err := l.putAuditEvent(context.WithValue(ctx, largeEventHandledContextKey, true), sessionID, se); err != nil {
		return trace.BadParameter(err.Error())
	}
	fields := log.Fields{"event_id": in.GetID(), "event_type": in.GetType()}
	l.WithFields(fields).Info("Uploaded trimmed event to DynamoDB backend.")
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
	l.WithFields(log.Fields{"event_id": in.GetID(), "event_type": in.GetType()}).Debug("Event index overwritten")
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
	m, ok := event.(messageSizeTrimmer)
	if !ok {
		return nil, false
	}
	return m.TrimToMaxSize(maxItemSize), true
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

	if _, err = l.svc.PutItemWithContext(ctx, input); err != nil {
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
			l.
				WithError(err).
				WithFields(log.Fields{"event_type": in.GetType(), "session_id": sessionID, "event_index": in.GetIndex()}).
				Error("Conflict on event session_id and event_index")
			return trace.Wrap(l.handleConditionError(ctx, err, sessionID, in))
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
	av, err := dynamodbattribute.MarshalMap(e)
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

type messageSizeTrimmer interface {
	TrimToMaxSize(int) apievents.AuditEvent
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
	Iterator map[string]*dynamodb.AttributeValue `json:"iterator,omitempty"`

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
	return l.searchEventsWithFilter(ctx, req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, searchEventsFilter{eventTypes: req.EventTypes}, "")
}

func (l *Log) searchEventsWithFilter(ctx context.Context, fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startKey string, filter searchEventsFilter, sessionID string) ([]apievents.AuditEvent, string, error) {
	rawEvents, lastKey, err := l.searchEventsRaw(ctx, fromUTC, toUTC, namespace, limit, order, startKey, filter, sessionID)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	eventArr := make([]apievents.AuditEvent, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		event, err := events.FromEventFields(rawEvent.FieldsMap)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		eventArr = append(eventArr, event)
	}

	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = byTimeAndIndex(eventArr)
	case types.EventOrderDescending:
		toSort = sort.Reverse(byTimeAndIndex(eventArr))
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}

	sort.Sort(toSort)
	return eventArr, lastKey, nil
}

// ByTimeAndIndex sorts events by time
// and if there are several session events with the same session by event index.
type byTimeAndIndex []apievents.AuditEvent

func (f byTimeAndIndex) Len() int {
	return len(f)
}

func (f byTimeAndIndex) Less(i, j int) bool {
	itime := f[i].GetTime()
	jtime := f[j].GetTime()
	if itime.Equal(jtime) && events.GetSessionID(f[i]) == events.GetSessionID(f[j]) {
		return f[i].GetIndex() < f[j].GetIndex()
	}
	return itime.Before(jtime)
}

func (f byTimeAndIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
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
	checkpoint, err := getCheckpointFromStartKey(startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if checkpoint.Date != "" {
		if t, err := time.Parse(time.DateOnly, checkpoint.Date); err == nil {
			d := fromUTC.Unix()
			// if fromUTC at 00:00:00 is bigger than the cursor,
			// reset the cursor and advance to next day.
			if time.Unix(d-d%(24*3600), 0).After(t) {
				checkpoint = checkpointKey{}
			}
		}
	}

	totalSize := 0
	dates := daysBetween(fromUTC, toUTC)
	if order == types.EventOrderDescending {
		dates = reverseStrings(dates)
	}

	indexName := aws.String(indexTimeSearchV2)
	var left int64
	if limit != 0 {
		left = int64(limit)
	} else {
		left = math.MaxInt64
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

	logger := l.WithFields(log.Fields{
		"From":      fromUTC,
		"To":        toUTC,
		"Namespace": namespace,
		"Filter":    filter,
		"Limit":     limit,
		"StartKey":  startKey,
		"Order":     order,
	})

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
	if checkpoint.Iterator == nil {
		return time.Time{}, errors.New("missing iterator")
	}
	var e event
	if err := dynamodbattribute.UnmarshalMap(checkpoint.Iterator, &e); err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	if e.CreatedAt <= 0 {
		// Value <= 0 means that either createdAt was not returned or
		// it has 0 values, either way, we can't use that value.
		return time.Time{}, errors.New("createdAt is invalid")
	}

	return time.Unix(e.CreatedAt, 0), nil
}

func getCheckpointFromStartKey(startKey string) (checkpointKey, error) {
	var checkpoint checkpointKey
	if startKey == "" {
		return checkpoint, nil
	}
	// If a checkpoint key is provided, unmarshal it so we can work with it's parts.
	if err := json.Unmarshal([]byte(startKey), &checkpoint); err != nil {
		return checkpoint, trace.Wrap(err)
	}
	return checkpoint, nil
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
	filter := searchEventsFilter{eventTypes: []string{events.SessionEndEvent, events.WindowsDesktopSessionEndEvent}}
	if req.Cond != nil {
		params := condFilterParams{attrValues: make(map[string]interface{}), attrNames: make(map[string]string)}
		expr, err := fromWhereExpr(req.Cond, &params)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condExpr = expr
		filter.condParams = params
	}
	return l.searchEventsWithFilter(ctx, req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, filter, req.SessionID)
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
	_, err := l.svc.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
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
	tableDescription, err := l.svc.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return false, trace.Wrap(convertError(err))
	}

	for _, gsi := range tableDescription.Table.GlobalSecondaryIndexes {
		if *gsi.IndexName == indexName && (*gsi.IndexStatus == dynamodb.IndexStatusActive || *gsi.IndexStatus == dynamodb.IndexStatusUpdating) {
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
	provisionedThroughput := dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(l.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(l.WriteCapacityUnits),
	}
	elems := []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String(keySessionID),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String(keyEventIndex),
			KeyType:       aws.String("RANGE"),
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             aws.String(tableName),
		AttributeDefinitions:  tableSchema,
		KeySchema:             elems,
		ProvisionedThroughput: &provisionedThroughput,
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String(indexTimeSearchV2),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						// Partition by date instead of namespace.
						AttributeName: aws.String(keyDate),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String(keyCreatedAt),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
				ProvisionedThroughput: &provisionedThroughput,
			},
		},
	}
	_, err := l.svc.CreateTableWithContext(ctx, &c)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Waiting until table %q is created", tableName)
	err = l.svc.WaitUntilTableExistsWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		log.Infof("Table %q has been created", tableName)
	}
	return trace.Wrap(err)
}

// Close the DynamoDB driver
func (l *Log) Close() error {
	return nil
}

// deleteAllItems deletes all items from the database, used in tests
func (l *Log) deleteAllItems(ctx context.Context) error {
	out, err := l.svc.ScanWithContext(ctx, &dynamodb.ScanInput{TableName: aws.String(l.Tablename)})
	if err != nil {
		return trace.Wrap(err)
	}
	var requests []*dynamodb.WriteRequest
	for _, item := range out.Items {
		requests = append(requests, &dynamodb.WriteRequest{
			DeleteRequest: &dynamodb.DeleteRequest{
				Key: map[string]*dynamodb.AttributeValue{
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

		_, err := l.svc.BatchWriteItemWithContext(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]*dynamodb.WriteRequest{
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
	_, err := l.svc.DeleteTableWithContext(ctx, &dynamodb.DeleteTableInput{TableName: tn})
	if err != nil {
		return trace.Wrap(err)
	}
	if wait {
		return trace.Wrap(
			l.svc.WaitUntilTableNotExistsWithContext(ctx, &dynamodb.DescribeTableInput{TableName: tn}))
	}
	return nil
}

var errAWSValidation = errors.New("aws validation error")

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
		return trace.AlreadyExists(aerr.Error())
	case dynamodb.ErrCodeProvisionedThroughputExceededException:
		return trace.ConnectionProblem(aerr, aerr.Error())
	case dynamodb.ErrCodeResourceNotFoundException:
		return trace.NotFound(aerr.Error())
	case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
		return trace.BadParameter(aerr.Error())
	case dynamodb.ErrCodeInternalServerError:
		return trace.BadParameter(aerr.Error())
	case ErrValidationException:
		// A ValidationException  type is missing from AWS SDK.
		// Use errAWSValidation that for most cases will contain:
		// "Item size has exceeded the maximum allowed size" AWS validation error.
		return trace.Wrap(errAWSValidation, aerr.Error())
	default:
		return err
	}
}

type query interface {
	QueryWithContext(ctx context.Context, input *dynamodb.QueryInput, opts ...request.Option) (*dynamodb.QueryOutput, error)
}

type eventsFetcher struct {
	log *log.Entry
	api query

	totalSize  int
	hasLeft    bool
	checkpoint *checkpointKey
	foundStart bool
	dates      []string
	left       int64

	fromUTC   time.Time
	toUTC     time.Time
	tableName string
	forward   bool
	indexName *string
	filter    searchEventsFilter
}

func (l *eventsFetcher) processQueryOutput(output *dynamodb.QueryOutput, hasLeftFun func() bool) ([]event, bool, error) {
	var out []event
	oldIterator := l.checkpoint.Iterator
	l.checkpoint.Iterator = output.LastEvaluatedKey

	for _, item := range output.Items {
		var e event
		if err := dynamodbattribute.UnmarshalMap(item, &e); err != nil {
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
			hf := false
			if hasLeftFun != nil {
				hf = hasLeftFun()
			}
			l.hasLeft = hf || len(l.checkpoint.Iterator) != 0

			key, err := getSubPageCheckpoint(&e)
			if err != nil {
				return nil, false, trace.Wrap(err)
			}
			l.checkpoint.EventKey = key

			// We need to reset the iterator so we get the previous page again.
			l.checkpoint.Iterator = oldIterator
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
			l.hasLeft = hf || len(l.checkpoint.Iterator) != 0
			l.checkpoint.EventKey = ""
			return out, true, nil
		}
	}
	return out, false, nil
}

func (l *eventsFetcher) QueryByDateIndex(ctx context.Context, filterExpr *string) (values []event, err error) {
	query := "CreatedAtDate = :date AND CreatedAt BETWEEN :start and :end"
	var attributeNames map[string]*string
	if len(l.filter.condParams.attrNames) > 0 {
		attributeNames = aws.StringMap(l.filter.condParams.attrNames)
	}

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
		attributeValues, err := dynamodbattribute.MarshalMap(attributes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for {
			input := dynamodb.QueryInput{
				KeyConditionExpression:    aws.String(query),
				TableName:                 aws.String(l.tableName),
				ExpressionAttributeNames:  attributeNames,
				ExpressionAttributeValues: attributeValues,
				IndexName:                 aws.String(indexTimeSearchV2),
				ExclusiveStartKey:         l.checkpoint.Iterator,
				Limit:                     aws.Int64(l.left),
				FilterExpression:          filterExpr,
				ScanIndexForward:          aws.Bool(l.forward),
			}
			start := time.Now()
			out, err := l.api.QueryWithContext(ctx, &input)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			l.log.WithFields(log.Fields{
				"duration": time.Since(start),
				"items":    len(out.Items),
				"forward":  l.forward,
				"iterator": l.checkpoint.Iterator,
			}).Debugf("Query completed.")

			hasLeft := func() bool {
				return i+1 != len(l.dates)
			}
			result, limitReached, err := l.processQueryOutput(out, hasLeft)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			values = append(values, result...)
			if limitReached {
				return values, nil
			}
			if len(l.checkpoint.Iterator) == 0 {
				continue dateLoop
			}
		}
	}
	return values, nil
}

func (l *eventsFetcher) QueryBySessionIDIndex(ctx context.Context, sessionID string, filterExpr *string) (values []event, err error) {
	query := "SessionID = :id"
	var attributeNames map[string]*string
	if len(l.filter.condParams.attrNames) > 0 {
		attributeNames = aws.StringMap(l.filter.condParams.attrNames)
	}

	attributes := map[string]interface{}{
		":id": sessionID,
	}
	for i, eventType := range l.filter.eventTypes {
		attributes[fmt.Sprintf(":eventType%d", i)] = eventType
	}
	maps.Copy(attributes, l.filter.condParams.attrValues)

	attributeValues, err := dynamodbattribute.MarshalMap(attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 aws.String(l.tableName),
		ExpressionAttributeNames:  attributeNames,
		ExpressionAttributeValues: attributeValues,
		IndexName:                 nil, // Use primary SessionID index.
		ExclusiveStartKey:         l.checkpoint.Iterator,
		Limit:                     aws.Int64(l.left),
		FilterExpression:          filterExpr,
		ScanIndexForward:          aws.Bool(l.forward),
	}
	start := time.Now()
	out, err := l.api.QueryWithContext(ctx, &input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	l.log.WithFields(log.Fields{
		"duration": time.Since(start),
		"items":    len(out.Items),
		"forward":  l.forward,
		"iterator": l.checkpoint.Iterator,
	}).Debugf("Query completed.")

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
