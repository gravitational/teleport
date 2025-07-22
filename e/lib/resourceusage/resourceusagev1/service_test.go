package resourceusagev1

import (
	context "context"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	eventstest "github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type fakeAuthorizer struct {
	authorize bool
}

// Authorize implements authz.Authorizer
func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if !a.authorize {
		return nil, trace.AccessDenied("not authorized")
	}

	user, err := types.NewUser("alice")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authz.Context{
		User:    user,
		Checker: fakeChecker{},
		Identity: &authz.LocalUser{
			Username: "alice",
			Identity: tlsca.Identity{
				Groups: []string{"dev"},
			},
		},
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
}

func (fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	return nil
}

func Test_GetUsage(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	t.Run("unauthorized", func(t *testing.T) {
		svc := &Service{
			authorizer: &fakeAuthorizer{
				authorize: false,
			},
			auditLog: nil, // not invoked in this case
			clock:    clock,
		}
		_, err := svc.GetUsage(ctx, &resourceusagepb.GetUsageRequest{})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "GetUsage returned err=%v (%T), want AccessDenied", err, err)
	})

	t.Run("not usage-based billing", func(t *testing.T) {
		svc := &Service{
			authorizer: &fakeAuthorizer{
				authorize: true,
			},
			auditLog: nil, // not invoked in this case
			clock:    clock,
		}

		got, err := svc.GetUsage(ctx, &resourceusagepb.GetUsageRequest{})
		require.NoError(t, err, "GetUsage")

		want := &resourceusagepb.GetUsageResponse{
			AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_UNLIMITED,
			AccessRequests:   &resourceusagepb.AccessRequestsUsage{},
			DevicesUsage:     &resourceusagepb.DevicesUsage{},
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetUsage mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("usage-based billing", func(t *testing.T) {
		makeEvent := func(eventType string, id string, timestamp time.Time) apievents.AuditEvent {
			return &apievents.AccessRequestCreate{
				Metadata: apievents.Metadata{
					Type: eventType,
					Time: timestamp,
				},
				RequestID: id,
			}
		}

		// Mock audit log
		clock := clockwork.NewFakeClockAt(time.Date(2023, 07, 15, 1, 2, 3, 0, time.UTC))
		now := clock.Now()
		mockEvents := []apievents.AuditEvent{
			makeEvent(events.AccessRequestCreateEvent, "aaa", now.AddDate(0, 0, -4)),
			makeEvent(events.AccessRequestCreateEvent, "bbb", now.AddDate(0, 0, -3)),
			makeEvent(events.AccessRequestCreateEvent, "ccc", now.AddDate(0, 0, -2)),
			makeEvent(events.AccessRequestCreateEvent, "ddd", now.AddDate(0, 0, -1)),
		}

		al := eventstest.NewMockAuditLogSessionStreamer(mockEvents, func(req events.SearchEventsRequest) error {
			if !slices.Equal([]string{events.AccessRequestCreateEvent}, req.EventTypes) {
				return trace.BadParameter("expected AccessRequestCreateEvent only, got %v", req.EventTypes)
			}
			return nil
		})

		// Set features
		const monthlyLimit = 42
		features := modules.GetModules().Features()
		features.IsUsageBasedBilling = true
		features.Entitlements[entitlements.AccessRequests] = modules.EntitlementInfo{Limit: monthlyLimit, Enabled: true}
		modulestest.SetTestModules(t, modulestest.Modules{
			TestFeatures: features,
		})

		devicesUsage := &resourceusagepb.DevicesUsage{
			DevicesUsageLimit: 10,
			DevicesInUse:      5,
		}
		svc := &Service{
			authorizer: &fakeAuthorizer{
				authorize: true,
			},
			auditLog: al,
			clock:    clock,
			getDevicesUsageFunc: func(ctx context.Context, f *modules.Features) (*resourceusagepb.DevicesUsage, error) {
				return devicesUsage, nil
			},
		}

		got, err := svc.GetUsage(ctx, &resourceusagepb.GetUsageRequest{})
		require.NoError(t, err, "GetUsage")

		want := &resourceusagepb.GetUsageResponse{
			AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_USAGE_BASED,
			AccessRequests: &resourceusagepb.AccessRequestsUsage{
				MonthlyLimit: monthlyLimit,
				MonthlyUsed:  int32(len(mockEvents)),
			},
			DevicesUsage: devicesUsage,
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetUsage mismatch (-want +got)\n%s", diff)
		}
	})
}
