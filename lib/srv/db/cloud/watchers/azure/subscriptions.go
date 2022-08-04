package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"
)

func GetSubscriptions(ctx context.Context, cred azcore.TokenCredential) ([]string, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment,
	// we will need to specify the cloud in these options
	opts := &arm.ClientOptions{}
	client, err := armsubscription.NewSubscriptionsClient(cred, opts)
	if err != nil {
		return nil, trace.Wrap(err) // TODO(gavin): convert from azure error
	}

	pagerOpts := &armsubscription.SubscriptionsClientListOptions{}
	pager := client.NewListPager(pagerOpts)
	subscriptions := []string{}
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err) // TODO(gavin): convert from azure error
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
	return subscriptions, nil
}
