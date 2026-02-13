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

package accessrequest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

type mockTeleportClient struct {
	mock.Mock
	teleport.Client
}

func (m *mockTeleportClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(types.Role), args.Error(1)
}

func (m *mockTeleportClient) SubmitAccessReview(ctx context.Context, review types.AccessReviewSubmission) (types.AccessRequest, error) {
	review.Review.Created = time.Time{}
	args := m.Called(ctx, review)
	return (types.AccessRequest)(nil), args.Error(1)
}

func (m *mockTeleportClient) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	args := m.Called(ctx, name, withSecrets)
	return args.Get(0).(types.User), args.Error(1)
}

func (m *mockTeleportClient) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	args := m.Called(ctx, name)
	userLoginState, ok := args.Get(0).(*userloginstate.UserLoginState)
	if ok {
		return userLoginState, args.Error(1)
	}
	return nil, args.Error(1)
}

type mockMessagingBot struct {
	mock.Mock
	MessagingBot
}

func (m *mockMessagingBot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]string), args.Error(1)
}

func TestGetLoginsByRoleWithUserLoginState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	teleportClient := &mockTeleportClient{}

	teleportClient.On("GetRole", mock.Anything, "admin").
		Return(&types.RoleV6{
			Metadata: types.Metadata{Name: "admin"},
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"root", "foo", "bar", "{{internal.logins}}"},
				},
			},
		}, nil)

	teleportClient.On("GetUserLoginState", mock.Anything, mock.Anything).
		Return(&userloginstate.UserLoginState{
			Spec: userloginstate.Spec{
				Traits: map[string][]string{
					"logins": {"buz"},
				},
			},
		}, nil)

	app := App{
		apiClient: teleportClient,
	}
	loginsByRole, err := app.getLoginsByRole(ctx, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User:  "admin",
			Roles: []string{"admin"},
		},
	})
	require.NoError(t, err)

	expected := map[string][]string{
		"admin": {"root", "foo", "bar", "buz"},
	}
	require.Equal(t, expected, loginsByRole)
}

func TestGetLoginsByRole(t *testing.T) {
	teleportClient := &mockTeleportClient{}
	teleportClient.On("GetUserLoginState", mock.Anything, mock.Anything).
		Return(nil, trace.AccessDenied("test error"))
	teleportClient.On("GetRole", mock.Anything, "admin").Return(&types.RoleV6{
		Metadata: types.Metadata{Name: "admin"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"root", "foo", "bar", "{{internal.logins}}"},
			},
		},
	}, (error)(nil))
	teleportClient.On("GetRole", mock.Anything, "foo").Return(&types.RoleV6{
		Metadata: types.Metadata{Name: "foo"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"foo"},
			},
		},
	}, (error)(nil))
	teleportClient.On("GetRole", mock.Anything, "dev").Return(&types.RoleV6{
		Metadata: types.Metadata{Name: "dev"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{},
			},
		},
	}, (error)(nil))
	teleportClient.On("GetUser", mock.Anything, "admin", mock.Anything).Return(&types.UserV2{
		Metadata: types.Metadata{Name: "admin"},
		Spec: types.UserSpecV2{
			Traits: map[string][]string{
				"logins": {"buz"},
			},
		},
	}, nil)

	app := App{
		apiClient: teleportClient,
	}
	ctx := context.Background()
	loginsByRole, err := app.getLoginsByRole(ctx, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User:  "admin",
			Roles: []string{"admin", "foo", "dev"},
		},
	})
	require.NoError(t, err)

	expected := map[string][]string{
		"admin": {"root", "foo", "bar", "buz"},
		"foo":   {"foo"},
		"dev":   {},
	}
	require.Equal(t, expected, loginsByRole)
	teleportClient.AssertNumberOfCalls(t, "GetRole", 3)
}

func TestTryApproveRequest(t *testing.T) {
	teleportClient := &mockTeleportClient{}
	bot := &mockMessagingBot{}
	app := App{
		apiClient:    teleportClient,
		bot:          bot,
		teleportUser: "test-access-plugin",
		pluginName:   "test",
	}
	user := "user@example.com"
	requestID := "request-0"

	// Example with user on-call
	bot.On("FetchOncallUsers", mock.Anything, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"team-includes-requester"},
			},
		},
	}).Return([]string{user}, (error)(nil))

	// Example with user not on-call
	bot.On("FetchOncallUsers", mock.Anything, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"team-not-includes-requester"},
			},
		},
	}).Return([]string{"admin@example.com"}, (error)(nil))

	// Example with uppercase user
	bot.On("FetchOncallUsers", mock.Anything, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"uppercase-user"},
			},
		},
	}).Return([]string{strings.ToUpper(user)}, (error)(nil))

	// Successful review
	teleportClient.On("SubmitAccessReview", mock.Anything, types.AccessReviewSubmission{
		RequestID: requestID,
		Review: types.AccessReview{
			Author:        app.teleportUser,
			ProposedState: types.RequestState_APPROVED,
			Reason:        fmt.Sprintf("Access request has been automatically approved by %q plugin because user %q is on-call.", app.pluginName, user),
		},
	}).Return((types.AccessRequest)(nil), (error)(nil))

	ctx := context.Background()

	// Test user is on-call
	require.NoError(t, app.tryApproveRequest(ctx, requestID, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"team-includes-requester"},
			},
		},
	}))
	bot.AssertNumberOfCalls(t, "FetchOncallUsers", 1)
	teleportClient.AssertNumberOfCalls(t, "SubmitAccessReview", 1)

	// Test user is not on-call
	require.NoError(t, app.tryApproveRequest(ctx, requestID, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"team-not-includes-requester"},
			},
		},
	}))
	bot.AssertNumberOfCalls(t, "FetchOncallUsers", 2)
	teleportClient.AssertNumberOfCalls(t, "SubmitAccessReview", 1)

	// Test user is on-call ignore casing
	require.NoError(t, app.tryApproveRequest(ctx, requestID, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: user,
			SystemAnnotations: map[string][]string{
				"example-auto-approvals": {"uppercase-user"},
			},
		},
	}))
	bot.AssertNumberOfCalls(t, "FetchOncallUsers", 3)
	teleportClient.AssertNumberOfCalls(t, "SubmitAccessReview", 2)
}

func TestPopulateRequestTimeFields(t *testing.T) {
	now := time.Date(2026, 2, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		requestExpiry     time.Time
		accessExpiry      time.Time
		wantRequestTTL    string
		wantAccessTTL     string
		wantRequestExpiry bool
		wantAccessExpiry  bool
	}{
		{
			name:              "future request and access expiries set ttl and expiry fields",
			requestExpiry:     now.Add(2*time.Hour + 31*time.Second),
			accessExpiry:      now.Add(8*time.Hour + 29*time.Second),
			wantRequestTTL:    "2h1m0s",
			wantAccessTTL:     "8h0m0s",
			wantRequestExpiry: true,
			wantAccessExpiry:  true,
		},
		{
			name:              "past expiries keep expiry fields without ttl",
			requestExpiry:     now.Add(-2 * time.Hour),
			accessExpiry:      now.Add(-30 * time.Minute),
			wantRequestExpiry: true,
			wantAccessExpiry:  true,
		},
		{
			name: "unset expiries remain empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := types.NewAccessRequest("req-id", "user", "role")
			require.NoError(t, err)
			if !tt.requestExpiry.IsZero() {
				req.SetExpiry(tt.requestExpiry)
			}
			if !tt.accessExpiry.IsZero() {
				req.SetAccessExpiry(tt.accessExpiry)
			}

			data := pd.AccessRequestData{}
			app := &App{}
			app.populateRequestTimeFields(req, now, &data)

			require.Equal(t, tt.wantRequestTTL, data.RequestTTL)
			require.Equal(t, tt.wantAccessTTL, data.AccessTTL)
			require.Equal(t, tt.wantRequestExpiry, data.RequestExpiry != nil)
			require.Equal(t, tt.wantAccessExpiry, data.AccessExpiry != nil)
			if tt.wantRequestExpiry {
				require.True(t, tt.requestExpiry.Equal(*data.RequestExpiry))
			}
			if tt.wantAccessExpiry {
				require.True(t, tt.accessExpiry.Equal(*data.AccessExpiry))
			}
		})
	}
}
