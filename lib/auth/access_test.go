/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func TestUpsertDeleteRoleEventsEmitted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// test create new role
	role, err := types.NewRoleV3("test-role", types.RoleSpecV5{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	// Creating a role should emit a RoleCreatedEvent.
	err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)
	require.Equal(t, p.mockEmitter.LastEvent().GetType(), events.RoleCreatedEvent)
	require.Equal(t, p.mockEmitter.LastEvent().(*apievents.RoleCreate).Name, role.GetName())
	p.mockEmitter.Reset()

	// Updating a role should emit a RoleCreatedEvent.
	err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)
	require.Equal(t, p.mockEmitter.LastEvent().GetType(), events.RoleCreatedEvent)
	require.Equal(t, p.mockEmitter.LastEvent().(*apievents.RoleCreate).Name, role.GetName())
	p.mockEmitter.Reset()

	// Deleting a role should emit a RoleDeletedEvent.
	err = p.a.DeleteRole(ctx, role.GetName())
	require.NoError(t, err)
	require.Equal(t, p.mockEmitter.LastEvent().GetType(), events.RoleDeletedEvent)
	require.Equal(t, p.mockEmitter.LastEvent().(*apievents.RoleDelete).Name, role.GetName())
	p.mockEmitter.Reset()

	// When deleting a nonexistent role, no event should be emitted.
	err = p.a.DeleteRole(ctx, role.GetName())
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, p.mockEmitter.LastEvent())
}

func TestRoleConstraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	testRoles := []struct {
		Name  string
		Rules []types.Rule
	}{
		{
			Name: "test-role-1",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindRole},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
		{
			Name: "test-role-2",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindRole},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
		{
			Name: "test-role-3",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
	}

	// create and insert roles
	for _, r := range testRoles {
		role, err := types.NewRoleV3(r.Name, types.RoleSpecV5{
			Options: types.RoleOptions{},
			Allow: types.RoleConditions{
				Rules: r.Rules,
			},
		})
		require.NoError(t, err)

		err = p.a.UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	// remove upsert rules from test-role-2.
	// the operation will succeed because test-role-1 still has rules
	// to upsert roles.
	role, err := types.NewRoleV3(testRoles[1].Name, types.RoleSpecV5{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	// remove upsert rules from test-role-1.
	// the operation will fail because test-role-1 is the only role left that has
	// rules to upsert roles.
	role, err = types.NewRoleV3(testRoles[0].Name, types.RoleSpecV5{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	err = p.a.UpsertRole(ctx, role)
	require.Error(t, err)

	// remove upsert rules from test-role-3.
	// this is a control operation that shows that other resource types
	// are not affected by the "role resource constraints".
	role, err = types.NewRoleV3(testRoles[2].Name, types.RoleSpecV5{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	// delete test-role-1.
	// the operation will fail because test-role-1 is the only role left that has
	// rules to upsert roles.
	err = p.a.DeleteRole(ctx, testRoles[0].Name)
	require.Error(t, err)

	// delete test-role-3.
	// this is a control operation that shows that other resource types
	// are not affected by the "role resource constraints".
	err = p.a.DeleteRole(ctx, testRoles[2].Name)
	require.NoError(t, err)
}

func TestUpsertDeleteLockEventsEmitted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{MFADevice: "mfa-device-id"},
	})
	require.NoError(t, err)

	// Creating a lock should emit a LockCreatedEvent.
	err = p.a.UpsertLock(ctx, lock)
	require.NoError(t, err)
	require.Equal(t, events.LockCreatedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Name)
	require.Equal(t, lock.Target(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Target)
	p.mockEmitter.Reset()

	// When a lock update results in an error, no event should be emitted.
	lock.SetTarget(types.LockTarget{})
	err = p.a.UpsertLock(ctx, lock)
	require.Error(t, err)
	require.Nil(t, p.mockEmitter.LastEvent())

	// Updating a lock should emit a LockCreatedEvent.
	lock.SetTarget(types.LockTarget{Role: "test-role"})
	err = p.a.UpsertLock(ctx, lock)
	require.NoError(t, err)
	require.Equal(t, events.LockCreatedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Name)
	require.Equal(t, lock.Target(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Target)
	p.mockEmitter.Reset()

	// Deleting a lock should emit a LockDeletedEvent.
	err = p.a.DeleteLock(ctx, lock.GetName())
	require.NoError(t, err)
	require.Equal(t, events.LockDeletedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockDelete).Name)
	p.mockEmitter.Reset()

	// When deleting a nonexistent lock, no event should be emitted.
	err = p.a.DeleteLock(ctx, lock.GetName())
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, p.mockEmitter.LastEvent())
}
