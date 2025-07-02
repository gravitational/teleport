/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// newBotInstance creates (but does not insert) a bot instance that is ready for
// insertion into the backend. If any modifier functions are provided, they will
// be executed on the instance before it is returned.
func newBotInstance(botName string, fns ...func(*machineidv1.BotInstance)) *machineidv1.BotInstance {
	id := uuid.New()

	bi := &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    botName,
			InstanceId: id.String(),
		},
		Status: &machineidv1.BotInstanceStatus{},
	}

	for _, fn := range fns {
		fn(bi)
	}

	return bi
}

// withBotInstanceInvalidMetadata modifies a BotInstance such that it should
// raise an error during an insert attempt.
func withBotInstanceInvalidMetadata() func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		bi.Metadata = &headerv1.Metadata{
			Name: "invalid",
		}
	}
}

// withBotInstanceExpiry sets the .Metadata.Expires field of a bot instance to
// the given timestamp.
func withBotInstanceExpiry(expiry time.Time) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if bi.Metadata == nil {
			bi.Metadata = &headerv1.Metadata{}
		}

		bi.Metadata.Expires = timestamppb.New(expiry)
	}
}

// withBotInstanceId sets the .Spec.InstanceId field of a bot instance to
// the given value.
func withBotInstanceId(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if bi.Spec == nil {
			bi.Spec = &machineidv1.BotInstanceSpec{}
		}

		bi.Spec.InstanceId = value
	}
}

// withBotInstanceHeartbeatJoinMethod sets the .Status.InitialHeartbeat.JoinMethod
// field of a bot instance to the given value.
func withBotInstanceHeartbeatJoinMethod(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if bi.Status == nil {
			bi.Status = &machineidv1.BotInstanceStatus{}
		}

		if bi.Status.InitialHeartbeat == nil {
			bi.Status.InitialHeartbeat = &machineidv1.BotInstanceStatusHeartbeat{}
		}

		bi.Status.InitialHeartbeat.JoinMethod = value
	}
}

// withBotInstanceHeartbeatVersion sets the .Status.InitialHeartbeat.Version
// field of a bot instance to the given value.
func withBotInstanceHeartbeatVersion(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if bi.Status == nil {
			bi.Status = &machineidv1.BotInstanceStatus{}
		}

		if bi.Status.InitialHeartbeat == nil {
			bi.Status.InitialHeartbeat = &machineidv1.BotInstanceStatusHeartbeat{}
		}

		bi.Status.InitialHeartbeat.Version = value
	}
}

// withBotInstanceHeartbeatHostname sets the .Status.InitialHeartbeat.Hostname
// field of a bot instance to the given value.
func withBotInstanceHeartbeatHostname(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if bi.Status == nil {
			bi.Status = &machineidv1.BotInstanceStatus{}
		}

		if bi.Status.InitialHeartbeat == nil {
			bi.Status.InitialHeartbeat = &machineidv1.BotInstanceStatusHeartbeat{}
		}

		bi.Status.InitialHeartbeat.Hostname = value
	}
}

// createInstances creates new bot instances for the named bot with random UUIDs
func createInstances(t *testing.T, ctx context.Context, service *BotInstanceService, botName string, count int) map[string]struct{} {
	t.Helper()

	ids := map[string]struct{}{}

	for range count {
		bi := newBotInstance(botName)
		_, err := service.CreateBotInstance(ctx, bi)
		require.NoError(t, err)

		ids[bi.Spec.InstanceId] = struct{}{}
	}

	return ids
}

// listInstances fetches all instances from the BotInstanceService matching the botName filter
func listInstances(t *testing.T, ctx context.Context, service *BotInstanceService, botName string, searchTerm string) []*machineidv1.BotInstance {
	t.Helper()

	var resources []*machineidv1.BotInstance
	var bis []*machineidv1.BotInstance
	var nextKey string
	var err error

	for {
		bis, nextKey, err = service.ListBotInstances(ctx, botName, 0, nextKey, searchTerm)
		require.NoError(t, err)

		resources = append(resources, bis...)

		if nextKey == "" {
			break
		}
	}

	return resources
}

// TestBotInstanceCreateMetadata ensures bot instance metadata is constructed
// correctly when a new bot instance is inserted into the backend.
func TestBotInstanceCreateMetadata(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	tests := []struct {
		name        string
		instance    *machineidv1.BotInstance
		assertError require.ErrorAssertionFunc
		assertValue require.ValueAssertionFunc
	}{
		{
			name:        "non-nil metadata",
			instance:    newBotInstance("foo", withBotInstanceInvalidMetadata()),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				// .Metadata.Name should be overwritten with the correct value
				require.Equal(t, bi.Spec.InstanceId, bi.Metadata.Name)
				require.Nil(t, bi.Metadata.Expires)
			},
		},
		{
			name:        "valid without expiry",
			instance:    newBotInstance("foo"),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				require.Equal(t, bi.Spec.InstanceId, bi.Metadata.Name)
				require.Nil(t, bi.Metadata.Expires)
			},
		},
		{
			name:        "valid with expiry",
			instance:    newBotInstance("foo", withBotInstanceExpiry(clock.Now().Add(time.Hour))),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				require.Equal(t, bi.Spec.InstanceId, bi.Metadata.Name)
				require.Equal(t, clock.Now().Add(time.Hour).UTC(), bi.Metadata.Expires.AsTime())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			mem, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)

			service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
			require.NoError(t, err)

			value, err := service.CreateBotInstance(ctx, tc.instance)
			tc.assertError(t, err)
			tc.assertValue(t, value)
		})
	}
}

// TestBotInstanceInvalidGetters ensures proper behavior for an invalid
// GetBotInstance call.
func TestBotInstanceInvalidGetters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	_, err = service.CreateBotInstance(ctx, newBotInstance("example"))
	require.NoError(t, err)

	_, err = service.GetBotInstance(ctx, "example", "invalid")
	require.True(t, trace.IsNotFound(err))
}

// TestBotInstanceCRUD tests backend CRUD functionality for the bot instance
// service.
func TestBotInstanceCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	bi := newBotInstance("example")
	patched, err := service.CreateBotInstance(ctx, bi)
	require.NoError(t, err)

	// metadata should be generated from the bot spec
	require.Equal(t, bi.Spec.InstanceId, patched.Metadata.Name)

	// we should be able to retrieve a matching instance
	bi2, err := service.GetBotInstance(ctx, bi.Spec.BotName, bi.Spec.InstanceId)
	require.NoError(t, err)
	require.EqualExportedValues(t, patched, bi2)
	require.Equal(t, bi.Metadata.Name, bi2.Metadata.Name)

	resources := listInstances(t, ctx, service, "example", "")

	require.Len(t, resources, 1, "must list only 1 bot instance")
	require.EqualExportedValues(t, patched, resources[0])

	// append a heartbeat to a stored instance
	heartbeat := &machineidv1.BotInstanceStatusHeartbeat{
		Hostname: "foo",
	}

	patched, err = service.PatchBotInstance(ctx, bi.Spec.BotName, bi.Spec.InstanceId, func(bi *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
		bi.Status.LatestHeartbeats = append([]*machineidv1.BotInstanceStatusHeartbeat{heartbeat}, bi.Status.LatestHeartbeats...)
		return bi, nil
	})
	require.NoError(t, err)

	require.Len(t, patched.Status.LatestHeartbeats, 1)
	require.EqualExportedValues(t, heartbeat, patched.Status.LatestHeartbeats[0])

	// delete the stored instance
	require.NoError(t, service.DeleteBotInstance(ctx, bi.Spec.BotName, bi.Spec.InstanceId))

	// subsequent delete attempts should fail
	require.Error(t, service.DeleteBotInstance(ctx, bi.Spec.BotName, bi.Spec.InstanceId))
}

// TestBotInstanceList verifies list and filtering by bot functionality for bot
// instances.
func TestBotInstanceList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	aIds := createInstances(t, ctx, service, "a", 3)
	bIds := createInstances(t, ctx, service, "b", 4)

	// listing "a" should only return known "a" instances
	aInstances := listInstances(t, ctx, service, "a", "")
	require.Len(t, aInstances, 3)
	for _, ins := range aInstances {
		require.Contains(t, aIds, ins.Spec.InstanceId)
	}

	// listing "b" should only return known "b" instances
	bInstances := listInstances(t, ctx, service, "b", "")
	require.Len(t, bInstances, 4)
	for _, ins := range bInstances {
		require.Contains(t, bIds, ins.Spec.InstanceId)
	}

	allIds := map[string]struct{}{}
	for i := range aIds {
		allIds[i] = struct{}{}
	}
	for i := range bIds {
		allIds[i] = struct{}{}
	}

	// Listing an empty bot name ("") should return all instances.
	allInstances := listInstances(t, ctx, service, "", "")
	require.Len(t, allInstances, 7)
	for _, ins := range allInstances {
		require.Contains(t, allIds, ins.Spec.InstanceId)
	}
}

// TestBotInstanceListWithSearchFilter verifies list and filtering wit39db3c10-870c-4544-aeec-9fc2e961eca3h search
// term functionality for bot instances.
func TestBotInstanceListWithSearchFilter(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	tcs := []struct {
		name       string
		searchTerm string
		instance   *machineidv1.BotInstance
	}{
		{
			name:       "match on bot name",
			searchTerm: "nick",
			instance:   newBotInstance("this-is-nicks-test-bot"),
		},
		{
			name:       "match on instance id",
			searchTerm: "cb2c352",
			instance:   newBotInstance("test-bot", withBotInstanceId("cb2c3523-01f6-4258-966b-ace9f38f9862")),
		},
		{
			name:       "match on join method",
			searchTerm: "uber",
			instance:   newBotInstance("test-bot", withBotInstanceHeartbeatJoinMethod("kubernetes")),
		},
		{
			name:       "match on version",
			searchTerm: "1.0.0",
			instance:   newBotInstance("test-bot", withBotInstanceHeartbeatVersion("1.0.0-dev-a2g3hd")),
		},
		{
			name:       "match on version (with v)",
			searchTerm: "v1.0.0",
			instance:   newBotInstance("test-bot", withBotInstanceHeartbeatVersion("1.0.0-dev-a2g3hd")),
		},
		{
			name:       "match on hostname",
			searchTerm: "tel-123",
			instance:   newBotInstance("test-bot", withBotInstanceHeartbeatHostname("svr-eu-tel-123-a")),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			mem, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)

			service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
			require.NoError(t, err)

			_, err = service.CreateBotInstance(ctx, tc.instance)
			require.NoError(t, err)
			_, err = service.CreateBotInstance(ctx, newBotInstance("bot-not-matched"))
			require.NoError(t, err)

			instances := listInstances(t, ctx, service, "", tc.searchTerm)

			require.Len(t, instances, 1)
			require.Equal(t, tc.instance.Spec.InstanceId, instances[0].Spec.InstanceId)
		})
	}
}
