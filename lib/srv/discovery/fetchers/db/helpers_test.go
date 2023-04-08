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

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

var (
	wildcardLabels = map[string]string{types.Wildcard: types.Wildcard}
	envProdLabels  = map[string]string{"env": "prod"}
	envDevLabels   = map[string]string{"env": "dev"}
)

func toTypeLabels(labels map[string]string) types.Labels {
	result := make(types.Labels)
	for key, value := range labels {
		result[key] = utils.Strings{value}
	}
	return result
}

func mustMakeAWSFetchers(t *testing.T, clients cloud.AWSClients, matchers []services.AWSMatcher) []common.Fetcher {
	t.Helper()

	fetchers, err := MakeAWSFetchers(context.Background(), clients, matchers)
	require.NoError(t, err)
	require.NotEmpty(t, fetchers)

	for _, fetcher := range fetchers {
		require.Equal(t, types.KindDatabase, fetcher.ResourceType())
		require.Equal(t, types.CloudAWS, fetcher.Cloud())
	}
	return fetchers
}

func mustMakeAWSFetchersForMatcher(t *testing.T, clients cloud.AWSClients, matcherType, region string, tags types.Labels) []common.Fetcher {
	t.Helper()

	return mustMakeAWSFetchers(t, clients, []services.AWSMatcher{{
		Types:   []string{matcherType},
		Regions: []string{region},
		Tags:    tags,
	}})
}

func mustMakeAzureFetchers(t *testing.T, clients cloud.AzureClients, matchers []services.AzureMatcher) []common.Fetcher {
	t.Helper()

	fetchers, err := MakeAzureFetchers(clients, matchers)
	require.NoError(t, err)
	require.NotEmpty(t, fetchers)

	for _, fetcher := range fetchers {
		require.Equal(t, types.KindDatabase, fetcher.ResourceType())
		require.Equal(t, types.CloudAzure, fetcher.Cloud())
	}
	return fetchers
}

func mustGetDatabases(t *testing.T, fetchers []common.Fetcher) types.Databases {
	t.Helper()

	var all types.Databases
	for _, fetcher := range fetchers {
		resources, err := fetcher.Get(context.TODO())
		require.NoError(t, err)

		databases, err := resources.AsDatabases()
		require.NoError(t, err)

		all = append(all, databases...)
	}
	return all
}
