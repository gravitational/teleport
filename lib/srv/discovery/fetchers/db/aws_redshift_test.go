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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestRedshiftFetcher(t *testing.T) {
	t.Parallel()

	redshiftUse1Prod, redshiftDatabaseUse1Prod := makeRedshiftCluster(t, "us-east-1", "prod")
	redshiftUse1Dev, redshiftDatabaseUse1Dev := makeRedshiftCluster(t, "us-east-1", "dev")
	redshiftUse1Unavailable, _ := makeRedshiftCluster(t, "us-east-1", "qa", withRedshiftStatus("paused"))
	redshiftUse1UnknownStatus, redshiftDatabaseUnknownStatus := makeRedshiftCluster(t, "us-east-1", "test", withRedshiftStatus("status-does-not-exist"))

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					redshiftClient: &mocks.RedshiftClient{
						Clusters: []redshifttypes.Cluster{*redshiftUse1Prod, *redshiftUse1Dev},
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshift, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod, redshiftDatabaseUse1Dev},
		},
		{
			name: "fetch prod",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					redshiftClient: &mocks.RedshiftClient{
						Clusters: []redshifttypes.Cluster{*redshiftUse1Prod, *redshiftUse1Dev},
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshift, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod},
		},
		{
			name: "skip unavailable",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					redshiftClient: &mocks.RedshiftClient{
						Clusters: []redshifttypes.Cluster{*redshiftUse1Prod, *redshiftUse1Unavailable, *redshiftUse1UnknownStatus},
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshift, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod, redshiftDatabaseUnknownStatus},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRedshiftCluster(t *testing.T, region, env string, opts ...func(*redshifttypes.Cluster)) (*redshifttypes.Cluster, types.Database) {
	cluster := mocks.RedshiftCluster(env, region, map[string]string{"env": env}, opts...)

	database, err := common.NewDatabaseFromRedshiftCluster(&cluster)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherRedshift)
	return &cluster, database
}

// withRedshiftStatus returns an option function for makeRedshiftCluster to overwrite status.
func withRedshiftStatus(status string) func(*redshifttypes.Cluster) {
	return func(cluster *redshifttypes.Cluster) {
		cluster.ClusterStatus = aws.String(status)
	}
}
