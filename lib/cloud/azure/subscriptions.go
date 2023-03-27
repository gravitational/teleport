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
