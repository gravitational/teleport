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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/russellhaering/gosaml2/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

func newBotInstance(botName string, fns ...func(*machineidv1.BotInstance)) *machineidv1.BotInstance {
	id := uuid.NewV4()

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

func withBotInstanceTTL(d time.Duration) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		bi.Spec.Ttl = durationpb.New(d)
	}
}

func withBotInstanceInvalidMetadata() func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		bi.Metadata = &headerv1.Metadata{
			Name: "invalid",
		}
	}
}

func TestBotInstanceCreateMetadata(t *testing.T) {
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
			assertError: require.Error,
			assertValue: require.Nil,
		},
		{
			name:        "valid without ttl",
			instance:    newBotInstance("foo"),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i interface{}, _ ...interface{}) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				require.Equal(t, bi.Spec.InstanceId, bi.Metadata.Name)
				require.Nil(t, bi.Metadata.Expires)
			},
		},
		{
			name:        "valid with ttl",
			instance:    newBotInstance("foo", withBotInstanceTTL(time.Hour)),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i interface{}, _ ...interface{}) {
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

func TestBotInstanceInvalidGetters(t *testing.T) {
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

func TestBotInstanceCRUD(t *testing.T) {
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

	// fetch all stored instances
	var resources []*machineidv1.BotInstance
	var bis []*machineidv1.BotInstance
	var nextKey string

	for {
		bis, nextKey, err = service.ListBotInstances(ctx, "example", 0, nextKey)
		require.NoError(t, err)

		resources = append(resources, bis...)

		if nextKey == "" {
			break
		}
	}

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
