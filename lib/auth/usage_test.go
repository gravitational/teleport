// Copyright 2023 Gravitational, Inc
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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	eventstest "github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

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
		// 3 created requests in July: can not create any more
		makeEvent(events.AccessRequestCreateEvent, "aaa", july.AddDate(0, 0, -3)),
		makeEvent(events.AccessRequestCreateEvent, "bbb", july.AddDate(0, 0, -2)),
		makeEvent(events.AccessRequestCreateEvent, "ccc", july.AddDate(0, 0, -1)),

		// 2 access requests created in August: can create one more
		makeEvent(events.AccessRequestCreateEvent, "ddd", august.AddDate(0, 0, -2)),
		makeEvent(events.AccessRequestCreateEvent, "eee", august.AddDate(0, 0, -1)),
	}

	al := eventstest.NewMockAuditLogSessionStreamer(mockEvents, func(req events.SearchEventsRequest) error {
		if !slices.Equal([]string{events.AccessRequestCreateEvent}, req.EventTypes) {
			return trace.BadParameter("expected AccessRequestCreateEvent only, got %v", req.EventTypes)
		}
		return nil
	})
	p.a.SetAuditLog(al)

	// Check July
	req, err := types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	_, err = p.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.Error(t, err, "expected access request creation to fail due to the monthly limit")

	// Check August
	clock.Advance(31 * 24 * time.Hour)
	req, err = types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	_, err = p.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.NoError(t, err)
}
