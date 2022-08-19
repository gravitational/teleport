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

type ARMSubscriptionsMock struct {
	Subscriptions []*armsubscription.Subscription
	NoAuth        bool
}

var _ ARMSubscriptions = (*ARMSubscriptionsMock)(nil)

func (m *ARMSubscriptionsMock) NewListPager(_ *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armsubscription.SubscriptionsClientListResponse]{
		More: func(page armsubscription.SubscriptionsClientListResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(ctx context.Context, page *armsubscription.SubscriptionsClientListResponse) (armsubscription.SubscriptionsClientListResponse, error) {
			if m.NoAuth {
				return armsubscription.SubscriptionsClientListResponse{}, trace.AccessDenied("unauthorized")
			}
			return armsubscription.SubscriptionsClientListResponse{
				ListResult: armsubscription.ListResult{
					Value: m.Subscriptions,
				},
			}, nil
		},
	})
}
