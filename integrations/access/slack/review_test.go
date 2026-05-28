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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

type slackUser struct {
	uid   string
	email string
}

type teleportUser struct {
	name   string
	traits map[string][]string
}

func TestResolveTeleportUser(t *testing.T) {
	authServer := newTestAuth(t, modulestest.OSSModules())

	// Create Slack users
	aliceSlackUser := slackUser{
		uid: "U_alice",
	}
	bobSlackUser := slackUser{
		uid:   "U_bob",
		email: "bob@example.com",
	}
	carolSlackUser := slackUser{
		uid: "U_carol",
	}
	daveSlackUser := slackUser{
		uid: "U_dave",
	}

	// Link some emails to Slack users
	mockBot := &mockReviewBot{}
	mockBot.On("LookupEmailByUserID", mock.Anything, aliceSlackUser.uid).Return("", nil)
	mockBot.On("LookupEmailByUserID", mock.Anything, bobSlackUser.uid).Return(bobSlackUser.email, nil)
	mockBot.On("LookupEmailByUserID", mock.Anything, carolSlackUser.uid).Return("", nil)
	mockBot.On("LookupEmailByUserID", mock.Anything, daveSlackUser.uid).Return("", nil)

	// Create Teleport users with traits
	aliceTeleportUser := teleportUser{
		name:   "alice",
		traits: map[string][]string{"slack_uid": {aliceSlackUser.uid}},
	}
	bobTeleportUser := teleportUser{
		name:   "bob@example.com",
		traits: nil,
	}
	carolTeleportUser := teleportUser{
		name:   "carol",
		traits: nil,
	}
	daveTeleportUser := teleportUser{
		name:   "dave",
		traits: map[string][]string{"test_trait_name": {daveSlackUser.uid}},
	}
	createTeleportUserWithTraits(t, authServer, aliceTeleportUser)
	createTeleportUserWithTraits(t, authServer, bobTeleportUser)
	createTeleportUserWithTraits(t, authServer, carolTeleportUser)
	createTeleportUserWithTraits(t, authServer, daveTeleportUser)

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
			slackUID: aliceSlackUser.uid,
			want:     aliceTeleportUser.name,
		},
		{
			name: "trait match, allow email match - no linked email",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: aliceSlackUser.uid,
			want:     aliceTeleportUser.name,
		},
		{
			name: "no trait match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: false,
			},
			slackUID: bobSlackUser.uid,
			wantErr:  true,
		},
		{
			name: "no trait match, allow email match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: bobSlackUser.uid,
			want:     bobTeleportUser.name,
		},
		{
			name: "no trait match, allow email match - no linked email",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "slack_uid",
				AllowEmailUsernameMatch: true,
			},
			slackUID: carolSlackUser.uid,
			wantErr:  true,
		},
		{
			name: "custom trait match",
			reviewConfig: ReviewConfig{
				SlackUserIDTrait:        "test_trait_name",
				AllowEmailUsernameMatch: false,
			},
			slackUID: daveSlackUser.uid,
			want:     daveTeleportUser.name,
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

func createTeleportUserWithTraits(t *testing.T, authServer *auth.Server, teleportUser teleportUser) {
	t.Helper()

	user, err := types.NewUser(teleportUser.name)
	require.NoError(t, err)
	user.SetTraits(teleportUser.traits)

	_, err = authServer.CreateUser(t.Context(), user)
	require.NoError(t, err)
}
