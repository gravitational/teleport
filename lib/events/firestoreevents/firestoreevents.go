// Copyright 2021 Gravitational, Inc
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

package firestoreevents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/firestore/admin/v1"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	firestorebk "github.com/gravitational/teleport/lib/backend/firestore"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"cloud.google.com/go/firestore"

	apiv1 "cloud.google.com/go/firestore/apiv1/admin"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

var (
	writeRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "firestore_events_backend_write_requests",
			Help: "Number of write requests to firestore events",
		},
	)
	batchWriteRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "firestore_events_backend_batch_write_requests",
			Help: "Number of batch write requests to firestore events",
		},
	)
	batchReadRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "firestore_events_backend_batch_read_requests",
			Help: "Number of batch read requests to firestore events",
		},
	)
	writeLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "firestore_events_backend_write_seconds",
			Help: "Latency for firestore events write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	batchWriteLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "firestore_events_backend_batch_write_seconds",
			Help: "Latency for firestore events batch write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	batchReadLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "firestore_events_backend_batch_read_seconds",
			Help: "Latency for firestore events batch read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)

	prometheusCollectors = []prometheus.Collector{
		writeRequests, batchWriteRequests, batchReadRequests,
		writeLatencies, batchWriteLatencies, batchReadLatencies,
	}
)

const (

	// defaultEventRetentionPeriod is the duration applied to time the ticker executes scrutinizing records for purging
	defaultEventRetentionPeriod = time.Hour * 8766

	// defaultPurgeInterval is the interval for the ticker that executes the expired record query and cleanup
	defaultPurgeInterval = time.Hour * 24

	// purgeIntervalPropertyKey is a property key used for URI param extraction
	purgeIntervalPropertyKey = "purgeInterval"

	// eventRetentionPeriodPropertyKey is a property key used for URI param extraction
	eventRetentionPeriodPropertyKey = "eventRetentionPeriod"

	// retryPeriodPropertyKey is a property key used for URI param extraction
	retryPeriodPropertyKey = "retryPeriod"

	// disableExpiredDocumentPurgePropertyKey is a property key used for URI param extraction
	disableExpiredDocumentPurgePropertyKey = "disableExpiredDocumentPurge"

	endpointPropertyKey = "endpoint"

	// sessionIDDocProperty is used internally to query for records and matches the key in the event struct tag
	sessionIDDocProperty = "sessionID"

	// eventIndexDocProperty is used internally to query for records and matches the key in the event struct tag
	eventIndexDocProperty = "eventIndex"

	// createdAtDocProperty is used internally to query for records and matches the key in the event struct tag
	createdAtDocProperty = "createdAt"

	// eventNamespaceDocProperty is used internally to query for records and matches the key in the event struct tag
	eventNamespaceDocProperty = "eventNamespace"

	// credentialsPath is used to supply credentials to teleport via JSON-typed service account key file
	credentialsPath = "credentialsPath"

	// projectID is used to to lookup firestore resources for a given GCP project
	projectID = "projectID"
)

// Config structure represents Firestore configuration as appears in `storage` section
// of Teleport YAML
type EventsConfig struct {
	firestorebk.Config
	// RetentionPeriod is a default retention period for events
	RetentionPeriod time.Duration
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator
	UIDGenerator utils.UID
}

// SetFromParams establishes values on an EventsConfig from the supplied params
func (cfg *EventsConfig) SetFromParams(params backend.Params) error {
	err := apiutils.ObjectToStruct(params, &cfg)
	if err != nil {
		return trace.BadParameter("firestore: configuration is invalid: %v", err)
	}
	return nil
}

// SetFromURL establishes values on an EventsConfig from the supplied URI
func (cfg *EventsConfig) SetFromURL(url *url.URL) error {
	disableExpiredDocumentPurgeParamString := url.Query().Get(disableExpiredDocumentPurgePropertyKey)
	if disableExpiredDocumentPurgeParamString == "" {
		cfg.DisableExpiredDocumentPurge = false
	} else {
		disableExpiredDocumentPurge, err := strconv.ParseBool(disableExpiredDocumentPurgeParamString)
		if err != nil {
			return trace.BadParameter("parameter %s with value '%s' is invalid: %v", disableExpiredDocumentPurgePropertyKey, disableExpiredDocumentPurgeParamString, err)
		}
		cfg.DisableExpiredDocumentPurge = disableExpiredDocumentPurge
	}

	endpointParamString := url.Query().Get(endpointPropertyKey)
	if len(endpointParamString) > 0 {
		cfg.EndPoint = endpointParamString
	}

	credentialsPathParamString := url.Query().Get(credentialsPath)
	if len(credentialsPathParamString) > 0 {
		cfg.CredentialsPath = credentialsPathParamString
	}

	projectIDParamString := url.Query().Get(projectID)
	if projectIDParamString == "" {
		return trace.BadParameter("parameter %s with value '%s' is invalid",
			projectID, projectIDParamString)
	}
	cfg.ProjectID = projectIDParamString

	eventRetentionPeriodParamString := url.Query().Get(eventRetentionPeriodPropertyKey)
	if eventRetentionPeriodParamString == "" {
		cfg.RetentionPeriod = defaultEventRetentionPeriod
	} else {
		eventRetentionPeriod, err := time.ParseDuration(eventRetentionPeriodParamString)
		if err != nil {
			return trace.BadParameter("parameter %s with value '%s' is invalid: %v", eventRetentionPeriodPropertyKey, eventRetentionPeriodParamString, err)
		}
		cfg.RetentionPeriod = eventRetentionPeriod
	}

	retryPeriodParamString := url.Query().Get(retryPeriodPropertyKey)
	if retryPeriodParamString == "" {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	} else {
		retryPeriodParamString, err := time.ParseDuration(retryPeriodParamString)
		if err != nil {
			return trace.BadParameter("parameter %s with value '%s' is invalid: %v", retryPeriodPropertyKey, retryPeriodParamString, err)
		}
		cfg.RetryPeriod = retryPeriodParamString
	}

	purgeIntervalParamString := url.Query().Get(purgeIntervalPropertyKey)
	if purgeIntervalParamString == "" {
		cfg.PurgeExpiredDocumentsPollInterval = defaultPurgeInterval
	} else {
		purgeInterval, err := time.ParseDuration(purgeIntervalParamString)
		if err != nil {
			return trace.BadParameter("parameter %s with value '%s' is invalid: %v", purgeIntervalPropertyKey, purgeIntervalParamString, err)
		}
		cfg.PurgeExpiredDocumentsPollInterval = purgeInterval
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UIDGenerator == nil {
		cfg.UIDGenerator = utils.NewRealUID()
	}

	if url.Host == "" {
		return trace.BadParameter("host should be set to the collection name for event storage")
	}
	cfg.CollectionName = url.Host

	return nil
}

// Log is a firestore-db backed storage of events
type Log struct {
	// Entry is a log entry
	*log.Entry
	// Config is a backend configuration
	EventsConfig
	// svc is the primary Firestore client
	svc *firestore.Client
	// svcContext is passed two both Firestore clients and used to cleanup resources on teardown
	svcContext context.Context
	// svcCancel cancels the root context for the firestore clients
	svcCancel context.CancelFunc
}

type event struct {
	SessionID      string `firestore:"sessionID,omitempty"`
	EventIndex     int64  `firestore:"eventIndex,omitempty"`
	EventType      string `firestore:"eventType,omitempty"`
	CreatedAt      int64  `firestore:"createdAt,omitempty"`
	Fields         string `firestore:"fields,omitempty"`
	EventNamespace string `firestore:"eventNamespace,omitempty"`
}

// New returns new instance of Firestore backend.
// It's an implementation of backend API's NewFunc
func New(cfg EventsConfig) (*Log, error) {
	err := utils.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := log.WithFields(log.Fields{
		trace.Component: teleport.Component(teleport.ComponentFirestore),
	})
	l.Info("Initializing event backend.")
	closeCtx, cancel := context.WithCancel(context.Background())
	firestoreAdminClient, firestoreClient, err := firestorebk.CreateFirestoreClients(closeCtx, cfg.ProjectID, cfg.EndPoint, cfg.CredentialsPath)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	defer firestoreAdminClient.Close()
	b := &Log{
		svcContext:   closeCtx,
		svcCancel:    cancel,
		Entry:        l,
		EventsConfig: cfg,
		svc:          firestoreClient,
	}

	if len(cfg.EndPoint) == 0 {
		err = b.ensureIndexes(firestoreAdminClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if !b.DisableExpiredDocumentPurge {
		go firestorebk.RetryingAsyncFunctionRunner(b.svcContext, utils.LinearConfig{
			Step: b.RetryPeriod / 10,
			Max:  b.RetryPeriod,
		}, b.Logger, b.purgeExpiredEvents, "purgeExpiredEvents")
	}
	return b, nil
}

// EmitAuditEvent emits audit event
func (l *Log) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
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

	event := event{
		SessionID:      sessionID,
		EventIndex:     in.GetIndex(),
		EventType:      in.GetType(),
		EventNamespace: apidefaults.Namespace,
		CreatedAt:      in.GetTime().Unix(),
		Fields:         string(data),
	}
	start := time.Now()
	_, err = l.svc.Collection(l.CollectionName).Doc(l.getDocIDForEvent(event)).Create(l.svcContext, event)
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return firestorebk.ConvertGRPCError(err)
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
	event := event{
		SessionID:      sessionID,
		EventIndex:     int64(eventIndex),
		EventType:      fields.GetString(events.EventType),
		EventNamespace: apidefaults.Namespace,
		CreatedAt:      created.Unix(),
		Fields:         string(data),
	}
	start := time.Now()
	_, err = l.svc.Collection(l.CollectionName).Doc(l.getDocIDForEvent(event)).Create(l.svcContext, event)
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return firestorebk.ConvertGRPCError(err)
	}
	return nil
}

// PostSessionSlice sends chunks of recorded session to the event log
func (l *Log) PostSessionSlice(slice events.SessionSlice) error {
	batch := l.svc.Batch()
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
		event := event{
			SessionID:      slice.SessionID,
			EventNamespace: apidefaults.Namespace,
			EventType:      chunk.EventType,
			EventIndex:     chunk.EventIndex,
			CreatedAt:      time.Unix(0, chunk.Time).In(time.UTC).Unix(),
			Fields:         string(data),
		}
		batch.Create(l.svc.Collection(l.CollectionName).Doc(l.getDocIDForEvent(event)), event)
	}
	start := time.Now()
	_, err := batch.Commit(l.svcContext)
	batchWriteLatencies.Observe(time.Since(start).Seconds())
	batchWriteRequests.Inc()
	if err != nil {
		return firestorebk.ConvertGRPCError(err)
	}
	return nil
}

func (l *Log) UploadSessionRecording(events.SessionRecording) error {
	return trace.NotImplemented("UploadSessionRecording not implemented for firestore backend")
}

// GetSessionChunk returns a reader which can be used to read a byte stream
// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
// beginning) up to maxBytes bytes.
//
// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
func (l *Log) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, trace.NotImplemented("GetSessionChunk not implemented for firestore backend")
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
	start := time.Now()
	docSnaps, err := l.svc.Collection(l.CollectionName).Where(sessionIDDocProperty, "==", string(sid)).
		Documents(l.svcContext).GetAll()
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err != nil {
		return nil, firestorebk.ConvertGRPCError(err)
	}
	for _, docSnap := range docSnaps {
		var e event
		err := docSnap.DataTo(&e)
		if err != nil {
			return nil, firestorebk.ConvertGRPCError(err)
		}
		var fields events.EventFields
		data := []byte(e.Fields)
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, trace.Errorf("failed to unmarshal event for session %q: %v", string(sid), err)
		}
		values = append(values, fields)
	}
	sort.Sort(events.ByTimeAndIndex(values))
	return values, nil
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
	return l.searchEventsWithFilter(fromUTC, toUTC, namespace, limit, order, startKey, searchEventsFilter{eventTypes: eventTypes})
}

func (l *Log) searchEventsWithFilter(fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startKey string, filter searchEventsFilter) ([]apievents.AuditEvent, string, error) {
	var eventsArr []apievents.AuditEvent
	var estimatedSize int
	checkpoint := startKey
	left := limit

	for {
		gotEvents, withSize, withCheckpoint, err := l.searchEventsOnce(fromUTC, toUTC, namespace, left, order, checkpoint, filter, events.MaxEventBytesInResponse-estimatedSize)
		if nil != err {
			return nil, "", trace.Wrap(err)
		}

		eventsArr = append(eventsArr, gotEvents...)
		estimatedSize += withSize
		left -= len(gotEvents)
		checkpoint = withCheckpoint

		if len(checkpoint) == 0 || left <= 0 || estimatedSize >= events.MaxEventBytesInResponse {
			break
		}
	}

	return eventsArr, checkpoint, nil
}

func (l *Log) searchEventsOnce(fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, startKey string, filter searchEventsFilter, spaceRemaining int) ([]apievents.AuditEvent, int, string, error) {
	g := l.WithFields(log.Fields{"From": fromUTC, "To": toUTC, "Namespace": namespace, "Filter": filter, "Limit": limit, "StartKey": startKey})

	var lastKey int64
	var values []events.EventFields
	var parsedStartKey int64
	var err error
	reachedEnd := false
	totalSize := 0

	if startKey != "" {
		parsedStartKey, err = strconv.ParseInt(startKey, 10, 64)
		if err != nil {
			return nil, 0, "", trace.WrapWithMessage(err, "failed to parse startKey, expected integer but found: %q", startKey)
		}
	}

	modifyquery := func(query firestore.Query) firestore.Query {
		if startKey != "" {
			return query.StartAfter(parsedStartKey)
		}

		return query
	}

	var firestoreOrdering firestore.Direction
	switch order {
	case types.EventOrderAscending:
		firestoreOrdering = firestore.Asc
	case types.EventOrderDescending:
		firestoreOrdering = firestore.Desc
	default:
		return nil, 0, "", trace.BadParameter("invalid event order: %v", order)
	}

	start := time.Now()
	docSnaps, err := modifyquery(l.svc.Collection(l.CollectionName).
		Where(eventNamespaceDocProperty, "==", apidefaults.Namespace).
		Where(createdAtDocProperty, ">=", fromUTC.Unix()).
		Where(createdAtDocProperty, "<=", toUTC.Unix()).
		OrderBy(createdAtDocProperty, firestoreOrdering)).
		Limit(limit).
		Documents(l.svcContext).GetAll()
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err != nil {
		return nil, 0, "", firestorebk.ConvertGRPCError(err)
	}

	// Correctly detecting if you've reached the end of a query in firestore is
	// tricky since it doesn't set any flag when it finds that there are no further events.
	// This solution here seems to be the most common, but I haven't been able to find
	// any documented hard guarantees on firestore not early returning for some reason like response
	// size like DynamoDB does. In short, this should work in all cases for lack of a better solution.
	if len(docSnaps) < limit {
		reachedEnd = true
	}

	g.WithFields(log.Fields{"duration": time.Since(start)}).Debugf("Query completed.")
	for _, docSnap := range docSnaps {
		var e event
		err = docSnap.DataTo(&e)
		if err != nil {
			return nil, 0, "", firestorebk.ConvertGRPCError(err)
		}

		accepted := len(filter.eventTypes) == 0
		for _, eventType := range filter.eventTypes {
			if e.EventType == eventType {
				accepted = true
				break
			}
		}
		if !accepted {
			continue
		}

		data := []byte(e.Fields)
		var fields events.EventFields
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, 0, "", trace.Errorf("failed to unmarshal event %v", err)
		}
		// Check that the filter condition is satisfied.
		if filter.condition != nil {
			accepted = accepted && filter.condition(fields)
		}

		if accepted {
			if totalSize+len(data) >= spaceRemaining {
				break
			}
			lastKey = docSnap.Data()["createdAt"].(int64)
			values = append(values, fields)
			totalSize += len(data)
			if limit > 0 && len(values) >= limit {
				break
			}
		}
	}

	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = events.ByTimeAndIndex(values)
	case types.EventOrderDescending:
		toSort = sort.Reverse(events.ByTimeAndIndex(values))
	default:
		return nil, 0, "", trace.BadParameter("invalid event order: %v", order)
	}
	sort.Sort(toSort)

	eventArr := make([]apievents.AuditEvent, 0, len(values))
	for _, fields := range values {
		event, err := events.FromEventFields(fields)
		if err != nil {
			return nil, 0, "", trace.Wrap(err)
		}
		eventArr = append(eventArr, event)
	}

	var lastKeyString string
	if lastKey != 0 && !reachedEnd {
		lastKeyString = fmt.Sprintf("%d", lastKey)
	}

	return eventArr, totalSize, lastKeyString, nil
}

// SearchSessionEvents returns session related events only. This is used to
// find completed sessions.
func (l *Log) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) ([]apievents.AuditEvent, string, error) {
	filter := searchEventsFilter{eventTypes: []string{events.SessionEndEvent}}
	if cond != nil {
		condFn, err := events.ToEventFieldsCondition(cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}
	return l.searchEventsWithFilter(fromUTC, toUTC, apidefaults.Namespace, limit, order, startKey, filter)
}

type searchEventsFilter struct {
	eventTypes []string
	condition  events.EventFieldsCondition
}

// WaitForDelivery waits for resources to be released and outstanding requests to
// complete after calling Close method
func (l *Log) WaitForDelivery(ctx context.Context) error {
	return nil
}

func (l *Log) getIndexParent() string {
	return "projects/" + l.ProjectID + "/databases/(default)/collectionGroups/" + l.CollectionName
}

func (l *Log) ensureIndexes(adminSvc *apiv1.FirestoreAdminClient) error {
	tuples := make([]*firestorebk.IndexTuple, 0)
	tuples = append(tuples, &firestorebk.IndexTuple{
		FirstField:       eventNamespaceDocProperty,
		SecondField:      createdAtDocProperty,
		SecondFieldOrder: admin.Index_IndexField_ASCENDING,
	})
	tuples = append(tuples, &firestorebk.IndexTuple{
		FirstField:       eventNamespaceDocProperty,
		SecondField:      createdAtDocProperty,
		SecondFieldOrder: admin.Index_IndexField_DESCENDING,
	})
	tuples = append(tuples, &firestorebk.IndexTuple{
		FirstField:       sessionIDDocProperty,
		SecondField:      eventIndexDocProperty,
		SecondFieldOrder: admin.Index_IndexField_ASCENDING,
	})
	err := firestorebk.EnsureIndexes(l.svcContext, adminSvc, tuples, l.getIndexParent())
	return err
}

// Close the Firestore driver
func (l *Log) Close() error {
	l.svcCancel()
	return l.svc.Close()
}

func (l *Log) getDocIDForEvent(event event) string {
	return uuid.New()
}

func (l *Log) purgeExpiredEvents() error {
	t := time.NewTicker(l.PurgeExpiredDocumentsPollInterval)
	defer t.Stop()
	for {
		select {
		case <-l.svcContext.Done():
			return nil
		case <-t.C:
			expiryTime := l.Clock.Now().UTC().Add(-1 * l.RetentionPeriod)
			start := time.Now()
			docSnaps, err := l.svc.Collection(l.CollectionName).Where(createdAtDocProperty, "<=", expiryTime.Unix()).Documents(l.svcContext).GetAll()
			batchReadLatencies.Observe(time.Since(start).Seconds())
			batchReadRequests.Inc()
			if err != nil {
				return firestorebk.ConvertGRPCError(err)
			}
			numDeleted := 0
			batch := l.svc.Batch()
			for _, docSnap := range docSnaps {
				batch.Delete(docSnap.Ref)
				numDeleted++
			}
			if numDeleted > 0 {
				start = time.Now()
				_, err := batch.Commit(l.svcContext)
				batchWriteLatencies.Observe(time.Since(start).Seconds())
				batchWriteRequests.Inc()
				if err != nil {
					return firestorebk.ConvertGRPCError(err)
				}
			}
		}
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
