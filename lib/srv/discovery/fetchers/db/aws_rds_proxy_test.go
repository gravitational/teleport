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

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
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
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					rdsClient: &mocks.RDSClient{
						DBProxies:        []rdstypes.DBProxy{*rdsProxyVpc1, *rdsProxyVpc2},
						DBProxyEndpoints: []rdstypes.DBProxyEndpoint{*rdsProxyEndpointVpc1, *rdsProxyEndpointVpc2},
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRDSProxy, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{rdsProxyDatabaseVpc1, rdsProxyDatabaseVpc2, rdsProxyEndpointDatabaseVpc1, rdsProxyEndpointDatabaseVpc2},
		},
		{
			name: "fetch vpc1",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					rdsClient: &mocks.RDSClient{
						DBProxies:        []rdstypes.DBProxy{*rdsProxyVpc1, *rdsProxyVpc2},
						DBProxyEndpoints: []rdstypes.DBProxyEndpoint{*rdsProxyEndpointVpc1, *rdsProxyEndpointVpc2},
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherRDSProxy, "us-east-1", map[string]string{"vpc-id": "vpc1"}),
			wantDatabases: types.Databases{rdsProxyDatabaseVpc1, rdsProxyEndpointDatabaseVpc1},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeRDSProxy(t *testing.T, name, region, vpcID string) (*rdstypes.DBProxy, types.Database) {
	rdsProxy := mocks.RDSProxy(name, region, vpcID)
	rdsProxyDatabase, err := common.NewDatabaseFromRDSProxy(rdsProxy, nil)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(rdsProxyDatabase, types.AWSMatcherRDSProxy)
	return rdsProxy, rdsProxyDatabase
}

func makeRDSProxyCustomEndpoint(t *testing.T, rdsProxy *rdstypes.DBProxy, name, region string) (*rdstypes.DBProxyEndpoint, types.Database) {
	rdsProxyEndpoint := mocks.RDSProxyCustomEndpoint(rdsProxy, name, region)
	rdsProxyEndpointDatabase, err := common.NewDatabaseFromRDSProxyCustomEndpoint(rdsProxy, rdsProxyEndpoint, nil)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(rdsProxyEndpointDatabase, types.AWSMatcherRDSProxy)
	return rdsProxyEndpoint, rdsProxyEndpointDatabase
}
