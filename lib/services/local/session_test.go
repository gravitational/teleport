// Copyright 2022 Gravitational, Inc
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

package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDeleteUserAppSessions(t *testing.T) {
	identity, clock := newIdentityService(t)
	users := []string{"alice", "bob"}
	ctx := context.Background()

	// Create app sessions for different users.
	for _, user := range users {
		session, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
			User:    user,
			Expires: clock.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		err = identity.UpsertAppSession(ctx, session)
		require.NoError(t, err)
	}

	// Ensure the number of app sessions is correct.
	sessions, err := identity.GetAppSessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	// Delete sessions of the first user.
	err = identity.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: users[0]})
	require.NoError(t, err)

	sessions, err = identity.GetAppSessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, users[1], sessions[0].GetUser())

	// Delete sessions of the second user.
	err = identity.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: users[1]})
	require.NoError(t, err)

	sessions, err = identity.GetAppSessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 0)
}
