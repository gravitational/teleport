// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestUserProvisionerTeardownCanceledContext checks that ending a session by
// canceling its context still disables or deletes the auto-provisioned user.
// The connection monitor cancels that context when it closes the connection.
func TestUserProvisionerTeardownCanceledContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		mode           types.CreateDatabaseUserMode
		cancel         bool
		wantDeactivate int
		wantDelete     int
	}{
		{
			name:           "keep after the monitor closes the connection",
			mode:           types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			cancel:         true,
			wantDeactivate: 1,
		},
		{
			name:       "drop after the monitor closes the connection",
			mode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			cancel:     true,
			wantDelete: 1,
		},
		{
			name:           "keep on a normal disconnect",
			mode:           types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			wantDeactivate: 1,
		},
		{
			name:       "drop on a normal disconnect",
			mode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			wantDelete: 1,
		},
		{
			name:   "provisioning off ignores a canceled context",
			mode:   types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
			cancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sem := &recordingSemaphores{}
			users := &recordingAutoUsers{}
			provisioner := &UserProvisioner{
				AuthClient: sem,
				Backend:    users,
				Log:        logtest.NewLogger(),
				Clock:      clockwork.NewRealClock(),
			}

			ctx := context.Background()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := provisioner.Teardown(ctx, testAutoUserSession(t, tt.mode))
			require.NoError(t, err)
			require.Equal(t, tt.wantDeactivate, users.deactivateCalls)
			require.Equal(t, tt.wantDelete, users.deleteCalls)
			if tt.wantDeactivate+tt.wantDelete == 0 {
				require.Zero(t, sem.acquires)
				return
			}
			require.Equal(t, 1, sem.acquires)
			require.Equal(t, 1, sem.cancels)
			require.NoError(t, sem.acquireCtxErr)
			require.NoError(t, sem.cancelCtxErr)
			require.NoError(t, users.cleanupCtxErr)
		})
	}
}

func testAutoUserSession(t *testing.T, mode types.CreateDatabaseUserMode) *Session {
	t.Helper()
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	return &Session{
		Database:           database,
		DatabaseUser:       "alice",
		AutoCreateUserMode: mode,
	}
}

// recordingSemaphores rejects calls whose context is already canceled, which
// is what the auth gRPC client does.
type recordingSemaphores struct {
	types.Semaphores

	mu            sync.Mutex
	acquires      int
	cancels       int
	acquireCtxErr error
	cancelCtxErr  error
}

func (s *recordingSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acquires++
	s.acquireCtxErr = ctx.Err()
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.SemaphoreLease{
		SemaphoreKind: params.SemaphoreKind,
		SemaphoreName: params.SemaphoreName,
		LeaseID:       "lease",
		Expires:       params.Expires,
	}, nil
}

func (s *recordingSemaphores) CancelSemaphoreLease(ctx context.Context, _ types.SemaphoreLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancels++
	s.cancelCtxErr = ctx.Err()
	return trace.Wrap(ctx.Err())
}

// recordingAutoUsers records whether cleanup ran with a live context. Database
// drivers fail a disable or drop when the context is already canceled.
type recordingAutoUsers struct {
	mu              sync.Mutex
	deactivateCalls int
	deleteCalls     int
	cleanupCtxErr   error
}

func (u *recordingAutoUsers) ActivateUser(context.Context, *Session) error {
	return nil
}

func (u *recordingAutoUsers) DeactivateUser(ctx context.Context, _ *Session) error {
	return u.record(ctx, true)
}

func (u *recordingAutoUsers) DeleteUser(ctx context.Context, _ *Session) error {
	return u.record(ctx, false)
}

func (u *recordingAutoUsers) record(ctx context.Context, deactivate bool) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.cleanupCtxErr = ctx.Err()
	if deactivate {
		u.deactivateCalls++
	} else {
		u.deleteCalls++
	}
	return trace.Wrap(ctx.Err())
}
