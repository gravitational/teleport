/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestElastiCacheServerlessFetcher(t *testing.T) {
	t.Parallel()
	subnets := []ec2types.Subnet{
		{SubnetId: aws.String("subnet-a"), VpcId: aws.String("vpc1")},
		{SubnetId: aws.String("subnet-b"), VpcId: aws.String("vpc1")},
		{SubnetId: aws.String("subnet-c"), VpcId: aws.String("vpc2")},
		{SubnetId: aws.String("subnet-d"), VpcId: aws.String("vpc2")},
	}
	subnetMap := map[string]ec2types.Subnet{}
	for _, s := range subnets {
		subnetMap[*s.SubnetId] = s
	}
	elasticacheProd, elasticacheDatabaseProd, elasticacheProdTags := makeElastiCacheServerlessCache(t, "ec1", "us-east-1", "prod", subnetMap)
	elasticacheDatabaseProdWithoutVPCLabel := elasticacheDatabaseProd.Copy()
	{
		labels := elasticacheDatabaseProdWithoutVPCLabel.GetAllLabels()
		require.Contains(t, labels, types.DiscoveryLabelVPCID)
		delete(labels, types.DiscoveryLabelVPCID)
	}
	elasticacheQA, elasticacheDatabaseQA, elasticacheQATags := makeElastiCacheServerlessCache(t, "ec2", "us-east-1", "qa", subnetMap, withElastiCacheServerlessEngine("valkey"))
	elasticacheUnavailable, _, elasticacheUnavailableTags := makeElastiCacheServerlessCache(t, "ec4", "us-east-1", "prod", subnetMap, func(cache *ectypes.ServerlessCache) {
		cache.Status = aws.String("deleting")
	})
	elasticacheUnsupported, _, elasticacheUnsupportedTags := makeElastiCacheServerlessCache(t, "ec5", "us-east-1", "prod", subnetMap, func(cache *ectypes.ServerlessCache) {
		cache.Engine = aws.String("memcached")
	})
	elasticacheTagsByARN := map[string][]ectypes.Tag{
		aws.ToString(elasticacheProd.ARN):        elasticacheProdTags,
		aws.ToString(elasticacheQA.ARN):          elasticacheQATags,
		aws.ToString(elasticacheUnavailable.ARN): elasticacheUnavailableTags,
		aws.ToString(elasticacheUnsupported.ARN): elasticacheUnsupportedTags,
	}

	var paginatedCaches []ectypes.ServerlessCache
	var paginatedDatabases types.Databases
	for i := range maxAWSPages + 1 {
		name := fmt.Sprintf("cache-%d", i)
		cache, db, tags := makeElastiCacheServerlessCache(t, name, "us-east-1", "prod", subnetMap)
		paginatedCaches = append(paginatedCaches, *cache)
		paginatedDatabases = append(paginatedDatabases, db)
		elasticacheTagsByARN[*cache.ARN] = tags
	}

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets},
					ecClient: &mocks.ElastiCacheClient{
						Caches:    []ectypes.ServerlessCache{*elasticacheProd, *elasticacheQA},
						TagsByARN: elasticacheTagsByARN,
					}},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{elasticacheDatabaseProd, elasticacheDatabaseQA},
		},
		{
			name: "limits max pages",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets},
					ecClient: &mocks.ElastiCacheClient{
						PaginateCaches: true,
						Caches:         paginatedCaches,
						TagsByARN:      elasticacheTagsByARN,
					}},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", wildcardLabels),
			wantDatabases: paginatedDatabases[:maxAWSPages],
		},
		{
			name: "fetch prod",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets},
					ecClient: &mocks.ElastiCacheClient{
						Caches:    []ectypes.ServerlessCache{*elasticacheProd, *elasticacheQA},
						TagsByARN: elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", envProdLabels),
			wantDatabases: types.Databases{elasticacheDatabaseProd},
		},
		{
			name: "skip unavailable",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets},
					ecClient: &mocks.ElastiCacheClient{
						Caches:    []ectypes.ServerlessCache{*elasticacheProd, *elasticacheUnavailable},
						TagsByARN: elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{elasticacheDatabaseProd},
		},
		{
			name: "skip unsupported",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets},
					ecClient: &mocks.ElastiCacheClient{
						Caches:    []ectypes.ServerlessCache{*elasticacheProd, *elasticacheUnsupported},
						TagsByARN: elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{elasticacheDatabaseProd},
		},
		{
			name: "skip vpc label on error listing subnets",
			fetcherCfg: AWSFetcherFactoryConfig{
				AWSClients: fakeAWSClients{
					ec2Client: fakeEC2Client{subnets: subnets, fakeErr: errors.New("operation failed due to alpaca stampede")},
					ecClient: &mocks.ElastiCacheClient{
						Caches:    []ectypes.ServerlessCache{*elasticacheProd, *elasticacheUnsupported},
						TagsByARN: elasticacheTagsByARN,
					},
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherElastiCacheServerless, "us-east-1", wildcardLabels),
			wantDatabases: types.Databases{elasticacheDatabaseProdWithoutVPCLabel},
		},
	}
	testAWSFetchers(t, tests...)
}

func makeElastiCacheServerlessCache(t *testing.T, name, region, env string, subnets map[string]ec2types.Subnet, opts ...func(*ectypes.ServerlessCache)) (*ectypes.ServerlessCache, types.Database, []ectypes.Tag) {
	t.Helper()
	cache := mocks.ElastiCacheServerless(name, region, opts...)

	tags := []ectypes.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}

	extraLabels := common.ExtraElastiCacheServerlessLabels(cache, tags, subnets)
	database, err := common.NewDatabaseFromElastiCacheServerlessCache(cache, extraLabels)
	require.NoError(t, err)
	common.ApplyAWSDatabaseNameSuffix(database, types.AWSMatcherElastiCacheServerless)
	return cache, database, tags
}

func withElastiCacheServerlessEngine(engine string) func(*ectypes.ServerlessCache) {
	return func(cache *ectypes.ServerlessCache) {
		cache.Engine = &engine
	}
}

type fakeEC2Client struct {
	subnets []ec2types.Subnet
	fakeErr error
}

// Returns information about AWS VPC subnets.
// This API supports pagination.
func (f fakeEC2Client) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if f.fakeErr != nil {
		return nil, f.fakeErr
	}
	if input.SubnetIds == nil {
		token := aws.ToString(input.NextToken)
		for i, subnet := range f.subnets {
			if token != "" && aws.ToString(subnet.SubnetId) != token {
				continue
			}
			out := &ec2.DescribeSubnetsOutput{
				Subnets: []ec2types.Subnet{subnet},
			}
			if i+1 < len(f.subnets) {
				out.NextToken = f.subnets[i+1].SubnetId
			}
			return out, nil
		}
		return &ec2.DescribeSubnetsOutput{}, nil
	}
	out := &ec2.DescribeSubnetsOutput{}
	for _, subnet := range f.subnets {
		if slices.Contains(input.SubnetIds, aws.ToString(subnet.SubnetId)) {
			out.Subnets = append(out.Subnets, subnet)
		}
	}
	return out, nil
}
