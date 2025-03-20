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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestRedshiftServerlessFetcher(t *testing.T) {
	t.Parallel()

	workgroupProd, workgroupProdDB := makeRedshiftServerlessWorkgroup(t, "wg1", "us-east-1", envProdLabels)
	workgroupDev, workgroupDevDB := makeRedshiftServerlessWorkgroup(t, "wg2", "us-east-1", envDevLabels)
	endpointProd, endpointProdDB := makeRedshiftServerlessEndpoint(t, workgroupProd, "endpoint1", "us-east-1", envProdLabels)
	endpointDev, endpointProdDev := makeRedshiftServerlessEndpoint(t, workgroupDev, "endpoint2", "us-east-1", envDevLabels)
	tagsByARN := map[string][]*redshiftserverless.Tag{
		aws.StringValue(workgroupProd.WorkgroupArn): libcloudaws.LabelsToTags[redshiftserverless.Tag](envProdLabels),
		aws.StringValue(workgroupDev.WorkgroupArn):  libcloudaws.LabelsToTags[redshiftserverless.Tag](envDevLabels),
	}

	workgroupNotAvailable := mocks.RedshiftServerlessWorkgroup("wg-creating", "us-east-1")
	workgroupNotAvailable.Status = aws.String("creating")
	endpointNotAvailable := mocks.RedshiftServerlessEndpointAccess(workgroupNotAvailable, "endpoint-creating", "us-east-1")
	endpointNotAvailable.EndpointStatus = aws.String("creating")

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				RedshiftServerless: &mocks.RedshiftServerlessMock{
					Workgroups: []*redshiftserverless.Workgroup{workgroupProd, workgroupDev},
					Endpoints:  []*redshiftserverless.EndpointAccess{endpointProd, endpointDev},
					TagsByARN:  tagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshiftServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{workgroupProdDB, workgroupDevDB, endpointProdDB, endpointProdDev},
		},
		{
			name: "fetch prod",
			inputClients: &cloud.TestCloudClients{
				RedshiftServerless: &mocks.RedshiftServerlessMock{
					Workgroups: []*redshiftserverless.Workgroup{workgroupProd, workgroupDev},
					Endpoints:  []*redshiftserverless.EndpointAccess{endpointProd, endpointDev},
					TagsByARN:  tagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshiftServerless, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{workgroupProdDB, endpointProdDB},
		},
		{
			name: "skip unavailable",
			inputClients: &cloud.TestCloudClients{
				RedshiftServerless: &mocks.RedshiftServerlessMock{
					Workgroups: []*redshiftserverless.Workgroup{workgroupProd, workgroupNotAvailable},
					Endpoints:  []*redshiftserverless.EndpointAccess{endpointNotAvailable},
					TagsByARN:  tagsByARN,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRedshiftServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{workgroupProdDB},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRedshiftServerlessWorkgroup(t *testing.T, name, region string, labels map[string]string) (*redshiftserverless.Workgroup, types.Database) {
	workgroup := mocks.RedshiftServerlessWorkgroup(name, region)
	tags := libcloudaws.LabelsToTags[redshiftserverless.Tag](labels)
	database, err := common.NewDatabaseFromRedshiftServerlessWorkgroup(workgroup, tags)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherRedshiftServerless)
	return workgroup, database
}

func makeRedshiftServerlessEndpoint(t *testing.T, workgroup *redshiftserverless.Workgroup, name, region string, labels map[string]string) (*redshiftserverless.EndpointAccess, types.Database) {
	endpoint := mocks.RedshiftServerlessEndpointAccess(workgroup, name, region)
	tags := libcloudaws.LabelsToTags[redshiftserverless.Tag](labels)
	database, err := common.NewDatabaseFromRedshiftServerlessVPCEndpoint(endpoint, workgroup, tags)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherRedshiftServerless)
	return endpoint, database
}
