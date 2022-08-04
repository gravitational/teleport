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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
)

func GetSubscriptions(ctx context.Context, cred azcore.TokenCredential) ([]string, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment,
	// we will need to specify the cloud in these options
	opts := &arm.ClientOptions{}
	client, err := armsubscription.NewSubscriptionsClient(cred, opts)
	if err != nil {
		return nil, common.ConvertError(err)
	}

	pagerOpts := &armsubscription.SubscriptionsClientListOptions{}
	pager := client.NewListPager(pagerOpts)
	subscriptions := []string{}
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, common.ConvertError(err)
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
