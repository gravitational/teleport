/*

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

package firestoreevents

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
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
)

func init() {
	prometheus.MustRegister(writeRequests)
	prometheus.MustRegister(batchWriteRequests)
	prometheus.MustRegister(batchReadRequests)
	prometheus.MustRegister(writeLatencies)
	prometheus.MustRegister(batchWriteLatencies)
	prometheus.MustRegister(batchReadLatencies)
}

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
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return trace.BadParameter("firestore: configuration is invalid: %v", err)
	} else {
		return nil
	}
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
	} else {
		cfg.ProjectID = projectIDParamString
	}

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
	} else {
		cfg.CollectionName = url.Host
	}

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
		EventNamespace: defaults.Namespace,
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
			EventNamespace: defaults.Namespace,
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

// SearchEvents is a flexible way to find  The format of a query string
// depends on the implementing backend. A recommended format is urlencoded
// (good enough for Lucene/Solr)
//
// Pagination is also defined via backend-specific query format.
//
// The only mandatory requirement is a date range (UTC). Results must always
// show up sorted by date (newest first)
func (l *Log) SearchEvents(fromUTC, toUTC time.Time, filter string, limit int) ([]events.EventFields, error) {
	g := l.WithFields(log.Fields{"From": fromUTC, "To": toUTC, "Filter": filter, "Limit": limit})
	filterVals, err := url.ParseQuery(filter)
	if err != nil {
		return nil, trace.BadParameter("missing or invalid parameter query in %q: %v", filter, err)
	}
	eventFilter, ok := filterVals[events.EventType]
	if !ok && len(filterVals) > 0 {
		return nil, nil
	}
	doFilter := len(eventFilter) > 0

	var values []events.EventFields

	start := time.Now()
	docSnaps, err := l.svc.Collection(l.CollectionName).
		Where(eventNamespaceDocProperty, "==", defaults.Namespace).
		Where(createdAtDocProperty, ">=", fromUTC.Unix()).
		Where(createdAtDocProperty, "<=", toUTC.Unix()).
		OrderBy(createdAtDocProperty, firestore.Asc).
		Limit(limit).
		Documents(l.svcContext).GetAll()
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err != nil {
		return nil, firestorebk.ConvertGRPCError(err)
	}

	g.WithFields(log.Fields{"duration": time.Since(start)}).Debugf("Query completed.")
	for _, docSnap := range docSnaps {

		var e event
		err = docSnap.DataTo(&e)
		if err != nil {
			return nil, firestorebk.ConvertGRPCError(err)
		}

		var fields events.EventFields
		data := []byte(e.Fields)
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, trace.Errorf("failed to unmarshal event %v", err)
		}
		var accepted bool
		for i := range eventFilter {
			if fields.GetString(events.EventType) == eventFilter[i] {
				accepted = true
				break
			}
		}
		if accepted || !doFilter {
			values = append(values, fields)
			if limit > 0 && len(values) >= limit {
				break
			}
		}
	}
	sort.Sort(events.ByTimeAndIndex(values))
	return values, nil
}

// SearchSessionEvents returns session related events only. This is used to
// find completed session.
func (l *Log) SearchSessionEvents(fromUTC time.Time, toUTC time.Time, limit int) ([]events.EventFields, error) {
	// only search for specific event types
	query := url.Values{}
	query[events.EventType] = []string{
		events.SessionStartEvent,
		events.SessionEndEvent,
	}
	return l.SearchEvents(fromUTC, toUTC, query.Encode(), limit)
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
		FirstField:  eventNamespaceDocProperty,
		SecondField: createdAtDocProperty,
	})
	tuples = append(tuples, &firestorebk.IndexTuple{
		FirstField:  sessionIDDocProperty,
		SecondField: eventIndexDocProperty,
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
	return event.SessionID + "-" + event.EventType
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
