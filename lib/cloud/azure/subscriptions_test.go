/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
					},
					{
						SubscriptionID: to.Ptr("sub2"),
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
