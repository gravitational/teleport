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
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ARMSubscriptions provides an interface for armsubscription.SubscriptionsClient.
// It is provided so that the client can be mocked.
type ARMSubscriptions interface {
	NewListPager(opts *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
}

var _ ARMSubscriptions = (*armsubscription.SubscriptionsClient)(nil)

// SubscriptionClient wraps the Azure SubscriptionsAPI to fetch subscription IDs.
type SubscriptionClient struct {
	api   ARMSubscriptions
	cache *utils.FnCache
}

// NewSubscriptionClient returns a SubscriptionsClient.
func NewSubscriptionClient(api ARMSubscriptions) (*SubscriptionClient, error) {
	azureSubscriptionCache, err := utils.NewFnCache(utils.FnCacheConfig{
		// Making an API call to list subscriptions at most once per
		// minute is fine and limits the delay before changes to Azure
		// permissions or subscriptions are seen by discovery services.
		TTL: time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SubscriptionClient{
		api:   api,
		cache: azureSubscriptionCache,
	}, nil
}

// ListSubscriptionIDs lists all subscription IDs using the Azure Subscription API.
func (c *SubscriptionClient) ListSubscriptionIDs(ctx context.Context) ([]string, error) {
	ids, err := utils.FnCacheGet(ctx, c.cache, struct{}{}, c.listSubscriptionIDsWithoutCache)
	return slices.Clone(ids), trace.Wrap(err)
}

func (c *SubscriptionClient) listSubscriptionIDsWithoutCache(ctx context.Context) ([]string, error) {
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

// ExpandSubscriptionIDs expands the provided subscription IDs.
// If the list contains a wildcard, it will fetch all accessible subscription IDs from Azure.
func ExpandSubscriptionIDs(ctx context.Context, azureClients Clients, subs []string) ([]string, error) {
	if !slices.Contains(subs, types.Wildcard) {
		return subs, nil
	}

	subscriptionsClient, err := azureClients.GetSubscriptionClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subscriptionIds, err := subscriptionsClient.ListSubscriptionIDs(ctx)
	return subscriptionIds, trace.Wrap(err)
}
