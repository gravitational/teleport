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
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
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
			inputClients: &cloud.TestCloudClients{
				Redshift: &mocks.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod, redshiftUse1Dev},
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherRedshift, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod, redshiftDatabaseUse1Dev},
		},
		{
			name: "fetch prod",
			inputClients: &cloud.TestCloudClients{
				Redshift: &mocks.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod, redshiftUse1Dev},
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherRedshift, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod},
		},
		{
			name: "skip unavailable",
			inputClients: &cloud.TestCloudClients{
				Redshift: &mocks.RedshiftMock{
					Clusters: []*redshift.Cluster{redshiftUse1Prod, redshiftUse1Unavailable, redshiftUse1UnknownStatus},
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherRedshift, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{redshiftDatabaseUse1Prod, redshiftDatabaseUnknownStatus},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRedshiftCluster(t *testing.T, region, env string, opts ...func(*redshift.Cluster)) (*redshift.Cluster, types.Database) {
	cluster := &redshift.Cluster{
		ClusterIdentifier:   aws.String(env),
		ClusterNamespaceArn: aws.String(fmt.Sprintf("arn:aws:redshift:%s:123456789012:namespace:%s", region, env)),
		ClusterStatus:       aws.String("available"),
		Endpoint: &redshift.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5439),
		},
		Tags: []*redshift.Tag{{
			Key:   aws.String("env"),
			Value: aws.String(env),
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}

	database, err := services.NewDatabaseFromRedshiftCluster(cluster)
	require.NoError(t, err)
	return cluster, database
}

// withRedshiftStatus returns an option function for makeRedshiftCluster to overwrite status.
func withRedshiftStatus(status string) func(*redshift.Cluster) {
	return func(cluster *redshift.Cluster) {
		cluster.ClusterStatus = aws.String(status)
	}
}
