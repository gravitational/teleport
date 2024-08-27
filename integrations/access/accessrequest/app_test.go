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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
)

func TestOpsGenieGetMessageRecipients(t *testing.T) {
	a := App{pluginType: types.PluginTypeOpsgenie, bot: testBot{}}
	ctx := context.Background()
	tests := []struct {
		name               string
		annotations        map[string][]string
		expectedRecipients []common.Recipient
	}{
		{
			name:               "no annotation",
			annotations:        map[string][]string{},
			expectedRecipients: []common.Recipient{},
		},
		{
			name: "just notify-schedules",
			annotations: map[string][]string{
				types.TeleportNamespace + types.ReqAnnotationNotifySchedulesLabel: {"foo", "bar"},
			},
			expectedRecipients: []common.Recipient{
				{
					Name: "foo",
					ID:   "foo",
				},
				{
					Name: "bar",
					ID:   "bar",
				},
			},
		},
		{
			name: "just approval-schedules",
			annotations: map[string][]string{
				types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel: {"foo", "bar"},
			},
			expectedRecipients: []common.Recipient{
				{
					Name: "foo",
					ID:   "foo",
				},
				{
					Name: "bar",
					ID:   "bar",
				},
			},
		},
		{
			name: "both notify and approval schedules",
			annotations: map[string][]string{
				types.TeleportNamespace + types.ReqAnnotationNotifySchedulesLabel:  {"foo", "bar"},
				types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel: {"baz", "hello"},
			},
			expectedRecipients: []common.Recipient{
				{
					Name: "foo",
					ID:   "foo",
				},
				{
					Name: "bar",
					ID:   "bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.AccessRequestV3{
				Spec: types.AccessRequestSpecV3{
					SystemAnnotations: wrappers.Traits(tt.annotations),
				},
			}
			recipients := a.getMessageRecipients(ctx, req)
			require.ElementsMatch(t, tt.expectedRecipients, recipients)
		})
	}

}

type testBot struct {
	MessagingBot
}

func (testBot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
	}, nil
}

type mockTeleportClient struct {
	mock.Mock
	teleport.Client
}

func (m *mockTeleportClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(types.Role), args.Error(1)
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
