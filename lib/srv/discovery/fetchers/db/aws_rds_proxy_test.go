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
	"testing"

	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestRDSDBProxyFetcher(t *testing.T) {
	t.Parallel()

	rdsProxyVpc1, rdsProxyDatabaseVpc1 := makeRDSProxy(t, "rds-proxy-1", "us-east-1", "vpc1")
	rdsProxyVpc2, rdsProxyDatabaseVpc2 := makeRDSProxy(t, "rds-proxy-2", "us-east-1", "vpc2")
	rdsProxyEndpointVpc1, rdsProxyEndpointDatabaseVpc1 := makeRDSProxyCustomEndpoint(t, rdsProxyVpc1, "endpoint-1", "us-east-1")
	rdsProxyEndpointVpc2, rdsProxyEndpointDatabaseVpc2 := makeRDSProxyCustomEndpoint(t, rdsProxyVpc2, "endpoint-2", "us-east-1")

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBProxies:         []*rds.DBProxy{rdsProxyVpc1, rdsProxyVpc2},
					DBProxyEndpoints:  []*rds.DBProxyEndpoint{rdsProxyEndpointVpc1, rdsProxyEndpointVpc2},
					DBProxyTargetPort: 9999,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherRDSProxy, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{rdsProxyDatabaseVpc1, rdsProxyDatabaseVpc2, rdsProxyEndpointDatabaseVpc1, rdsProxyEndpointDatabaseVpc2},
		},
		{
			name: "fetch vpc1",
			inputClients: &cloud.TestCloudClients{
				RDS: &mocks.RDSMock{
					DBProxies:         []*rds.DBProxy{rdsProxyVpc1, rdsProxyVpc2},
					DBProxyEndpoints:  []*rds.DBProxyEndpoint{rdsProxyEndpointVpc1, rdsProxyEndpointVpc2},
					DBProxyTargetPort: 9999,
				},
			},
			inputMatchers: makeAWSMatchersForType(services.AWSMatcherRDSProxy, "us-east-1", map[string]string{"vpc-id": "vpc1"}),
			wantDatabases: types.Databases{rdsProxyDatabaseVpc1, rdsProxyEndpointDatabaseVpc1},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRDSProxy(t *testing.T, name, region, vpcID string) (*rds.DBProxy, types.Database) {
	rdsProxy := mocks.RDSProxy(name, region, vpcID)
	rdsProxyDatabase, err := services.NewDatabaseFromRDSProxy(rdsProxy, 9999, nil)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(rdsProxyDatabase, services.AWSMatcherRDSProxy)
	return rdsProxy, rdsProxyDatabase
}

func makeRDSProxyCustomEndpoint(t *testing.T, rdsProxy *rds.DBProxy, name, region string) (*rds.DBProxyEndpoint, types.Database) {
	rdsProxyEndpoint := mocks.RDSProxyCustomEndpoint(rdsProxy, name, region)
	rdsProxyEndpointDatabase, err := services.NewDatabaseFromRDSProxyCustomEndpoint(rdsProxy, rdsProxyEndpoint, 9999, nil)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(rdsProxyEndpointDatabase, services.AWSMatcherRDSProxy)
	return rdsProxyEndpoint, rdsProxyEndpointDatabase
}
