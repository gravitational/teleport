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
