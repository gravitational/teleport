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

package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAssignedIdentitiesClient(t *testing.T) {
	t.Parallel()

	bot1 := NewUserAssignedIdentity("my-sub", "my-group", "bot1", "bot1-id")
	mockAPI := NewARMUserAssignedIdentitiesMock(bot1)

	tests := []struct {
		name                   string
		inputResourceGroupName string
		inputUserName          string
		wantError              bool
		wantClientID           string
	}{
		{
			name:                   "success",
			inputResourceGroupName: "my-group",
			inputUserName:          "bot1",
			wantClientID:           "bot1-id",
		},
		{
			name:                   "not found",
			inputResourceGroupName: "my-group",
			inputUserName:          "bot5",
			wantError:              true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewUserAssignedIdentitiesClientByAPI(mockAPI)
			actualClientID, err := client.GetClientID(context.Background(), test.inputResourceGroupName, test.inputUserName)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantClientID, actualClientID)
		})
	}
}
