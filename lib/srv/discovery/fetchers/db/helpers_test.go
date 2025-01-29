/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

func mustMakeAWSFetchers(t *testing.T, cfg AWSFetcherFactoryConfig, matchers []types.AWSMatcher, discoveryConfigName string) []common.Fetcher {
	t.Helper()

	fetcherFactory, err := NewAWSFetcherFactory(cfg)
	require.NoError(t, err)
	fetchers, err := fetcherFactory.MakeFetchers(context.Background(), matchers, discoveryConfigName)
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

	fetchers, err := MakeAzureFetchers(clients, matchers, "" /* discovery config */)
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
	fetcherCfg    AWSFetcherFactoryConfig
	inputMatchers []types.AWSMatcher
	wantDatabases types.Databases
}

// testAWSFetchers is a helper that tests AWS fetchers, since
// all of the AWS fetcher tests are fundamentally the same.
func testAWSFetchers(t *testing.T, tests ...awsFetcherTest) {
	t.Helper()
	for _, test := range tests {
		test := test
		fakeSTS := &mocks.STSClient{}
		if test.inputClients != nil {
			require.Nil(t, test.inputClients.STS, "testAWSFetchers injects an STS mock itself, but test input had already configured it. This is a test configuration error.")
			test.inputClients.STS = &fakeSTS.STSClientV1
		}
		test.fetcherCfg.CloudClients = test.inputClients
		require.Nil(t, test.fetcherCfg.AWSConfigProvider, "testAWSFetchers injects a fake AWSConfigProvider, but the test input had already configured it. This is a test configuration error.")
		test.fetcherCfg.AWSConfigProvider = &mocks.AWSConfigProvider{
			STSClient: fakeSTS,
		}
		t.Run(test.name, func(t *testing.T) {
			t.Helper()
			fetchers := mustMakeAWSFetchers(t, test.fetcherCfg, test.inputMatchers, "" /* discovery config */)
			require.ElementsMatch(t, test.wantDatabases, mustGetDatabases(t, fetchers))
		})
		t.Run(test.name+" with assume role", func(t *testing.T) {
			t.Helper()
			fakeSTS.ResetAssumeRoleHistory()
			matchers := copyAWSMatchersWithAssumeRole(testAssumeRole, test.inputMatchers...)
			wantDBs := copyDatabasesWithAWSAssumeRole(testAssumeRole, test.wantDatabases...)
			fetchers := mustMakeAWSFetchers(t, test.fetcherCfg, matchers, "" /* discovery config */)
			require.ElementsMatch(t, wantDBs, mustGetDatabases(t, fetchers))
			require.Equal(t, []string{testAssumeRole.RoleARN}, fakeSTS.GetAssumedRoleARNs())
			require.Equal(t, []string{testAssumeRole.ExternalID}, fakeSTS.GetAssumedRoleExternalIDs())
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
		dbCopy := db.Copy()
		dbCopy.SetAWSAssumeRole(role.RoleARN)
		dbCopy.SetAWSExternalID(role.ExternalID)
		out = append(out, dbCopy)
	}
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
