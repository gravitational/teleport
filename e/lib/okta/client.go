/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
)

var _ oktaClient = (*wrappedClient)(nil)

// wrappedClient is a client type that is backed by an Okta SDK client.
type wrappedClient struct {
	client     *okta.Client
	oktaOrgURL string
}

func (w *wrappedClient) iterateGroups(ctx context.Context, fn func(*okta.Group) error) error {
	// The default page size is 10000 here, which is fine for our purposes.
	// https://developer.okta.com/docs/reference/api/groups/#list-groups
	oktaGroups, resp, err := w.client.Group.ListGroups(ctx, query.NewQueryParams())
	for {
		if err != nil {
			return trace.Wrap(err, "error when iterating through groups")
		}

		for _, oktaGroup := range oktaGroups {
			if err := fn(oktaGroup); err != nil {
				return trace.Wrap(err)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &oktaGroups)
	}

	return nil
}

func (w *wrappedClient) iterateApps(ctx context.Context, fn func(okta.App) error) error {
	// The default for application listing is 20 per page. Here we'll bump it
	// to the max of 200 per page to minimize API calls.
	// https://developer.okta.com/docs/reference/api/apps/#list-applications
	oktaApps, resp, err := w.client.Application.ListApplications(ctx, query.NewQueryParams(
		query.WithLimit(200),
	))
	for {
		if err != nil {
			return trace.Wrap(err, "error when iterating through apps")
		}

		for _, oktaApp := range oktaApps {
			if err := fn(oktaApp); err != nil {
				return trace.Wrap(err)
			}
		}

		if !resp.HasNextPage() {
			break
		}

		resp, err = resp.Next(ctx, &oktaApps)
	}

	return nil
}

func (w *wrappedClient) orgURL() string {
	return w.oktaOrgURL
}
