/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package slack

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

type mockReviewBot struct {
	mock.Mock
	ReviewBot
}

func (m *mockReviewBot) LookupEmailByUserID(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func newTestAuth(t *testing.T, m *modulestest.Modules) *auth.Server {
	t.Helper()

	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:     t.TempDir(),
			Clock:   clockwork.NewFakeClock(),
			Modules: m,
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	})
	require.NoError(t, err)

	authServer := server.Auth()
	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})
	return authServer
}

type user struct {
	slackUserId      string
	slackEmail       string
	teleportUsername string
	teleportTraits   map[string][]string
}

func createUserInSlackAndTeleport(t *testing.T, mockBot *mockReviewBot, authServer *auth.Server, u user) {
	t.Helper()

	// Link some emails to Slack users
	if u.slackEmail != "" {
		mockBot.On("LookupEmailByUserID", mock.Anything, u.slackUserId).Return(u.slackEmail, nil)
	} else {
		mockBot.On("LookupEmailByUserID", mock.Anything, u.slackUserId).Return("", nil)
	}

	// Create Teleport user with traits
	teleportUser, err := types.NewUser(u.teleportUsername)
	require.NoError(t, err)
	teleportUser.SetTraits(u.teleportTraits)

	_, err = authServer.CreateUser(t.Context(), teleportUser)
	require.NoError(t, err)
}

func TestResolveTeleportUser(t *testing.T) {
	authServer := newTestAuth(t, modulestest.OSSModules())

	aliceUser := user{
		slackUserId:      "U_alice",
		teleportUsername: "alice",
		teleportTraits:   map[string][]string{"slack_uid": {"U_alice"}},
	}
	bobUser := user{
		slackUserId:      "U_bob",
		slackEmail:       "bob@example.com",
		teleportUsername: "bob@example.com",
		teleportTraits:   nil,
	}
	carolUser := user{
		slackUserId:      "U_carol",
		teleportUsername: "carol",
		teleportTraits:   nil,
	}
	daveUser := user{
		slackUserId:      "U_dave",
		teleportUsername: "dave",
		teleportTraits:   map[string][]string{"test_trait_name": {"U_dave"}},
	}
	eveUser := user{
		slackUserId:      "U_eve",
		teleportUsername: "eve",
		teleportTraits:   map[string][]string{"slack_uid": {"U_eve"}},
	}
	eveDupeUser := user{
		slackUserId:      "U_eveDupe",
		teleportUsername: "eveDupe",
		teleportTraits:   map[string][]string{"slack_uid": {"U_eve"}},
	}

	mockBot := &mockReviewBot{}
	createUserInSlackAndTeleport(t, mockBot, authServer, aliceUser)
	createUserInSlackAndTeleport(t, mockBot, authServer, bobUser)
	createUserInSlackAndTeleport(t, mockBot, authServer, carolUser)
	createUserInSlackAndTeleport(t, mockBot, authServer, daveUser)
	createUserInSlackAndTeleport(t, mockBot, authServer, eveUser)
	createUserInSlackAndTeleport(t, mockBot, authServer, eveDupeUser)

	tests := []struct {
		name         string
		reviewConfig ReviewConfig
		slackUID     string
		want         string
		wantErr      bool
	}{
		{
			name: "trait match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: false,
			},
			slackUID: aliceUser.slackUserId,
			want:     aliceUser.teleportUsername,
		},
		{
			name: "trait match, allow email match - no linked email",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: aliceUser.slackUserId,
			want:     aliceUser.teleportUsername,
		},
		{
			name: "no trait match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: false,
			},
			slackUID: bobUser.slackUserId,
			wantErr:  true,
		},
		{
			name: "no trait match, allow email match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: bobUser.slackUserId,
			want:     bobUser.teleportUsername,
		},
		{
			name: "no trait match, allow email match - no linked email",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: carolUser.slackUserId,
			wantErr:  true,
		},
		{
			name: "custom trait match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "test_trait_name",
				AllowEmailUsernameMatch: false,
			},
			slackUID: daveUser.slackUserId,
			want:     daveUser.teleportUsername,
		},
		{
			name: "no teleport user",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: false,
			},
			slackUID: "U_slackonly_user",
			wantErr:  true,
		},
		{
			name: "trait matches multiple teleport users",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: false,
			},
			slackUID: eveUser.slackUserId,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		app := &ReviewApp{
			apiClient: authServer,
			bot:       mockBot,
			conf:      tt.reviewConfig,
		}

		got, err := app.resolveTeleportUser(t.Context(), tt.slackUID)
		if tt.wantErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)

		require.Equal(t, tt.want, got)
	}
}
