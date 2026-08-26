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

package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/stretchr/testify/require"
)

func TestListSubscriptionIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mockAPI *ARMSubscriptionsMock
		wantIDs []string
	}{
		{
			name: "client lists all subscriptions",
			mockAPI: &ARMSubscriptionsMock{
				Subscriptions: []*armsubscription.Subscription{
					{
						SubscriptionID: to.Ptr("sub1"),
						State:          to.Ptr(armsubscription.SubscriptionStateEnabled),
					},
					{
						SubscriptionID: to.Ptr("sub2"),
						State:          to.Ptr(armsubscription.SubscriptionStateWarned),
					},
					{
						SubscriptionID: to.Ptr("sub3"),
						State:          to.Ptr(armsubscription.SubscriptionStateDeleted),
					},
				},
			},
			wantIDs: []string{"sub1", "sub2"},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSubscriptionClient(tt.mockAPI)

			// verify we get all subscriptions
			subIDs, err := client.ListSubscriptionIDs(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.wantIDs, subIDs)
		})
	}
}
