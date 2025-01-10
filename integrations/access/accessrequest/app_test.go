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
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
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

type mockMessagingBot struct {
	mock.Mock
	MessagingBot
}

func (m *mockMessagingBot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]string), args.Error(1)
}

func TestGetLoginsByRole(t *testing.T) {
	teleportClient := &mockTeleportClient{}
	teleportClient.On("GetRole", mock.Anything, "admin").Return(&types.RoleV6{
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"root", "foo", "bar"},
			},
		},
	}, (error)(nil))
	teleportClient.On("GetRole", mock.Anything, "foo").Return(&types.RoleV6{
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"foo"},
			},
		},
	}, (error)(nil))
	teleportClient.On("GetRole", mock.Anything, "dev").Return(&types.RoleV6{
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{},
			},
		},
	}, (error)(nil))

	app := App{
		apiClient: teleportClient,
	}
	ctx := context.Background()
	loginsByRole, err := app.getLoginsByRole(ctx, &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			Roles: []string{"admin", "foo", "dev"},
		},
	})
	require.NoError(t, err)

	expected := map[string][]string{
		"admin": {"root", "foo", "bar"},
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
}
