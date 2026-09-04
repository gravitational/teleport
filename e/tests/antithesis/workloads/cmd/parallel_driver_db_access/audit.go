package main

import (
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apievents "github.com/gravitational/teleport/api/types/events"
	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
	"github.com/gravitational/teleport/lib/events"
)

func (p *TestCaseParams) assertDatabaseAccessAuditEvents(ctx context.Context, marker string, route proto.RouteToDatabase, start, end time.Time) error {
	const adminIdentity = "admin"
	admin, err := workloadclient.NewAPIClient(ctx, adminIdentity)
	if err != nil {
		return trace.Wrap(err, "creating api client")
	}
	defer admin.Close()

	details := p.Details(map[string]any{
		"marker": marker,
		"route":  route,
		"from":   start,
		"to":     end,
	})

	var discoveredSID string
	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Database session query event exists with the expected value.",
		Timeout: testenv.AuditEventEmitDeadline,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			for evt, err := range workloadclient.RangeAllAuditEventsByType(
				ctx,
				admin,
				start,
				end,
				events.DatabaseSessionQueryEvent,
			) {
				if err != nil {
					return false, trace.Wrap(err, "reading events")
				}
				e, ok := evt.(*apievents.DatabaseSessionQuery)
				if !ok || !databaseMetadataMatches(e.DatabaseMetadata, route) {
					continue
				}
				if !databaseQueryContainsMarker(e, marker) {
					continue
				}
				discoveredSID = e.SessionMetadata.SessionID
				addDetail("discovered_session_id", discoveredSID)
				addDetail("event_query", e.DatabaseQuery)
				return discoveredSID != "", nil
			}
			return false, nil
		},
	})
	if err != nil {
		return trace.Wrap(err, "discovering database session id")
	}

	return trace.Wrap(eventually.Assert(ctx, eventually.AssertParams{
		Message: "Successful PostgreSQL database access has matching db.session.start, db.session.query and db.session.end audit events",
		Timeout: testenv.AuditEventEmitDeadline,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			eventmap := map[string]int{}
			metadataMatches := map[string]int{}
			markerQueries := 0

			for evt, err := range workloadclient.RangeAllAuditEventsByType(
				ctx,
				admin,
				start,
				end,
				events.DatabaseSessionStartEvent,
				events.DatabaseSessionQueryEvent,
				events.DatabaseSessionEndEvent,
			) {
				if err != nil {
					return false, trace.Wrap(err, "reading events")
				}

				getter, ok := evt.(events.SessionMetadataGetter)
				if !ok || getter.GetSessionID() != discoveredSID {
					continue
				}

				eventmap[evt.GetType()]++
				switch e := evt.(type) {
				case *apievents.DatabaseSessionStart:
					if databaseMetadataMatches(e.DatabaseMetadata, route) {
						metadataMatches[evt.GetType()]++
					}
				case *apievents.DatabaseSessionQuery:
					if databaseMetadataMatches(e.DatabaseMetadata, route) {
						metadataMatches[evt.GetType()]++
					}
					if databaseQueryContainsMarker(e, marker) {
						markerQueries++
					}
				case *apievents.DatabaseSessionEnd:
					if databaseMetadataMatches(e.DatabaseMetadata, route) {
						metadataMatches[evt.GetType()]++
					}
				}
			}

			addDetail("session_id", discoveredSID)
			addDetail("eventcounts", eventmap)
			addDetail("event_metadata_matches", metadataMatches)
			addDetail("marker_queries", markerQueries)

			return eventmap[events.DatabaseSessionStartEvent] == 1 &&
				eventmap[events.DatabaseSessionQueryEvent] >= 1 &&
				eventmap[events.DatabaseSessionEndEvent] == 1 &&
				metadataMatches[events.DatabaseSessionStartEvent] == 1 &&
				metadataMatches[events.DatabaseSessionQueryEvent] >= 1 &&
				metadataMatches[events.DatabaseSessionEndEvent] == 1 &&
				markerQueries == 1, nil
		},
	}))
}

func databaseQueryContainsMarker(e *apievents.DatabaseSessionQuery, marker string) bool {
	if strings.Contains(e.DatabaseQuery, marker) {
		return true
	}
	for _, parameter := range e.DatabaseQueryParameters {
		if strings.Contains(parameter, marker) {
			return true
		}
	}
	return false
}

func databaseMetadataMatches(metadata apievents.DatabaseMetadata, route proto.RouteToDatabase) bool {
	return metadata.DatabaseService == route.GetServiceName() &&
		metadata.DatabaseProtocol == route.GetProtocol() &&
		metadata.DatabaseName == route.GetDatabase() &&
		metadata.DatabaseUser == route.GetUsername()
}
