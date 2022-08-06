package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"
)

var _ SubscriptionsClient = (*subscriptionsClient)(nil)

//TODO(gavin)
type subscriptionsClient struct {
	client *armsubscription.SubscriptionsClient
	cache  []string
}

func NewSubscriptionsClient(cred azcore.TokenCredential) (SubscriptionsClient, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment,
	// we will need to specify the cloud in these options
	opts := &arm.ClientOptions{}
	client, err := armsubscription.NewSubscriptionsClient(cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &subscriptionsClient{client: client}, nil
}

//TODO(gavin)
func (c *subscriptionsClient) ListSubscriptions(ctx context.Context, maxPages int, useCache bool) ([]string, error) {
	if useCache && c.cache != nil {
		return c.cache, nil
	}
	pagerOpts := &armsubscription.SubscriptionsClientListOptions{}
	pager := c.client.NewListPager(pagerOpts)
	subscriptions := []string{}
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range res.Value {
			if v != nil && v.SubscriptionID != nil {
				subscriptions = append(subscriptions, *v.SubscriptionID)
			}
		}
	}
	if len(subscriptions) == 0 {
		return nil, trace.NotFound("no azure subscriptions")
	}
	c.cache = subscriptions
	return c.cache, nil
}
