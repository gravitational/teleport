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
	"github.com/gravitational/teleport/lib/cloud/mocks"
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

func makeAWSMatchersForType(matcherType, region string, tags map[string]string) []types.AWSMatcher {
	return []types.AWSMatcher{{
		Types:   []string{matcherType},
		Regions: []string{region},
		Tags:    toTypeLabels(tags),
	}}
}

func mustMakeAWSFetchers(t *testing.T, clients cloud.AWSClients, matchers []types.AWSMatcher) []common.Fetcher {
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

func mustMakeAzureFetchers(t *testing.T, clients cloud.AzureClients, matchers []types.AzureMatcher) []common.Fetcher {
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

// testAssumeRole is a fixture for testing fetchers.
// every matcher, stub database, and mock AWS Session created uses this fixture.
// Tests will cover:
//   - that fetchers use the configured assume role when using AWS cloud clients.
//   - that databases discovered and created by fetchers have the assumed role used to discover them populated.
var testAssumeRole = types.AssumeRole{
	RoleARN:    "arn:aws:iam::123456789012:role/test-role",
	ExternalID: "externalID123",
}

// awsFetcherTest is a common test struct for AWS fetchers.
type awsFetcherTest struct {
	name          string
	inputClients  *cloud.TestCloudClients
	inputMatchers []types.AWSMatcher
	wantDatabases types.Databases
}

// testAWSFetchers is a helper that tests AWS fetchers, since
// all of the AWS fetcher tests are fundamentally the same.
func testAWSFetchers(t *testing.T, tests ...awsFetcherTest) {
	t.Helper()
	for _, test := range tests {
		test := test
		require.Nil(t, test.inputClients.STS, "testAWSFetchers injects an STS mock itself, but test input had already configured it. This is a test configuration error.")
		stsMock := &mocks.STSMock{}
		test.inputClients.STS = stsMock
		t.Run(test.name, func(t *testing.T) {
			t.Helper()
			fetchers := mustMakeAWSFetchers(t, test.inputClients, test.inputMatchers)
			require.ElementsMatch(t, test.wantDatabases, mustGetDatabases(t, fetchers))
		})
		t.Run(test.name+" with assume role", func(t *testing.T) {
			t.Helper()
			matchers := copyAWSMatchersWithAssumeRole(testAssumeRole, test.inputMatchers...)
			wantDBs := copyDatabasesWithAWSAssumeRole(testAssumeRole, test.wantDatabases...)
			fetchers := mustMakeAWSFetchers(t, test.inputClients, matchers)
			require.ElementsMatch(t, wantDBs, mustGetDatabases(t, fetchers))
			require.Equal(t, []string{testAssumeRole.RoleARN}, stsMock.GetAssumedRoleARNs())
			require.Equal(t, []string{testAssumeRole.ExternalID}, stsMock.GetAssumedRoleExternalIDs())
		})
	}
}

// copyDatabasesWithAWSAssumeRole copies input databases and sets a given AWS assume role for each copy.
func copyDatabasesWithAWSAssumeRole(role types.AssumeRole, databases ...types.Database) types.Databases {
	if len(databases) == 0 {
		return databases
	}
	out := make(types.Databases, 0, len(databases))
	for _, db := range databases {
		out = append(out, db.Copy())
	}
	applyAssumeRoleToDatabases(out, role)
	return out
}

// copyAWSMatchersWithAssumeRole copies input AWS matchers and sets a given AWS assume role for each copy.
func copyAWSMatchersWithAssumeRole(role types.AssumeRole, matchers ...types.AWSMatcher) []types.AWSMatcher {
	if len(matchers) == 0 {
		return matchers
	}
	out := make([]types.AWSMatcher, 0, len(matchers))
	for _, m := range matchers {
		m.AssumeRole = &role
		out = append(out, m)
	}
	return out
}
