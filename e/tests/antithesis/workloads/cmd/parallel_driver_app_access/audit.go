package main

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
	"github.com/gravitational/teleport/lib/events"
)

func (p *TestCaseParams) assertExistingSessionAppAccessAuditEvents(ctx context.Context, sessionID string, start, end time.Time) error {
	admin, err := workloadclient.NewAPIClient(ctx, adminIdentity)
	if err != nil {
		return trace.Wrap(err, "creating api client")
	}
	defer admin.Close()

	details := p.Details(map[string]any{
		"session_id": sessionID,
		"from":       start,
		"to":         end,
	})

	// When using tbot application credentials, the session is baked into the
	// credential itself, so the only audit event we can expect here is app chunk.
	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Existing-session HTTP app access eventually emits at least one app.session.chunk audit event with matching app metadata",
		Timeout: auditEventEmitDeadline,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			for evt, err := range workloadclient.RangeAllAuditEventsByType(
				ctx,
				admin,
				start,
				end,
				events.AppSessionChunkEvent,
			) {
				if err != nil {
					return false, trace.Wrap(err, "reading events")
				}
				e, ok := evt.(*apievents.AppSessionChunk)
				if !ok || e.SessionMetadata.SessionID != sessionID {
					continue
				}

				addDetail("event_app_name", e.AppName)
				addDetail("event_app_public_addr", e.AppPublicAddr)
				addDetail("event_app_uri", e.AppURI)
				return e.AppName == p.App.Name &&
					e.AppPublicAddr == p.App.PublicAddr &&
					e.AppURI == p.App.URI, nil
			}
			return false, nil
		},
	})
}

func (p *TestCaseParams) assertMintedAppAccessAuditEvents(ctx context.Context, sessionID string, start, end time.Time) error {
	admin, err := workloadclient.NewAPIClient(ctx, adminIdentity)
	if err != nil {
		return trace.Wrap(err, "creating admin client")
	}
	defer admin.Close()

	details := p.Details(map[string]any{
		"session_id": sessionID,
		"from":       start,
		"to":         end,
	})

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "HTTP app access with minted credentials eventually emits exactly 1 app.session.start audit event with matching app metadata",
		Timeout: auditEventEmitDeadline,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			eventmap := map[string]int{}
			var gotAppName, gotPublicAddr, gotURI string
			for evt, err := range workloadclient.RangeAllAuditEventsByType(
				ctx,
				admin,
				start,
				end,
				events.AppSessionStartEvent,
			) {
				if err != nil {
					return false, trace.Wrap(err, "reading events")
				}
				e, ok := evt.(*apievents.AppSessionStart)
				if !ok || e.SessionMetadata.SessionID != sessionID {
					continue
				}

				eventmap[evt.GetType()]++
				gotAppName = e.AppName
				gotPublicAddr = e.AppPublicAddr
				gotURI = e.AppURI
			}

			addDetail("event_app_name", gotAppName)
			addDetail("event_app_public_addr", gotPublicAddr)
			addDetail("event_app_uri", gotURI)
			addDetail("eventcounts", eventmap)
			return eventmap[events.AppSessionStartEvent] == 1 &&
				appMetadataMatches(p.App, gotAppName, gotPublicAddr, gotURI), nil
		},
	})
}

func (p *TestCaseParams) assertTCPAppAccessAuditEvents(ctx context.Context, sessionID string, start, end time.Time) error {
	admin, err := workloadclient.NewAPIClient(ctx, adminIdentity)
	if err != nil {
		return trace.Wrap(err, "creating admin client")
	}
	defer admin.Close()

	details := p.Details(map[string]any{
		"session_id": sessionID,
		"from":       start,
		"to":         end,
	})

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "TCP app access with minted credentials eventually emits exactly 2 app.session.start audit events and exactly 1 app.session.end audit event with matching app metadata",
		Timeout: auditEventEmitDeadline,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			eventmap := map[string]int{}
			metadataMatches := map[string]int{}
			var gotAppName, gotPublicAddr, gotURI string
			for evt, err := range workloadclient.RangeAllAuditEventsByType(
				ctx,
				admin,
				start,
				end,
				events.AppSessionStartEvent,
				events.AppSessionEndEvent,
			) {
				if err != nil {
					return false, trace.Wrap(err, "reading events")
				}

				switch e := evt.(type) {
				case *apievents.AppSessionStart:
					if e.SessionMetadata.SessionID != sessionID {
						continue
					}
					eventmap[evt.GetType()]++
					gotAppName = e.AppName
					gotPublicAddr = e.AppPublicAddr
					gotURI = e.AppURI
					if appMetadataMatches(p.App, e.AppName, e.AppPublicAddr, e.AppURI) {
						metadataMatches[evt.GetType()]++
					}
				case *apievents.AppSessionEnd:
					if e.SessionMetadata.SessionID != sessionID {
						continue
					}
					eventmap[evt.GetType()]++
					gotAppName = e.AppName
					gotPublicAddr = e.AppPublicAddr
					gotURI = e.AppURI
					if appMetadataMatches(p.App, e.AppName, e.AppPublicAddr, e.AppURI) {
						metadataMatches[evt.GetType()]++
					}
				}
			}

			addDetail("event_app_name", gotAppName)
			addDetail("event_app_public_addr", gotPublicAddr)
			addDetail("event_app_uri", gotURI)
			addDetail("eventcounts", eventmap)
			addDetail("event_metadata_matches", metadataMatches)
			return eventmap[events.AppSessionStartEvent] == 2 &&
				eventmap[events.AppSessionEndEvent] == 1 &&
				metadataMatches[events.AppSessionStartEvent] == 2 &&
				metadataMatches[events.AppSessionEndEvent] == 1, nil
		},
	})
}

func appMetadataMatches(app AppTarget, name, publicAddr, uri string) bool {
	return name == app.Name &&
		publicAddr == app.PublicAddr &&
		uri == app.URI
}
