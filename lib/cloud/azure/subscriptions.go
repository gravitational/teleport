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

	"github.com/gravitational/trace"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

// SubscriptionIDsClient wraps the Azure SubscriptionsAPI to fetch subscription IDs
type SubscriptionIDsClient struct {
	api   SubscriptionsAPI
	cache []string
}

// NewSubscriptionIDsClient returns a SubscriptionsClient
func NewSubscriptionIDsClient(api SubscriptionsAPI) *SubscriptionIDsClient {
	return &SubscriptionIDsClient{api: api}
}

// ListSubscriptionIDs lists all subscription IDs using the Azure SubscriptionsAPI
func (c *SubscriptionIDsClient) ListSubscriptionIDs(ctx context.Context, maxPages int, useCache bool) ([]string, error) {
	if useCache && c.cache != nil {
		return c.cache, nil
	}
	pagerOpts := &armsubscription.SubscriptionsClientListOptions{}
	pager := c.api.NewListPager(pagerOpts)
	subIDs := []string{}
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, ConvertResponseError(err)
		}
		for _, v := range res.Value {
			if v != nil && v.SubscriptionID != nil {
				subIDs = append(subIDs, *v.SubscriptionID)
			}
		}
	}
	if len(subIDs) == 0 {
		return nil, trace.NotFound("no azure subscriptions")
	}

	c.cache = subIDs
	return c.cache, nil
}
