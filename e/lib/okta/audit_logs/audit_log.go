package auditlogs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type cursor struct {
	lastEventID   string    // document ID of the last event
	lastEventTime time.Time // time of the last event
	after         string
}

func (a *Service) pollAuditLogs(ctx context.Context, startTime time.Time, cursor cursor) ([]*accessgraphv1alpha.OktaEventV1, cursor, error) {
	const (
		order                   = "ASCENDING"
		limit                   = 1000
		maxReturningElems       = 5000
		maxMessageSizeSoftLimit = 3 * 1024 * 1024 // 3MB
	)
	queryParams := query.NewQueryParams(
		query.WithSince(startTime.Format(time.RFC3339)),
		query.WithUntil(a.clock.Now().UTC().Format(time.RFC3339)),
		query.WithSortOrder(order),
		query.WithLimit(limit),
		query.WithAfter(cursor.after),
	)

	var (
		events []*accessgraphv1alpha.OktaEventV1
		size   int
	)
	evts, rsp, err := a.client.ListLogEvents(ctx, queryParams)
	var oktaErr *okta.Error
	for {
		switch {
		case errors.As(err, &oktaErr) && oktaErr.ErrorCode == oktaapi.OktaErrCodeRateLimitException:
			return events, cursor, nil
		case err != nil:
			return nil, cursor, trace.Wrap(err)
		}

		if cursor.lastEventID != "" {
			idx := 0
			for i, entry := range evts {
				if entry.Uuid == cursor.lastEventID {
					idx = i + 1
					break
				}
			}
			evts = evts[idx:]
			if len(evts) == 0 && rsp.HasNextPage() {
				cursor.after = extractAfterFromNext(rsp.NextPage)
				cursor.lastEventID = ""
				rsp, err = rsp.Next(ctx, &evts)
				continue
			}
		}

		if len(evts) == 0 {
			slog.DebugContext(ctx, "No audit log entries found")
			break
		}

		for _, log := range evts {
			if log.Published != nil && log.Published.Before(cursor.lastEventTime) {
				continue
			}
			evt, err := toProtobuf(log, a.orgURL)
			if err != nil {
				return nil, cursor, trace.Wrap(err)
			}
			events = append(events, evt)
			size += proto.Size(evt)
		}

		if evts[len(evts)-1].Published != nil {
			cursor.lastEventTime = *evts[len(evts)-1].Published
		}

		if !rsp.HasNextPage() {
			cursor.lastEventID = evts[len(evts)-1].Uuid
			break
		}

		cursor.after = extractAfterFromNext(rsp.NextPage)
		cursor.lastEventID = ""

		if size >= maxMessageSizeSoftLimit || len(events) >= maxReturningElems {
			break
		}
		rsp, err = rsp.Next(ctx, &evts)
	}
	return events, cursor, nil
}

func extractAfterFromNext(next string) string {
	const afterPrefix = "after"
	u, err := url.Parse(next)
	if err != nil {
		return ""
	}
	q := u.Query()
	return q.Get(afterPrefix)
}

func toProtobuf(log *okta.LogEvent, orgHref string) (*accessgraphv1alpha.OktaEventV1, error) {
	// Create root level event properties
	location := &accessgraphv1alpha.OktaLocationV1{}
	if log.Client != nil {
		location.SetIp(log.Client.IpAddress)
	}
	identity := &accessgraphv1alpha.OktaIdentityV1{}
	if log.Actor != nil {
		identity.SetId(log.Actor.AlternateId)
		identity.SetName(log.Actor.DisplayName)
		identity.SetKind(log.Actor.Type)
	}
	if log.Transaction != nil && log.Transaction.Detail != nil {
		detail := log.Transaction.Detail
		// Check if the detail is a map
		if detailMap, ok := detail.(map[string]any); ok {
			if value, ok := detailMap["requestApiTokenId"]; ok {
				tokenId, ok := value.(string)
				if ok {
					identity.SetToken(tokenId)
				}
			}
		}
	}
	eventType := log.EventType
	published := time.Now().UTC()
	if log.Published != nil {
		published = *log.Published
	}

	if log.Client != nil && log.Client.UserAgent != nil {
		identity.SetUserAgent(log.Client.UserAgent.RawUserAgent)
	}

	outcome := ""
	if log.Outcome != nil {
		outcome = log.Outcome.Result
	}

	eventData, err := structToPbStruct(log)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var targets []*accessgraphv1alpha.OktaTargetV1
	for _, oktaTarget := range log.Target {
		targets = append(targets, accessgraphv1alpha.OktaTargetV1_builder{
			Kind: oktaTarget.Type,
			Name: oktaTarget.DisplayName,
			Id:   oktaTarget.AlternateId,
		}.Build())
	}

	event := accessgraphv1alpha.OktaEventV1_builder{
		Origin:    orgHref,
		Identity:  identity,
		Location:  location,
		EventType: eventType,
		Result:    outcome,
		Targets:   targets,
		Published: timestamppb.New(published),
		EventData: eventData,
	}.Build()

	return event, nil
}

func structToPbStruct(data any) (*structpb.Struct, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pb := &structpb.Struct{}
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, pb)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb, nil
}
