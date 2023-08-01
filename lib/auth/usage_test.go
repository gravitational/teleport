package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type mockAuditLogSessionStreamer struct {
	events.DiscardAuditLog
	t      *testing.T
	events []apievents.AuditEvent
}

func newMockAuditLogSessionStreamer(t *testing.T, events []apievents.AuditEvent) *mockAuditLogSessionStreamer {
	return &mockAuditLogSessionStreamer{
		t:      t,
		events: events,
	}
}

// SearchEvents implements events.AuditLogSessionStreamer
func (m *mockAuditLogSessionStreamer) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	require.Equal(m.t, []string{events.AccessRequestCreateEvent, events.AccessRequestReviewEvent}, req.EventTypes)
	var results []apievents.AuditEvent
	for _, ev := range m.events {
		if !req.From.IsZero() && ev.GetTime().Before(req.From) {
			continue
		}
		if !req.To.IsZero() && ev.GetTime().After(req.To) {
			continue
		}
		results = append(results, ev)
	}
	return results, "", nil
}

func TestAccessRequestLimit(t *testing.T) {
	const monthlyLimit = 3

	makeEvent := func(eventType string, id string, timestamp time.Time) apievents.AuditEvent {
		return &apievents.AccessRequestCreate{
			Metadata: apievents.Metadata{
				Type: eventType,
				Time: timestamp,
			},
			RequestID: id,
		}
	}

	features := modules.GetModules().Features()
	features.IsUsageBasedBilling = true
	features.AccessRequests.MonthlyRequestLimit = monthlyLimit
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: features,
	})

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// Set up RBAC
	access, err := types.NewRole("access", types.RoleSpecV6{})
	require.NoError(t, err)
	p.a.CreateRole(ctx, access)
	require.NoError(t, err)
	requestor, err := types.NewRole("requestor", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)
	p.a.CreateRole(ctx, requestor)
	require.NoError(t, err)
	alice, err := types.NewUser("alice")
	alice.SetRoles([]string{"requestor"})
	require.NoError(t, err)
	err = p.a.CreateUser(ctx, alice)
	require.NoError(t, err)

	// Mock audit log
	// Create a clock in the middle of the month for easy manipulation
	clock := clockwork.NewFakeClockAt(
		time.Date(2023, 07, 15, 1, 2, 3, 0, time.UTC))
	p.a.SetClock(clock)

	july := clock.Now()
	august := clock.Now().AddDate(0, 1, 0)
	mockEvents := []apievents.AuditEvent{
		// 3 "used" (created + reviewed) requests in July:
		// can not create any more
		makeEvent(events.AccessRequestCreateEvent, "aaa", july.AddDate(0, 0, -6)),
		makeEvent(events.AccessRequestReviewEvent, "aaa", july.AddDate(0, 0, -5)),
		makeEvent(events.AccessRequestCreateEvent, "bbb", july.AddDate(0, 0, -4)),
		makeEvent(events.AccessRequestReviewEvent, "bbb", july.AddDate(0, 0, -3)),
		makeEvent(events.AccessRequestCreateEvent, "ccc", july.AddDate(0, 0, -2)),
		makeEvent(events.AccessRequestReviewEvent, "ccc", july.AddDate(0, 0, -1)),

		// 3 access requests created in August,
		// only 2 of them reviewed: can create one more
		makeEvent(events.AccessRequestCreateEvent, "ddd", august.AddDate(0, 0, -5)),
		makeEvent(events.AccessRequestReviewEvent, "ddd", august.AddDate(0, 0, -4)),
		makeEvent(events.AccessRequestCreateEvent, "eee", august.AddDate(0, 0, -3)),
		makeEvent(events.AccessRequestCreateEvent, "fff", august.AddDate(0, 0, -2)),
		makeEvent(events.AccessRequestReviewEvent, "fff", august.AddDate(0, 0, -1)),
	}

	al := newMockAuditLogSessionStreamer(t, mockEvents)
	p.a.SetAuditLog(al)

	// Check July
	usage, err := p.a.GetResourceUsage(ctx, &proto.GetResourceUsageRequest{})
	require.NoError(t, err)
	require.Equal(t, int32(monthlyLimit), usage.GetAccessRequestsMonthly().Limit)
	require.Equal(t, int32(3), usage.GetAccessRequestsMonthly().Used)

	req, err := types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	err = p.a.CreateAccessRequest(ctx, req, tlsca.Identity{})
	require.NoError(t, err)

	// Check August
	clock.Advance(31 * 24 * time.Hour)
	usage, err = p.a.GetResourceUsage(ctx, &proto.GetResourceUsageRequest{})
	require.NoError(t, err)
	require.Equal(t, int32(monthlyLimit), usage.GetAccessRequestsMonthly().Limit)
	require.Equal(t, int32(2), usage.GetAccessRequestsMonthly().Used)

	req, err = types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	err = p.a.CreateAccessRequest(ctx, req, tlsca.Identity{})
	require.NoError(t, err)
}
