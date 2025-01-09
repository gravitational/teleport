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

package firestoreevents

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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
	"google.golang.org/api/iterator"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
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

	// batchReadLimit is the maximum number of documents to read in a single batch
	batchReadLimit = 2000
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

	cfg.DatabaseID = url.Query().Get("databaseID")

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
	// logger emits log messages
	logger *slog.Logger
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

	closeCtx, cancel := context.WithCancel(context.Background())
	l := slog.With(teleport.ComponentKey, teleport.ComponentFirestore)
	l.InfoContext(closeCtx, "Initializing event backend.")
	firestoreAdminClient, firestoreClient, err := firestorebk.CreateFirestoreClients(closeCtx, cfg.ProjectID, cfg.DatabaseID, cfg.EndPoint, cfg.CredentialsPath)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	defer firestoreAdminClient.Close()
	b := &Log{
		svcContext:   closeCtx,
		svcCancel:    cancel,
		logger:       l,
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
		}, b.logger.With("task_name", "purge_expired_events"), b.purgeExpiredEvents)
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
	_, err = l.svc.Collection(l.CollectionName).Doc(l.getDocIDForEvent()).Create(l.svcContext, event)
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
	return l.searchEventsWithFilter(
		ctx,
		searchEventsWithFilterParams{
			fromUTC:   req.From,
			toUTC:     req.To,
			namespace: apidefaults.Namespace,
			limit:     req.Limit,
			order:     req.Order,
			lastKey:   req.StartKey,
			filter:    searchEventsFilter{eventTypes: req.EventTypes},
			sessionID: "",
		})
}

type searchEventsWithFilterParams struct {
	fromUTC, toUTC time.Time
	namespace      string
	limit          int
	order          types.EventOrder
	lastKey        string
	filter         searchEventsFilter
	sessionID      string
}

func (l *Log) searchEventsWithFilter(ctx context.Context, params searchEventsWithFilterParams) ([]apievents.AuditEvent, string, error) {
	if params.limit <= 0 {
		params.limit = batchReadLimit
	}

	g := l.logger.With(
		"from", params.fromUTC,
		"to", params.toUTC,
		"namespace", params.namespace,
		"filter", params.filter,
		"limit", params.limit,
		"start_key", params.lastKey,
	)

	var firestoreOrdering firestore.Direction
	switch params.order {
	case types.EventOrderAscending:
		firestoreOrdering = firestore.Asc
	case types.EventOrderDescending:
		firestoreOrdering = firestore.Desc
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", params.order)
	}

	query := l.svc.Collection(l.CollectionName).
		Where(eventNamespaceDocProperty, "==", apidefaults.Namespace).
		Where(createdAtDocProperty, ">=", params.fromUTC.Unix()).
		Where(createdAtDocProperty, "<=", params.toUTC.Unix())

	if params.sessionID != "" {
		query = query.Where(sessionIDDocProperty, "==", params.sessionID)
	}

	if len(params.filter.eventTypes) > 0 {
		query = query.Where(eventTypeDocProperty, "in", params.filter.eventTypes)
	}

	query = query.OrderBy(createdAtDocProperty, firestoreOrdering).
		OrderBy(firestore.DocumentID, firestore.Asc).
		Limit(params.limit)

	values, lastKey, err := l.query(ctx, query, params.lastKey, params.filter, params.limit, g)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	var toSort sort.Interface
	switch params.order {
	case types.EventOrderAscending:
		toSort = events.ByTimeAndIndex(values)
	case types.EventOrderDescending:
		toSort = sort.Reverse(events.ByTimeAndIndex(values))
	default:
		return nil, "", trace.BadParameter("invalid event order: %v", params.order)
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

func (l *Log) query(
	ctx context.Context,
	query firestore.Query,
	lastKey string,
	filter searchEventsFilter,
	limit int,
	g *slog.Logger,
) (values []events.EventFields, _ string, err error) {
	var (
		checkpointTime int64
		docID          string
		totalSize      int
	)
	if lastKey != "" {
		checkpointParts := strings.Split(lastKey, ":")
		if len(checkpointParts) != 2 {
			return nil, "", trace.BadParameter("invalid checkpoint key: %q", lastKey)
		}

		checkpointTime, err = strconv.ParseInt(checkpointParts[0], 10, 64)
		if err != nil {
			return nil, "", trace.BadParameter("invalid checkpoint key: %q", lastKey)
		}
		docID = checkpointParts[1]
	}

	for {
		if lastKey != "" {
			query = query.StartAfter(checkpointTime, docID)
		}
		start := time.Now()
		fstoreIterator := query.Documents(ctx)
		defer fstoreIterator.Stop()

		batchReadLatencies.Observe(time.Since(start).Seconds())
		batchReadRequests.Inc()
		if err != nil {
			return nil, "", firestorebk.ConvertGRPCError(err)
		}

		g.DebugContext(ctx, "Query completed.", "duration", time.Since(start))

		// Iterate over the documents in the query.
		// The iterator is limited to [limit] documents so in order to know if we
		// have more pages to read when filtering, we can read only [limit] documents.
		for i := 0; i < limit; i++ {
			docSnap, err := fstoreIterator.Next()
			if errors.Is(err, iterator.Done) {
				// iterator.Done is returned when there are no more documents to read.
				// In this case, return the events collected so far and an empty last key
				// to indicate that the query is complete.
				return values, "", nil
			} else if err != nil {
				return nil, "", firestorebk.ConvertGRPCError(err)
			}

			var e event
			if err := docSnap.DataTo(&e); err != nil {
				return nil, "", firestorebk.ConvertGRPCError(err)
			}

			data := []byte(e.Fields)
			var fields events.EventFields
			if err := json.Unmarshal(data, &fields); err != nil {
				return nil, "", trace.Errorf("failed to unmarshal event %v", err)
			}

			// if the total size of the events exceeds the limit, return the events
			// collected so far and the last key to resume the query.
			if totalSize+len(data) >= events.MaxEventBytesInResponse {
				return values, lastKey, nil
			}

			checkpointTime = docSnap.Data()[createdAtDocProperty].(int64)
			docID = docSnap.Ref.ID
			lastKey = strconv.FormatInt(checkpointTime, 10) + ":" + docID

			// Check that the filter condition is satisfied.
			if filter.condition != nil && !filter.condition(utils.Fields(fields)) {
				continue
			}

			values = append(values, fields)
			totalSize += len(data)
			if limit > 0 && len(values) >= limit {
				return values, lastKey, nil
			}
		}
	}
}

// SearchSessionEvents returns session related events only. This is used to
// find completed sessions.
func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	filter := searchEventsFilter{eventTypes: events.SessionRecordingEvents}
	if req.Cond != nil {
		condFn, err := utils.ToFieldsCondition(req.Cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}
	return l.searchEventsWithFilter(
		ctx,
		searchEventsWithFilterParams{
			fromUTC:   req.From,
			toUTC:     req.To,
			namespace: apidefaults.Namespace,
			limit:     req.Limit,
			order:     req.Order,
			lastKey:   req.StartKey,
			filter:    filter,
			sessionID: req.SessionID,
		},
	)
}

func (l *Log) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotImplemented("firestoreevents backend does not support streaming export"))
}

func (l *Log) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	return stream.Fail[*auditlogpb.EventExportChunk](trace.NotImplemented("firestoreevents backend does not support streaming export"))
}

type searchEventsFilter struct {
	eventTypes []string
	condition  utils.FieldsCondition
}

func (l *Log) getIndexParent() string {
	database := cmp.Or(l.Config.DatabaseID, "(default)")
	return "projects/" + l.ProjectID + "/databases/" + database + "/collectionGroups/" + l.CollectionName
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
	err := firestorebk.EnsureIndexes(l.svcContext, adminSvc, l.logger, tuples, l.getIndexParent())
	return trace.Wrap(err)
}

// Close the Firestore driver
func (l *Log) Close() error {
	l.svcCancel()
	return l.svc.Close()
}

func (l *Log) getDocIDForEvent() string {
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
