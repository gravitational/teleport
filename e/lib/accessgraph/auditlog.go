package accessgraph

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/export"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func initiateAndProcessAuditLogStream(ctx context.Context, log *slog.Logger, service accessgraphv1.AccessGraphServiceClient, authServer *auth.Server, config AuditLogConfig) error {
	auditLogStream, err := service.AuditLogStream(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create AuditLog stream", "error", err)
		return trace.Wrap(err)
	}
	clusterName, err := authServer.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err, "Failed to get cluster name for audit log export")
	}
	exporter := auditLogExporter{
		log:                 log,
		client:              authServer,
		stream:              auditLogStream,
		teleportClusterName: clusterName.GetClusterName(),
	}
	if err := exporter.start(ctx, config); err != nil {
		log.ErrorContext(ctx, "Error processing audit log stream", "error", err)
	}
	return nil
}

type auditLogExporter struct {
	// set by caller
	log                 *slog.Logger
	client              events.AuditLogSessionStreamer
	stream              auditLogStream
	teleportClusterName string

	// used by bulk exporter
	idleCh chan struct{}
	// batchRecvCh is used to receive event batches from the bulk exporter.
	// It ensures stream.Send() is not called concurrently, as the stream is not thread-safe
	// and may deadlock if multiple goroutines attempt to call Send() simultaneously.
	batchRecvCh chan eventsBatch
	activeDates []time.Time

	// used by search exporter
	startKey string
	lastID   string
}

func (a *auditLogExporter) start(ctx context.Context, config AuditLogConfig) error {
	setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)
	defer setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)

	a.log.DebugContext(ctx, "Starting stream processing")
	isBulkExporter, err := a.isBulkExporter(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	configPB, err := a.reconcileConfig(ctx, config)
	if err != nil {
		return trace.Wrap(err, "Failed to reconciled config for audit log export")
	}
	startDate := configPB.GetStartDate().AsTime()
	a.log.DebugContext(ctx, "Audit log stream parameters", "bulk_exporter", isBulkExporter, "effective_start_date", startDate)

	resumeState, err := a.getResumeState(ctx, isBulkExporter)
	if err != nil {
		return trace.Wrap(err, "Failed to retrieve audit log resume state")
	}
	setAccessGraphConnected(accessGraphMetricStreamAuditLog, true)

	if isBulkExporter {
		err = a.exportBulk(ctx, startDate, resumeState)
	} else {
		err = a.exportSearch(ctx, startDate, resumeState)
	}
	return trace.Wrap(err, "Failed to export audit log events on stream")
}

// isBulkExporter checks if the client implements the new bulk event export API
// by performing a fake request. This is done by querying the client to see if
// it implements the GetEventExportChunks method.
func (a *auditLogExporter) isBulkExporter(ctx context.Context) (bool, error) {
	chunks := a.client.GetEventExportChunks(ctx, auditlogv1.GetEventExportChunksRequest_builder{
		// target a date 2 days in the future to be confident that we're querying a valid but
		// empty date range, even in the context of reasonable clock drift.
		Date: timestamppb.New(time.Now().AddDate(0, 0, 2)),
	}.Build())

	if err := stream.Drain(chunks); err != nil {
		if trace.IsNotImplemented(err) {
			// fallback to search
			return false, nil
		}
		return false, trace.Wrap(err, "Failed to determined export type (bulk or search) in auditlog export")
	}

	return true, nil
}

// receiveUntilErr drains the stream's receive side (in) by repeatedly calling
// in.Recv() and discarding messages until an error occurs.
//
// This should be called after the send side has closed or errored (e.g., Send()
// returned io.EOF), as the definitive final stream error is often only
// revealed by Recv(). It returns the error from the first failing Recv() call
// (e.g., io.EOF for graceful closure, or a gRPC status error).
func receiveUntilErr(in auditLogStream) error {
	for {
		if _, err := in.Recv(); err != nil {
			return err
		}
	}
}

func (a *auditLogExporter) reconcileConfig(ctx context.Context, config AuditLogConfig) (*accessgraphv1.AuditLogConfig, error) {
	startDate := config.StartDate
	pbConfig := accessgraphv1.AuditLogConfig_builder{
		TeleportCluster: a.teleportClusterName,
	}.Build()
	if !startDate.IsZero() {
		pbConfig.SetStartDate(timestamppb.New(startDate))
	}
	req := accessgraphv1.AuditLogStreamRequest_builder{
		Config: proto.ValueOrDefault(pbConfig),
	}.Build()
	err := a.stream.Send(req)
	if err != nil {
		return nil, trace.Errorf("failed to send initial audit log config on stream. Send error %w, followed by receive error %w", err, receiveUntilErr(a.stream))
	}

	resp, err := a.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, "Failed to receive initial audit log response")
	}
	pbConfig = resp.GetAuditLogConfig()
	if pbConfig == nil {
		return nil, trace.Errorf("Expected response of type AuditLogConfig, got %T: %w", resp.GetState(), err)
	}
	persistedStartDate := pbConfig.GetStartDate().AsTime()
	if !startDate.IsZero() && startDate != persistedStartDate {
		const msg = "Audit Log config provided and persisted start date differ, using persisted. To fix remove start date from config."
		a.log.WarnContext(ctx, msg, "config_start_date", startDate, "persisted_start_date", pbConfig.GetStartDate().AsTime())
	}
	return pbConfig, nil
}

func (a *auditLogExporter) getResumeState(ctx context.Context, isBulkExporter bool) (*accessgraphv1.AuditLogStreamResponse, error) {
	resp, err := a.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, "Failed to receive second audit log response to track resume state")
	}
	switch resp.WhichState() {
	case accessgraphv1.AuditLogStreamResponse_NoResumeState_case, accessgraphv1.AuditLogStreamResponse_SearchResumeState_case: // new, resume or upgrade.
	case accessgraphv1.AuditLogStreamResponse_BulkResumeState_case:
		if !isBulkExporter {
			a.log.WarnContext(ctx, "Search exporter with bulk resume state, undefined behavior")
		}
	default:
		t := fmt.Sprintf("%T", resp.GetState())
		a.log.WarnContext(ctx, "Expected response State field of type NoResumeState, SearchResumeState or BulkResumeState", "state_type", t)
	}
	return resp, nil
}

// exportBulk exports the audit logs to grpc stream stream using the bulk event export API.
func (a *auditLogExporter) exportBulk(ctx context.Context, startDate time.Time, resumeState *accessgraphv1.AuditLogStreamResponse) error {
	a.log.DebugContext(ctx, "Starting audit log bulk exporting", "start_date", startDate, "resume_state", resumeState)
	a.idleCh = make(chan struct{}, 1)
	a.batchRecvCh = make(chan eventsBatch, 10)

	exporter, err := export.NewExporter(export.ExporterConfig{
		Client:        a.client,
		StartDate:     startDate,
		PreviousState: a.previousState(resumeState),
		BatchExport:   &export.BatchExportConfig{Callback: a.publishThroughChan},
		OnIdle:        a.sendIdleCh,
		Concurrency:   3, // TODO(juliaogris): Make configurable. Minimum should be 3. See: https://github.com/gravitational/teleport/blob/v17.3.3/integrations/event-handler/events_job.go#L156
	})
	if err != nil {
		return trace.Wrap(err, "Failed to create bulk exporter for audit log exports to access-graph")
	}
	defer exporter.Close()

	// pruneTicker start quickly while backfilling and slows down once idle
	pruneTicker := time.NewTicker(time.Minute)
	defer pruneTicker.Stop()
	firstIdleCall := true

	for {
		select {
		case batch := <-a.batchRecvCh:
			if err := a.sendBatch(ctx, batch.events, batch.resumeState); err != nil {
				return trace.Wrap(err, "Failed to send batch of events on audit log stream")
			}
		case <-pruneTicker.C:
			err := a.syncActiveDates(ctx, exporter.GetState())
			if err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "Context done for audit log bulk exporting")
		case <-a.idleCh:
			if firstIdleCall {
				err := a.syncActiveDates(ctx, exporter.GetState())
				if err != nil {
					return trace.Wrap(err)
				}
				a.log.DebugContext(ctx, "Switching to real-time audit log bulk exporting")
				pruneTicker.Reset(12 * time.Hour)
				firstIdleCall = false
			}
		}
	}
}

func (a *auditLogExporter) sendIdleCh(ctx context.Context) {
	select {
	case a.idleCh <- struct{}{}:
	case <-ctx.Done():
	}
}

func (a *auditLogExporter) previousState(resumeStatePB *accessgraphv1.AuditLogStreamResponse) export.ExporterState {
	bulkStatePB := resumeStatePB.GetBulkResumeState()
	if bulkStatePB == nil {
		searchStatePB := resumeStatePB.GetSearchResumeState()
		if searchStatePB == nil {
			return export.ExporterState{}
		}
		// Search state is not fully compatible with bulk state. Skip to the same target date as
		// tracked by the search state at 00:00:00 UTC.
		cutoverDate := searchStatePB.GetLastEventTime().AsTime()
		cutoverDate = cutoverDate.Truncate(24 * time.Hour)
		return export.ExporterState{
			Dates: map[time.Time]export.DateExporterState{
				cutoverDate: {},
			},
		}
	}

	datesPB := bulkStatePB.GetDates()
	state := export.ExporterState{
		Dates: map[time.Time]export.DateExporterState{},
	}
	for _, datePB := range datesPB {
		date := datePB.GetDate().AsTime()
		state.Dates[date] = export.DateExporterState{
			Completed: datePB.GetCompletedChunks(),
			Cursors:   datePB.GetChunkCursors(),
		}
	}
	return state
}

type eventsBatch struct {
	events      []*auditlogv1.EventUnstructured
	resumeState export.BulkExportResumeState
}

// publishThroughChan publishes a batch of events to the batchRecvCh channel.
func (b *auditLogExporter) publishThroughChan(ctx context.Context, events []*auditlogv1.EventUnstructured, resumeState export.BulkExportResumeState) error {
	select {
	case b.batchRecvCh <- eventsBatch{events: events, resumeState: resumeState}:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "Context done for audit log bulk exporting")
	}
}

// sendBatch sends a batch of events to the audit log stream.
func (a *auditLogExporter) sendBatch(ctx context.Context, events []*auditlogv1.EventUnstructured, resumeState export.BulkExportResumeState) error {
	a.log.DebugContext(ctx, "Sending bulk exported events", "event_count", len(events))
	err := a.stream.Send(accessgraphv1.AuditLogStreamRequest_builder{
		Events: accessgraphv1.AuditLogEvents_builder{
			Events: events,
			BulkResumeStateUpdate: accessgraphv1.BulkResumeStateUpdate_builder{
				Date:      timestamppb.New(resumeState.Date),
				Chunk:     resumeState.Chunk,
				Cursor:    resumeState.Cursor,
				Completed: resumeState.Completed,
			}.Build(),
		}.Build(),
	}.Build())
	if err != nil {
		return trace.Errorf("failed to send bulk export on audit log stream. Send error %w, followed by receive error %w", err, receiveUntilErr(a.stream))
	}
	return nil
}

func (a *auditLogExporter) syncActiveDates(ctx context.Context, newtState export.ExporterState) error {
	newActiveDates := slices.Collect(maps.Keys(newtState.Dates))
	slices.SortFunc(newActiveDates, time.Time.Compare)
	if slices.Equal(newActiveDates, a.activeDates) {
		return nil // We are up-to-date, no need to resend.
	}
	a.log.DebugContext(ctx, "Sending bulk sync for new active dates", "dates", newActiveDates)
	activeDatesPB := make([]*timestamppb.Timestamp, len(newActiveDates))
	for i, date := range newActiveDates {
		activeDatesPB[i] = timestamppb.New(date)
	}

	err := a.stream.Send(accessgraphv1.AuditLogStreamRequest_builder{
		BulkSync: accessgraphv1.BulkResumeStateSync_builder{
			ActiveDates: activeDatesPB,
		}.Build(),
	}.Build())
	if err != nil {
		return trace.Errorf("failed to send bulk sync request on audit log stream. Send error %w, followed by receive error %w", err, receiveUntilErr(a.stream))
	}
	a.activeDates = newActiveDates
	return nil
}

// exportSearch exports the audit logs to grpc stream stream using the SearchEvents API.
func (a *auditLogExporter) exportSearch(ctx context.Context, startDate time.Time, resumeState *accessgraphv1.AuditLogStreamResponse) error {
	a.log.DebugContext(ctx, "Starting search export")
	a.startKey = resumeState.GetSearchResumeState().GetStartKey()
	a.lastID = resumeState.GetSearchResumeState().GetLastEventId()

	for {
		unstructuredEvents, err := a.searchUnstructuredEvents(ctx, startDate)
		if err != nil {
			return trace.Wrap(err, "Failed to search events for audit log exporting")
		}
		if len(unstructuredEvents) == 0 { // we are caught up
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err(), "Context done for search event audit log exporting")
			case <-time.After(time.Minute):
				continue
			}
		}
		a.log.DebugContext(ctx, "Sending search events", "event_count", len(unstructuredEvents))
		req := accessgraphv1.AuditLogStreamRequest_builder{
			Events: accessgraphv1.AuditLogEvents_builder{
				Events: unstructuredEvents,
				SearchResumeState: accessgraphv1.SearchResumeState_builder{
					StartKey:    a.startKey,
					LastEventId: a.lastID,
					// LastEventTime: can be empty, inferred by the server from Events[-1].
				}.Build(),
			}.Build(),
		}.Build()
		err = a.stream.Send(req)
		if err != nil {
			return trace.Errorf("failed to send search export on audit log stream. Send error %w, followed by receive error %w", err, receiveUntilErr(a.stream))
		}
		if a.lastID != "" { // we have just sent a batch, but we are caught up because the lastID is set, so we don't have a new startKey
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err(), "Context done for search event audit log exporting")
			case <-time.After(time.Minute):
				continue
			}
		}
	}
}

func (a *auditLogExporter) searchUnstructuredEvents(ctx context.Context, startDate time.Time) ([]*auditlogv1.EventUnstructured, error) {
	req := events.SearchEventsRequest{
		From:     startDate,
		To:       time.Now().UTC(),
		Limit:    0,
		StartKey: a.startKey,
	}
	auditEvents, startKey, err := a.client.SearchEvents(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auditEvents, err = a.skipPastLastID(auditEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(auditEvents) == 0 {
		return nil, nil
	}
	unstructuredEvents, err := toUnstructuredEvents(auditEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a.updateSearchState(startKey, unstructuredEvents) // update startKey and lastID
	return unstructuredEvents, nil
}

func (a *auditLogExporter) skipPastLastID(events []apievents.AuditEvent) ([]apievents.AuditEvent, error) {
	if a.lastID == "" {
		return events, nil
	}
	for i, event := range events {
		eventID := event.GetID()
		if eventID == "" {
			unstructuredEvent, err := apievents.ToUnstructured(event)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			eventID = unstructuredEvent.GetId()
		}
		if eventID == a.lastID {
			return events[i+1:], nil
		}
	}
	return nil, trace.Errorf("lastID %q not found in search exporting", a.lastID)
}

func (a *auditLogExporter) updateSearchState(startKey string, events []*auditlogv1.EventUnstructured) {
	if startKey != "" {
		a.startKey = startKey
		a.lastID = ""
		return
	}

	// if we don't have a new start key, but still received new events track most recent event ID
	a.lastID = events[len(events)-1].GetId()
}

func toUnstructuredEvents(events []apievents.AuditEvent) ([]*auditlogv1.EventUnstructured, error) {
	unstructuredEvents := make([]*auditlogv1.EventUnstructured, len(events))
	for i, auditEvent := range events {
		unstructuredEvent, err := apievents.ToUnstructured(auditEvent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		unstructuredEvents[i] = unstructuredEvent
	}
	return unstructuredEvents, nil
}
