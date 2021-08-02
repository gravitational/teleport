/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package dynamoevents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dynamo"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

// iso8601DateFormat is the time format used by the date attribute on events.
const iso8601DateFormat = "2006-01-02"

// The maximum amount of concurrent batch upload workers for data migration.
// 32 was chosen here as it's a non-crazy number that allows reasonably
// fast migration of millions of events.
const maxMigrationWorkers = 32

// The maximum size of a DynamoDB batch write.
const DynamoBatchSize = 25

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

const indexV2CreationLock = "dynamoEvents/indexV2Creation"
const rfd24MigrationLock = "dynamoEvents/rfd24Migration"
const rfd24MigrationLockTTL = 5 * time.Minute

// Config structure represents DynamoDB confniguration as appears in `storage` section
// of Teleport YAML
type Config struct {
	// Region is where DynamoDB Table will be used to store k/v
	Region string `json:"region,omitempty"`
	// Tablename where to store K/V in DynamoDB
	Tablename string `json:"table_name,omitempty"`
	// ReadCapacityUnits is Dynamodb read capacity units
	ReadCapacityUnits int64 `json:"read_capacity_units"`
	// WriteCapacityUnits is Dynamodb write capacity units
	WriteCapacityUnits int64 `json:"write_capacity_units"`
	// RetentionPeriod is a default retention period for events
	RetentionPeriod time.Duration
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator
	UIDGenerator utils.UID
	// Endpoint is an optional non-AWS endpoint
	Endpoint string `json:"endpoint,omitempty"`

	// EnableContinuousBackups is used to enable PITR (Point-In-Time Recovery).
	EnableContinuousBackups bool

	// EnableAutoScaling is used to enable auto scaling policy.
	EnableAutoScaling bool
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
}

// SetFromURL sets values on the Config from the supplied URI
func (cfg *Config) SetFromURL(in *url.URL) error {
	if endpoint := in.Query().Get(teleport.Endpoint); endpoint != "" {
		cfg.Endpoint = endpoint
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
	if cfg.RetentionPeriod == 0 {
		cfg.RetentionPeriod = DefaultRetentionPeriod
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
	svc *dynamodb.DynamoDB

	// session holds the AWS client.
	session *awssession.Session

	// Backend holds the data backend used.
	// This is used for locking.
	backend backend.Backend

	// readyForQuery is used to determine if all indexes are in place
	// for event queries.
	readyForQuery *atomic.Bool
}

type event struct {
	SessionID      string
	EventIndex     int64
	EventType      string
	CreatedAt      int64
	Expires        *int64 `json:"Expires,omitempty"`
	Fields         string
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

	// indexTimeSearch is a secondary global index that allows searching
	// of the events by time
	indexTimeSearch = "timesearch"

	// indexTimeSearchV2 is the new secondary global index proposed in RFD 24.
	// Allows searching events by time.
	indexTimeSearchV2 = "timesearchV2"

	// DefaultReadCapacityUnits specifies default value for read capacity units
	DefaultReadCapacityUnits = 10

	// DefaultWriteCapacityUnits specifies default value for write capacity units
	DefaultWriteCapacityUnits = 10

	// DefaultRetentionPeriod is a default data retention period in events table
	// default is 1 year
	DefaultRetentionPeriod = 365 * 24 * time.Hour
)

// New returns new instance of DynamoDB backend.
// It's an implementation of backend API's NewFunc
func New(ctx context.Context, cfg Config, backend backend.Backend) (*Log, error) {
	l := log.WithFields(log.Fields{
		trace.Component: teleport.Component(teleport.ComponentDynamoDB),
	})
	l.Info("Initializing event backend.")

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b := &Log{
		Entry:         l,
		Config:        cfg,
		backend:       backend,
		readyForQuery: atomic.NewBool(false),
	}
	// create an AWS session using default SDK behavior, i.e. it will interpret
	// the environment and ~/.aws directory just like an AWS CLI tool would:
	b.session, err = awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// override the default environment (region + credentials) with the values
	// from the YAML file:
	if cfg.Region != "" {
		b.session.Config.Region = aws.String(cfg.Region)
	}

	// Override the service endpoint using the "endpoint" query parameter from
	// "audit_events_uri". This is for non-AWS DynamoDB-compatible backends.
	if cfg.Endpoint != "" {
		b.session.Config.Endpoint = aws.String(cfg.Endpoint)
	}

	// create DynamoDB service:
	b.svc = dynamodb.New(b.session)

	// check if the table exists?
	ts, err := b.getTableStatus(b.Tablename)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		break
	case tableStatusMissing:
		err = b.createTable(b.Tablename)
	case tableStatusNeedsMigration:
		return nil, trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.turnOnTimeToLive()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Migrate the table according to RFD 24 if it still has the old schema.
	go b.migrateRFD24WithRetry(ctx)

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

// migrateRFD24WithRetry tries the migration multiple times until it succeeds in the case
// of spontaneous errors.
func (l *Log) migrateRFD24WithRetry(ctx context.Context) {
	for {
		err := l.migrateRFD24(ctx)

		if err == nil {
			break
		}

		delay := utils.HalfJitter(time.Minute)
		log.WithError(err).Errorf("Background migration task failed, retrying in %f seconds", delay.Seconds())
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			log.WithError(ctx.Err()).Error("Background migration task cancelled")
			return
		}
	}
}

// migrateRFD24 checks if any migration actions need to be performed
// as specified in RFD 24 and applies them as needed.
//
// In the case of this being called concurrently from multiple auth servers the
// behaviour depends on the current state of the migration. If the V2 index is not
// yet visible, one server will receive an error. In the case of event migration
// being in progress both servers will attempt to migrate events, in some cases this may
// lead to increased migration performance via parallelism but it may also lead to duplicated work.
// No data or schema can be broken by multiple auth servers calling this function
// but it is preferable to perform the migration with only one active auth server.
// To combat this behaviour the servers will detect errors and wait a relatively long
// jittered interval until retrying migration again. This allows one server to pull ahead
// and finish or make significant progress on the migration.
func (l *Log) migrateRFD24(ctx context.Context) error {
	hasIndexV1, err := l.indexExists(l.Tablename, indexTimeSearch)
	if err != nil {
		return trace.Wrap(err)
	}

	// Table is already up to date.
	// We use the existence of the V1 index as a completion flag
	// for migration. We remove it at the end of the migration which
	// means it is finished if it doesn't exist.
	if !hasIndexV1 {
		l.readyForQuery.Store(true)
		return nil
	}

	// Creates the v2 index if it doesn't already exist.
	err = backend.RunWhileLocked(ctx, l.backend, indexV2CreationLock, rfd24MigrationLockTTL, func(ctx context.Context) error {
		err = l.createV2GSI(ctx)
		l.readyForQuery.Store(true)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	if err != nil {
		return trace.Wrap(err)
	}

	// Acquire a lock so that only one auth server attempts to perform the migration at any given time.
	// If an auth server does in a HA-setup the other auth servers will pick up the migration automatically.
	err = backend.RunWhileLocked(ctx, l.backend, rfd24MigrationLock, rfd24MigrationLockTTL, func(ctx context.Context) error {
		hasIndexV1, err := l.indexExists(l.Tablename, indexTimeSearch)
		if err != nil {
			return trace.Wrap(err)
		}

		if !hasIndexV1 {
			return nil
		}

		// Migrate events to the new format so that the V2 index can use them.
		log.Info("Starting event migration to v6.2 format")
		err = l.migrateDateAttribute(ctx)
		if err != nil {
			return trace.WrapWithMessage(err, "Encountered error migrating events to v6.2 format")
		}

		// Remove the old index, marking migration as complete
		log.Info("Removing old DynamoDB index")
		err = l.removeV1GSI()
		if err != nil {
			return trace.WrapWithMessage(err, "Migrated all events to v6.2 format successfully but failed to remove old index.")
		}

		return nil
	})

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// EmitAuditEvent emits audit event
func (l *Log) EmitAuditEvent(ctx context.Context, in events.AuditEvent) error {
	data, err := utils.FastMarshal(in)
	if err != nil {
		return trace.Wrap(err)
	}

	var sessionID string
	getter, ok := in.(events.SessionMetadataGetter)
	if ok && getter.GetSessionID() != "" {
		sessionID = getter.GetSessionID()
	} else {
		// no session id - global event gets a random uuid to get a good partition
		// key distribution
		sessionID = uuid.New()
	}

	e := event{
		SessionID:      sessionID,
		EventIndex:     in.GetIndex(),
		EventType:      in.GetType(),
		EventNamespace: defaults.Namespace,
		CreatedAt:      in.GetTime().Unix(),
		Fields:         string(data),
		CreatedAtDate:  in.GetTime().Format(iso8601DateFormat),
	}
	l.setExpiry(&e)
	av, err := dynamodbattribute.MarshalMap(e)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(l.Tablename),
	}
	_, err = l.svc.PutItemWithContext(ctx, &input)
	err = convertError(err)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// EmitAuditEventLegacy emits audit event
func (l *Log) EmitAuditEventLegacy(ev events.Event, fields events.EventFields) error {
	sessionID := fields.GetString(events.SessionEventID)
	eventIndex := fields.GetInt(events.EventIndex)
	// no session id - global event gets a random uuid to get a good partition
	// key distribution
	if sessionID == "" {
		sessionID = uuid.New()
	}
	err := events.UpdateEventFields(ev, fields, l.Clock, l.UIDGenerator)
	if err != nil {
		log.Error(trace.DebugReport(err))
	}
	created := fields.GetTime(events.EventTime)
	if created.IsZero() {
		created = l.Clock.Now().UTC()
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err)
	}
	e := event{
		SessionID:      sessionID,
		EventIndex:     int64(eventIndex),
		EventType:      fields.GetString(events.EventType),
		EventNamespace: defaults.Namespace,
		CreatedAt:      created.Unix(),
		Fields:         string(data),
		CreatedAtDate:  created.Format(iso8601DateFormat),
	}
	l.setExpiry(&e)
	av, err := dynamodbattribute.MarshalMap(e)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(l.Tablename),
	}
	_, err = l.svc.PutItem(&input)
	err = convertError(err)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (l *Log) setExpiry(e *event) {
	if l.RetentionPeriod == 0 {
		return
	}
	e.Expires = aws.Int64(l.Clock.Now().UTC().Add(l.RetentionPeriod).Unix())
}

// PostSessionSlice sends chunks of recorded session to the event log
func (l *Log) PostSessionSlice(slice events.SessionSlice) error {
	var requests []*dynamodb.WriteRequest
	for _, chunk := range slice.Chunks {
		// if legacy event with no type or print event, skip it
		if chunk.EventType == events.SessionPrintEvent || chunk.EventType == "" {
			continue
		}
		fields, err := events.EventFromChunk(slice.SessionID, chunk)
		if err != nil {
			return trace.Wrap(err)
		}
		data, err := json.Marshal(fields)
		if err != nil {
			return trace.Wrap(err)
		}

		timeAt := time.Unix(0, chunk.Time).In(time.UTC)

		event := event{
			SessionID:      slice.SessionID,
			EventNamespace: defaults.Namespace,
			EventType:      chunk.EventType,
			EventIndex:     chunk.EventIndex,
			CreatedAt:      timeAt.Unix(),
			Fields:         string(data),
			CreatedAtDate:  timeAt.Format(iso8601DateFormat),
		}
		l.setExpiry(&event)
		item, err := dynamodbattribute.MarshalMap(event)
		if err != nil {
			return trace.Wrap(err)
		}
		requests = append(requests, &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		})
	}
	// no chunks to post (all chunks are print events)
	if len(requests) == 0 {
		return nil
	}
	input := dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			l.Tablename: requests,
		},
	}
	req, _ := l.svc.BatchWriteItemRequest(&input)
	err := req.Send()
	err = convertError(err)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (l *Log) UploadSessionRecording(events.SessionRecording) error {
	return trace.BadParameter("not supported")
}

// GetSessionChunk returns a reader which can be used to read a byte stream
// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
// beginning) up to maxBytes bytes.
//
// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
func (l *Log) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, nil
}

// Returns all events that happen during a session sorted by time
// (oldest first).
//
// after tells to use only return events after a specified cursor Id
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (l *Log) GetSessionEvents(namespace string, sid session.ID, after int, inlcudePrintEvents bool) ([]events.EventFields, error) {
	var values []events.EventFields
	query := "SessionID = :sessionID AND EventIndex >= :eventIndex"
	attributes := map[string]interface{}{
		":sessionID":  string(sid),
		":eventIndex": after,
	}
	attributeValues, err := dynamodbattribute.MarshalMap(attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 aws.String(l.Tablename),
		ExpressionAttributeValues: attributeValues,
	}
	out, err := l.svc.Query(&input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range out.Items {
		var e event
		if err := dynamodbattribute.UnmarshalMap(item, &e); err != nil {
			return nil, trace.BadParameter("failed to unmarshal event for session %q: %v", string(sid), err)
		}
		var fields events.EventFields
		data := []byte(e.Fields)
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, trace.BadParameter("failed to unmarshal event for session %q: %v", string(sid), err)
		}
		values = append(values, fields)
	}
	sort.Sort(events.ByTimeAndIndex(values))
	return values, nil
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
func (l *Log) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	rawEvents, lastKey, err := l.searchEventsRaw(fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	eventArr := make([]events.AuditEvent, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		var fields events.EventFields
		if err := utils.FastUnmarshal([]byte(rawEvent.Fields), &fields); err != nil {
			return nil, "", trace.Wrap(err)
		}
		event, err := events.FromEventFields(fields)
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
type byTimeAndIndex []events.AuditEvent

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

type notReadyYetError struct{}

func (notReadyYetError) Error() string {
	return "The DynamoDB event backend is not ready to accept queries yet. Please retry in a couple of seconds."
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
func (l *Log) searchEventsRaw(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]event, string, error) {
	if !l.readyForQuery.Load() {
		return nil, "", trace.Wrap(notReadyYetError{})
	}

	var checkpoint checkpointKey

	// If a checkpoint key is provided, unmarshal it so we can work with it's parts.
	if startKey != "" {
		if err := json.Unmarshal([]byte(startKey), &checkpoint); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var values []event
	totalSize := 0
	dates := daysBetween(fromUTC, toUTC)
	if order == types.EventOrderDescending {
		dates = reverseStrings(dates)
	}

	query := "CreatedAtDate = :date AND CreatedAt BETWEEN :start and :end"
	g := l.WithFields(log.Fields{"From": fromUTC, "To": toUTC, "Namespace": namespace, "EventTypes": eventTypes, "Limit": limit, "StartKey": startKey, "Order": order})
	var left int64
	if limit != 0 {
		left = int64(limit)
	} else {
		left = math.MaxInt64
	}

	var typeFilter *string
	if len(eventTypes) != 0 {
		typeList := eventFilterList(len(eventTypes))
		typeFilter = aws.String(fmt.Sprintf("EventType IN %s", typeList))
	}

	// Resume scanning at the correct date. We need to do this because we send individual queries per date
	// and you can't resume a query with the wrong iterator checkpoint.
	//
	// We need to perform a guard check on the length of `dates` here in case a query is submitted with
	// `toUTC` occurring before `fromUTC`.
	if checkpoint.Date != "" && len(dates) > 0 {
		for dates[0] != checkpoint.Date {
			dates = dates[1:]
		}
	}

	hasLeft := false
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

	// This is the main query loop, here we send individual queries for each date and
	// we stop if we hit `limit` or process all dates, whichever comes first.
dateLoop:
	for i, date := range dates {
		checkpoint.Date = date

		attributes := map[string]interface{}{
			":date":  date,
			":start": fromUTC.Unix(),
			":end":   toUTC.Unix(),
		}

		for i := range eventTypes {
			attributes[fmt.Sprintf(":eventType%d", i)] = eventTypes[i]
		}

		attributeValues, err := dynamodbattribute.MarshalMap(attributes)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		for {
			input := dynamodb.QueryInput{
				KeyConditionExpression:    aws.String(query),
				TableName:                 aws.String(l.Tablename),
				ExpressionAttributeValues: attributeValues,
				IndexName:                 aws.String(indexTimeSearchV2),
				ExclusiveStartKey:         checkpoint.Iterator,
				Limit:                     aws.Int64(left),
				FilterExpression:          typeFilter,
				ScanIndexForward:          aws.Bool(forward),
			}

			start := time.Now()
			out, err := l.svc.Query(&input)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			g.WithFields(log.Fields{"duration": time.Since(start), "items": len(out.Items), "forward": forward, "iterator": checkpoint.Iterator}).Debugf("Query completed.")
			oldIterator := checkpoint.Iterator
			checkpoint.Iterator = out.LastEvaluatedKey

			for _, item := range out.Items {
				var e event
				if err := dynamodbattribute.UnmarshalMap(item, &e); err != nil {
					return nil, "", trace.WrapWithMessage(err, "failed to unmarshal event")
				}
				var fields events.EventFields
				data := []byte(e.Fields)
				if err := json.Unmarshal(data, &fields); err != nil {
					return nil, "", trace.BadParameter("failed to unmarshal event %v", err)
				}

				if !foundStart {
					key, err := getSubPageCheckpoint(&e)
					if err != nil {
						return nil, "", trace.Wrap(err)
					}

					if key != checkpoint.EventKey {
						continue
					}

					foundStart = true
				}

				// Because this may break on non page boundaries an additional
				// checkpoint is needed for sub-page breaks.
				if totalSize+len(data) >= events.MaxEventBytesInResponse {
					hasLeft = i+1 != len(dates) || len(checkpoint.Iterator) != 0

					key, err := getSubPageCheckpoint(&e)
					if err != nil {
						return nil, "", trace.Wrap(err)
					}
					checkpoint.EventKey = key

					// We need to reset the iterator so we get the previous page again.
					checkpoint.Iterator = oldIterator
					break dateLoop
				}

				totalSize += len(data)
				values = append(values, e)
				left--

				if left == 0 {
					hasLeft = i+1 != len(dates) || len(checkpoint.Iterator) != 0
					checkpoint.EventKey = ""
					break dateLoop
				}
			}

			if len(checkpoint.Iterator) == 0 {
				continue dateLoop
			}
		}
	}

	var lastKey []byte
	var err error

	if hasLeft {
		lastKey, err = json.Marshal(&checkpoint)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	return values, string(lastKey), nil
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
func (l *Log) SearchSessionEvents(fromUTC time.Time, toUTC time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	// only search for specific event types
	query := []string{
		events.SessionStartEvent,
		events.SessionEndEvent,
	}
	return l.SearchEvents(fromUTC, toUTC, apidefaults.Namespace, query, limit, order, startKey)
}

// WaitForDelivery waits for resources to be released and outstanding requests to
// complete after calling Close method
func (l *Log) WaitForDelivery(ctx context.Context) error {
	return nil
}

func (l *Log) turnOnTimeToLive() error {
	status, err := l.svc.DescribeTimeToLive(&dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(l.Tablename),
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	switch aws.StringValue(status.TimeToLiveDescription.TimeToLiveStatus) {
	case dynamodb.TimeToLiveStatusEnabled, dynamodb.TimeToLiveStatusEnabling:
		return nil
	}
	_, err = l.svc.UpdateTimeToLive(&dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(l.Tablename),
		TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
			AttributeName: aws.String(keyExpires),
			Enabled:       aws.Bool(true),
		},
	})
	return convertError(err)
}

// getTableStatus checks if a given table exists
func (l *Log) getTableStatus(tableName string) (tableStatus, error) {
	_, err := l.svc.DescribeTable(&dynamodb.DescribeTableInput{
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
func (l *Log) indexExists(tableName, indexName string) (bool, error) {
	tableDescription, err := l.svc.DescribeTable(&dynamodb.DescribeTableInput{
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

// createV2GSI creates the new global secondary index if it does not exist and updates
// the schema to add a string key `date`.
//
// This does not remove the old global secondary index.
// This must be done at a later point in time when all events have been migrated as per RFD 24.
//
// Invariants:
// - This function may not be called concurrently across the cluster.
// - This function must be called before the
//   backend is considered initialized and the main Teleport process is started.
func (l *Log) createV2GSI(ctx context.Context) error {
	v2Exists, err := l.indexExists(l.Tablename, indexTimeSearchV2)
	if err != nil {
		return trace.Wrap(err)
	}

	if v2Exists {
		return nil
	}

	provisionedThroughput := dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(l.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(l.WriteCapacityUnits),
	}

	// This defines the update event we send to DynamoDB.
	// This update sends an updated schema and an child event
	// to create the new global secondary index.
	c := dynamodb.UpdateTableInput{
		TableName:            aws.String(l.Tablename),
		AttributeDefinitions: tableSchema,
		GlobalSecondaryIndexUpdates: []*dynamodb.GlobalSecondaryIndexUpdate{
			{
				Create: &dynamodb.CreateGlobalSecondaryIndexAction{
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
		},
	}

	if _, err := l.svc.UpdateTable(&c); err != nil {
		return trace.Wrap(convertError(err))
	}

	// If we hit this time, we give up waiting.
	waitStart := time.Now()
	endWait := waitStart.Add(time.Minute * 10)

	// Wait until the index is created and active or updating.
	for time.Now().Before(endWait) {
		indexExists, err := l.indexExists(l.Tablename, indexTimeSearchV2)
		if err != nil {
			return trace.Wrap(err)
		}

		if indexExists {
			log.Info("DynamoDB index created")
			break
		}

		select {
		case <-time.After(time.Second * 5):
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}

		elapsed := time.Since(waitStart).Seconds()
		log.Infof("Creating new DynamoDB index, %f seconds elapsed...", elapsed)
	}

	return nil
}

// removeV1GSI removes the pre RFD 24 global secondary index from the table.
//
// Invariants:
// - This function must not be called concurrently with itself.
// - This may only be executed after the post RFD 24 global secondary index has been created.
func (l *Log) removeV1GSI() error {
	v1Exists, err := l.indexExists(l.Tablename, indexTimeSearch)
	if err != nil {
		return trace.Wrap(err)
	}

	if !v1Exists {
		log.Info("v1 event index already deleted.")
		return nil
	}

	c := dynamodb.UpdateTableInput{
		TableName: aws.String(l.Tablename),
		GlobalSecondaryIndexUpdates: []*dynamodb.GlobalSecondaryIndexUpdate{
			{
				Delete: &dynamodb.DeleteGlobalSecondaryIndexAction{
					IndexName: aws.String(indexTimeSearch),
				},
			},
		},
	}

	if _, err := l.svc.UpdateTable(&c); err != nil {
		return trace.Wrap(convertError(err))
	}

	return nil
}

// migrateDateAttribute walks existing events and calculates the value of the new `date`
// attribute and updates the event. This is needed by the new global secondary index
// schema introduced in RFD 24.
//
// This function is not atomic on error but safely interruptible.
// This means that the function may return an error without having processed
// all data but no residual temporary or broken data is left and
// the process can be resumed at any time by running this function again.
//
// Invariants:
// - This function must be called after `createV2GSI` has completed successfully on the table.
// - This function must not be called concurrently with itself.
// - The rfd24MigrationLock must be held by the node.
func (l *Log) migrateDateAttribute(ctx context.Context) error {
	var startKey map[string]*dynamodb.AttributeValue
	workerCounter := atomic.NewInt32(0)
	totalProcessed := atomic.NewInt32(0)
	workerErrors := make(chan error, maxMigrationWorkers)
	workerBarrier := sync.WaitGroup{}

	for {
		// Check for worker errors and escalate if found.
		select {
		case err := <-workerErrors:
			return trace.Wrap(err)
		default:
		}

		c := &dynamodb.ScanInput{
			ExclusiveStartKey: startKey,
			// Without consistent reads we may miss events as DynamoDB does not
			// specify a sufficiently short synchronisation grace period we can rely on instead.
			// This makes the scan operation slightly slower but the other alternative is scanning a second time
			// for any missed events after an appropriate grace period which is far worse.
			ConsistentRead: aws.Bool(true),
			// `DynamoBatchSize*maxMigrationWorkers` is the maximum concurrent event uploads.
			Limit:     aws.Int64(DynamoBatchSize * maxMigrationWorkers),
			TableName: aws.String(l.Tablename),
			// Without the `date` attribute.
			FilterExpression: aws.String("attribute_not_exists(CreatedAtDate)"),
		}

		// Resume the scan at the end of the previous one.
		// This processes `DynamoBatchSize*maxMigrationWorkers` events at maximum
		// which is why we need to run this multiple times on the dataset.
		scanOut, err := l.svc.Scan(c)
		if err != nil {
			return trace.Wrap(convertError(err))
		}

		writeRequests := make([]*dynamodb.WriteRequest, 0, DynamoBatchSize*maxMigrationWorkers)

		// For every item processed by this scan iteration we generate a write request.
		for _, item := range scanOut.Items {
			// Extract the UTC timestamp integer of the event.
			timestampAttribute := item[keyCreatedAt]
			var timestampRaw int64
			err = dynamodbattribute.Unmarshal(timestampAttribute, &timestampRaw)
			if err != nil {
				return trace.Wrap(err)
			}

			// Convert the timestamp into a date string of format `yyyy-mm-dd`.
			timestamp := time.Unix(timestampRaw, 0)
			date := timestamp.Format(iso8601DateFormat)

			dateAttribute, err := dynamodbattribute.Marshal(date)
			if err != nil {
				return trace.Wrap(err)
			}

			item[keyDate] = dateAttribute

			wr := &dynamodb.WriteRequest{
				PutRequest: &dynamodb.PutRequest{
					Item: item,
				},
			}

			writeRequests = append(writeRequests, wr)
		}

		for len(writeRequests) > 0 {
			var top int
			if len(writeRequests) > DynamoBatchSize {
				top = DynamoBatchSize
			} else {
				top = len(writeRequests)
			}

			// We need to make a copy of the slice here so it doesn't get changed later due to subslicing.
			batch := append(make([]*dynamodb.WriteRequest, 0, DynamoBatchSize), writeRequests[:top]...)
			writeRequests = writeRequests[top:]

			// Don't exceed maximum workers.
			for workerCounter.Load() >= maxMigrationWorkers {
				select {
				case <-time.After(time.Millisecond * 50):
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				}
			}

			workerCounter.Add(1)
			workerBarrier.Add(1)
			go func() {
				defer workerCounter.Sub(1)
				defer workerBarrier.Done()
				amountProcessed := len(batch)

				if err := l.uploadBatch(batch); err != nil {
					workerErrors <- trace.Wrap(err)
					return
				}

				total := totalProcessed.Add(int32(amountProcessed))
				log.Infof("Migrated %d total events to 6.2 format...", total)
			}()
		}

		// Setting the startKey to the last evaluated key of the previous scan so that
		// the next scan doesn't return processed events.
		startKey = scanOut.LastEvaluatedKey

		// If the `LastEvaluatedKey` field is not set we have finished scanning
		// the entire dataset and we can now break out of the loop.
		if scanOut.LastEvaluatedKey == nil {
			break
		}
	}

	// Wait until all upload tasks finish.
	workerBarrier.Wait()

	// Check for worker errors and escalate if found.
	select {
	case err := <-workerErrors:
		return trace.Wrap(err)
	default:
	}

	return nil
}

// uploadBatch creates or updates a batch of `DynamoBatchSize` events or less in one API call.
func (l *Log) uploadBatch(writeRequests []*dynamodb.WriteRequest) error {
	for {
		c := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]*dynamodb.WriteRequest{l.Tablename: writeRequests},
		}

		out, err := l.svc.BatchWriteItem(c)
		if err != nil {
			return trace.Wrap(err)
		}

		writeRequests = out.UnprocessedItems[l.Tablename]
		if len(writeRequests) == 0 {
			return nil
		}
	}
}

// createTable creates a DynamoDB table with a requested name and applies
// the back-end schema to it. The table must not exist.
//
// rangeKey is the name of the 'range key' the schema requires.
// currently is always set to "FullPath" (used to be something else, that's
// why it's a parameter for migration purposes)
func (l *Log) createTable(tableName string) error {
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
	_, err := l.svc.CreateTable(&c)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Waiting until table %q is created", tableName)
	err = l.svc.WaitUntilTableExists(&dynamodb.DescribeTableInput{
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
func (l *Log) deleteAllItems() error {
	out, err := l.svc.Scan(&dynamodb.ScanInput{TableName: aws.String(l.Tablename)})
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

		req, _ := l.svc.BatchWriteItemRequest(&dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]*dynamodb.WriteRequest{
				l.Tablename: chunk,
			},
		})
		err = req.Send()
		err = convertError(err)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// deleteTable deletes DynamoDB table with a given name
func (l *Log) deleteTable(tableName string, wait bool) error {
	tn := aws.String(tableName)
	_, err := l.svc.DeleteTable(&dynamodb.DeleteTableInput{TableName: tn})
	if err != nil {
		return trace.Wrap(err)
	}
	if wait {
		return trace.Wrap(
			l.svc.WaitUntilTableNotExists(&dynamodb.DescribeTableInput{TableName: tn}))
	}
	return nil
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
		return trace.AlreadyExists(aerr.Error())
	case dynamodb.ErrCodeProvisionedThroughputExceededException:
		return trace.ConnectionProblem(aerr, aerr.Error())
	case dynamodb.ErrCodeResourceNotFoundException:
		return trace.NotFound(aerr.Error())
	case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
		return trace.BadParameter(aerr.Error())
	case dynamodb.ErrCodeInternalServerError:
		return trace.BadParameter(aerr.Error())
	default:
		return err
	}
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise it is simply closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (l *Log) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	e <- trace.NotImplemented("not implemented")
	return c, e
}
