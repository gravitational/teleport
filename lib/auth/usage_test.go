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

package auth

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	eventstest "github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAccessRequestLimit(t *testing.T) {
	username := "alice"
	rolename := "access"
	ctx := context.Background()

	s := setUpAccessRequestLimitForJulyAndAugust(t, username, rolename)

	// Check July
	req, err := types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.Error(t, err, "expected access request creation to fail due to the monthly limit")

	// Check August
	s.clock.Advance(31 * 24 * time.Hour)
	req, err = types.NewAccessRequest(uuid.New().String(), "alice", "access")
	require.NoError(t, err)
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.NoError(t, err)
}

func TestAccessRequest_WithAndWithoutLimit(t *testing.T) {
	username := "alice"
	rolename := "access"
	ctx := context.Background()

	s := setUpAccessRequestLimitForJulyAndAugust(t, username, rolename)

	// Check July
	req, err := types.NewAccessRequest(uuid.New().String(), username, rolename)
	require.NoError(t, err)
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.Error(t, err, "expected access request creation to fail due to the monthly limit")

	// Lift limit with IGS, expect no limit error.
	s.features.IdentityGovernanceSecurity = true
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: s.features,
	})
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.NoError(t, err)

	// Put back limit, expect limit error.
	s.features.IdentityGovernanceSecurity = false
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: s.features,
	})
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.Error(t, err, "expected access request creation to fail due to the monthly limit")

	// Lift limit with legacy non-usage based, expect no limit error.
	s.features.IsUsageBasedBilling = false
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: s.features,
	})
	_, err = s.testpack.a.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
	require.NoError(t, err)
}

type setupAccessRequestLimist struct {
	monthlyLimit int
	testpack     testPack
	clock        clockwork.FakeClock
	features     modules.Features
}

func setUpAccessRequestLimitForJulyAndAugust(t *testing.T, username string, rolename string) setupAccessRequestLimist {
	monthlyLimit := 3

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
	access, err := types.NewRole(rolename, types.RoleSpecV6{})
	require.NoError(t, err)
	p.a.CreateRole(ctx, access)
	require.NoError(t, err)
	requestor, err := types.NewRole("requestor", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{rolename},
			},
		},
	})
	require.NoError(t, err)
	p.a.CreateRole(ctx, requestor)
	require.NoError(t, err)

	alice, err := types.NewUser(username)
	alice.SetRoles([]string{"requestor"})
	require.NoError(t, err)
	_, err = p.a.CreateUser(ctx, alice)
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

	return setupAccessRequestLimist{
		testpack:     p,
		monthlyLimit: monthlyLimit,
		features:     features,
		clock:        clock,
	}
}
