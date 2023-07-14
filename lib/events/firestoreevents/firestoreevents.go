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
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	apiv1 "cloud.google.com/go/firestore/apiv1/admin"
	"cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	firestorebk "github.com/gravitational/teleport/lib/backend/firestore"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/utils"
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

	// createdAtDocProperty is used internally to query for records and matches the key in the event struct tag
	createdAtDocProperty = "createdAt"

	// eventNamespaceDocProperty is used internally to query for records and matches the key in the event struct tag
	eventNamespaceDocProperty = "eventNamespace"

	// eventTypeDocProperty is the event struct tag associated with event type.
	eventTypeDocProperty = "eventType"

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
	err := metrics.RegisterPrometheusCollectors(prometheusCollectors...)
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
		go firestorebk.RetryingAsyncFunctionRunner(b.svcContext, retryutils.LinearConfig{
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
		sessionID = uuid.New().String()
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

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC).
//
// This function may never return more than 1 MiB of event data.
func (l *Log) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return l.searchEventsWithFilter(req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, searchEventsFilter{eventTypes: req.EventTypes}, "")
}

func (l *Log) searchEventsWithFilter(fromUTC, toUTC time.Time, namespace string, limit int, order types.EventOrder, lastKey string, filter searchEventsFilter, sessionID string) ([]apievents.AuditEvent, string, error) {
	g := l.WithFields(log.Fields{"From": fromUTC, "To": toUTC, "Namespace": namespace, "Filter": filter, "Limit": limit, "StartKey": lastKey})

	var values []events.EventFields
	var err error
	totalSize := 0
	var checkpointParts []string
	var checkpointTime int

	if lastKey != "" {
		checkpointParts = strings.Split(lastKey, ":")
		if len(checkpointParts) != 2 {
			return nil, "", trace.BadParameter("invalid checkpoint key: %q", lastKey)
		}

		checkpointTime, err = strconv.Atoi(checkpointParts[0])
		if err != nil {
			return nil, "", trace.BadParameter("invalid checkpoint key: %q", lastKey)
		}
	}

	var firestoreOrdering firestore.Direction
	switch order {
	case types.EventOrderAscending:
		firestoreOrdering = firestore.Asc
	case types.EventOrderDescending:
		firestoreOrdering = firestore.Desc
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}

	query := l.svc.Collection(l.CollectionName).
		Where(eventNamespaceDocProperty, "==", apidefaults.Namespace).
		Where(createdAtDocProperty, ">=", fromUTC.Unix()).
		Where(createdAtDocProperty, "<=", toUTC.Unix())

	if sessionID != "" {
		query = query.Where(sessionIDDocProperty, "==", sessionID)
	}

	if len(filter.eventTypes) > 0 {
		query = query.Where(eventTypeDocProperty, "in", filter.eventTypes)
	}

	query = query.OrderBy(createdAtDocProperty, firestoreOrdering).
		OrderBy(firestore.DocumentID, firestore.Asc)

	if lastKey != "" {
		query = query.StartAfter(checkpointTime, checkpointParts[1])
	}

	start := time.Now()
	docSnaps, err := query.Documents(l.svcContext).GetAll()
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err != nil {
		return nil, "", firestorebk.ConvertGRPCError(err)
	}

	g.WithFields(log.Fields{"duration": time.Since(start)}).Debugf("Query completed.")
	for _, docSnap := range docSnaps {
		var e event
		err = docSnap.DataTo(&e)
		if err != nil {
			return nil, "", firestorebk.ConvertGRPCError(err)
		}

		data := []byte(e.Fields)
		if totalSize+len(data) >= events.MaxEventBytesInResponse {
			break
		}

		var fields events.EventFields
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, "", trace.Errorf("failed to unmarshal event %v", err)
		}

		time := docSnap.Data()[createdAtDocProperty].(int64)
		lastKey = strconv.Itoa(int(time)) + ":" + docSnap.Ref.ID

		// Check that the filter condition is satisfied.
		if filter.condition != nil && !filter.condition(utils.Fields(fields)) {
			continue
		}

		values = append(values, fields)
		totalSize += len(data)
		if limit > 0 && len(values) >= limit {
			break
		}
	}

	if len(docSnaps) < limit {
		lastKey = ""
	}

	var toSort sort.Interface
	switch order {
	case types.EventOrderAscending:
		toSort = events.ByTimeAndIndex(values)
	case types.EventOrderDescending:
		toSort = sort.Reverse(events.ByTimeAndIndex(values))
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", order)
	}

	sort.Sort(toSort)
	eventArr := make([]apievents.AuditEvent, 0, len(values))
	for _, fields := range values {
		event, err := events.FromEventFields(fields)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		eventArr = append(eventArr, event)
	}

	return eventArr, lastKey, nil
}

// SearchSessionEvents returns session related events only. This is used to
// find completed sessions.
func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	filter := searchEventsFilter{eventTypes: []string{events.SessionEndEvent, events.WindowsDesktopSessionEndEvent}}
	if req.Cond != nil {
		condFn, err := utils.ToFieldsCondition(req.Cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}
	return l.searchEventsWithFilter(req.From, req.To, apidefaults.Namespace, req.Limit, req.Order, req.StartKey, filter, req.SessionID)
}

type searchEventsFilter struct {
	eventTypes []string
	condition  utils.FieldsCondition
}

func (l *Log) getIndexParent() string {
	return "projects/" + l.ProjectID + "/databases/(default)/collectionGroups/" + l.CollectionName
}

func (l *Log) ensureIndexes(adminSvc *apiv1.FirestoreAdminClient) error {
	tuples := firestorebk.IndexList{}
	tuples.Index(
		firestorebk.Field(eventNamespaceDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(createdAtDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(firestore.DocumentID, adminpb.Index_IndexField_ASCENDING),
	)
	tuples.Index(
		firestorebk.Field(eventNamespaceDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(createdAtDocProperty, adminpb.Index_IndexField_DESCENDING),
		firestorebk.Field(firestore.DocumentID, adminpb.Index_IndexField_ASCENDING),
	)
	tuples.Index(
		firestorebk.Field(eventNamespaceDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(eventTypeDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(createdAtDocProperty, adminpb.Index_IndexField_DESCENDING),
		firestorebk.Field(firestore.DocumentID, adminpb.Index_IndexField_ASCENDING),
	)
	tuples.Index(
		firestorebk.Field(eventNamespaceDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(eventTypeDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(sessionIDDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(createdAtDocProperty, adminpb.Index_IndexField_ASCENDING),
		firestorebk.Field(firestore.DocumentID, adminpb.Index_IndexField_ASCENDING),
	)
	err := firestorebk.EnsureIndexes(l.svcContext, adminSvc, tuples, l.getIndexParent())
	return trace.Wrap(err)
}

// Close the Firestore driver
func (l *Log) Close() error {
	l.svcCancel()
	return l.svc.Close()
}

func (l *Log) getDocIDForEvent(event event) string {
	return uuid.New().String()
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
			batch := l.svc.BulkWriter(l.svcContext)
			jobs := make([]*firestore.BulkWriterJob, 0, len(docSnaps))
			for _, docSnap := range docSnaps {
				job, err := batch.Delete(docSnap.Ref)
				if err != nil {
					return firestorebk.ConvertGRPCError(err)
				}

				jobs = append(jobs, job)
			}

			if len(jobs) == 0 {
				continue
			}

			start = time.Now()
			var errs []error
			batch.End()
			for _, job := range jobs {
				if _, err := job.Results(); err != nil {
					errs = append(errs, firestorebk.ConvertGRPCError(err))
				}
			}

			batchWriteLatencies.Observe(time.Since(start).Seconds())
			batchWriteRequests.Inc()
			return trace.NewAggregate(errs...)
		}
	}
}
