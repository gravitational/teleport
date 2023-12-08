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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"
)

// ARMSubscriptions provides an interface for armsubscription.SubscriptionsClient.
// It is provided so that the client can be mocked.
type ARMSubscriptions interface {
	NewListPager(opts *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
}

var _ ARMSubscriptions = (*armsubscription.SubscriptionsClient)(nil)

// SubscriptionClient wraps the Azure SubscriptionsAPI to fetch subscription IDs.
type SubscriptionClient struct {
	api ARMSubscriptions
}

// NewSubscriptionClient returns a SubscriptionsClient.
func NewSubscriptionClient(api ARMSubscriptions) *SubscriptionClient {
	return &SubscriptionClient{api: api}
}

// ListSubscriptionIDs lists all subscription IDs using the Azure Subscription API.
func (c *SubscriptionClient) ListSubscriptionIDs(ctx context.Context) ([]string, error) {
	pagerOpts := &armsubscription.SubscriptionsClientListOptions{}
	pager := c.api.NewListPager(pagerOpts)
	subIDs := []string{}
	for pageNum := 0; pager.More(); pageNum++ {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, v := range res.Value {
			if isValidSubscription(v) {
				subIDs = append(subIDs, *v.SubscriptionID)
			}
		}
	}

	return subIDs, nil
}

func isValidSubscription(subscription *armsubscription.Subscription) bool {
	if subscription == nil || subscription.SubscriptionID == nil || subscription.State == nil {
		return false
	}

	// State "Enabled" and "Past Due": all operations are available.
	// State "Disabled", "Expired", and "Warned": can retrieve or delete resources (GET, DELETE).
	// State "Deleted": No operations are available.
	//
	// https://learn.microsoft.com/en-us/azure/cost-management-billing/manage/subscription-states
	return *subscription.State != armsubscription.SubscriptionStateDeleted
}
