/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package servicenow

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockServicenow struct {
	ServiceNowClient

	responses map[string][]string
}

func (msn mockServicenow) GetOnCall(ctx context.Context, rotaID string) ([]string, error) {
	if ret, ok := msn.responses[rotaID]; ok {
		return ret, nil
	}
	return nil, trace.NotFound("someError")
}

func TestGetOnCallUsers(t *testing.T) {
	a := App{
		serviceNow: mockServicenow{
			responses: map[string][]string{
				"rota1": {"user1", "user2"},
				"rota2": {"user3", "user2"},
			},
		},
	}
	users, err := a.getOnCallUsers(context.Background(), []string{"rota1"})
	require.NoError(t, err)
	require.Equal(t, []string{"user1", "user2"}, users)

	users, err = a.getOnCallUsers(context.Background(), []string{"rota1", "rota3", "rota2"})
	require.NoError(t, err)
	require.Equal(t, []string{"user1", "user2", "user3", "user2"}, users)
}
