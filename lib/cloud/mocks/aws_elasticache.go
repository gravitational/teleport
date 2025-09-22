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

package mocks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/gravitational/trace"
)

// ElastiCache mocks AWS ElastiCache API.
type ElastiCacheClient struct {
	// Unauth set to true will make API calls return unauthorized errors.
	Unauth bool

	// PaginateCaches will return each serverless cache in a page by itself, to fake a multi-page response.
	PaginateCaches    bool
	Caches            []ectypes.ServerlessCache
	ReplicationGroups []ectypes.ReplicationGroup
	Users             []ectypes.User
	TagsByARN         map[string][]ectypes.Tag
}

func (m *ElastiCacheClient) AddMockUser(user ectypes.User, tagsMap map[string]string) {
	m.Users = append(m.Users, user)
	m.addTags(aws.ToString(user.ARN), tagsMap)
}

func (m *ElastiCacheClient) addTags(arn string, tagsMap map[string]string) {
	if m.TagsByARN == nil {
		m.TagsByARN = make(map[string][]ectypes.Tag)
	}

	var tags []ectypes.Tag
	for key, value := range tagsMap {
		tags = append(tags, ectypes.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	m.TagsByARN[arn] = tags
}

func (m *ElastiCacheClient) DescribeUsers(_ context.Context, input *elasticache.DescribeUsersInput, opts ...func(*elasticache.Options)) (*elasticache.DescribeUsersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if input.UserId == nil {
		return &elasticache.DescribeUsersOutput{Users: m.Users}, nil
	}
	for _, user := range m.Users {
		if aws.ToString(user.UserId) == aws.ToString(input.UserId) {
			return &elasticache.DescribeUsersOutput{Users: []ectypes.User{user}}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache UserId %q not found", aws.ToString(input.UserId))
}

func (m *ElastiCacheClient) DescribeReplicationGroups(_ context.Context, input *elasticache.DescribeReplicationGroupsInput, opts ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if input.ReplicationGroupId == nil {
		return &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: m.ReplicationGroups,
		}, nil
	}
	for _, replicationGroup := range m.ReplicationGroups {
		if aws.ToString(replicationGroup.ReplicationGroupId) == aws.ToString(input.ReplicationGroupId) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []ectypes.ReplicationGroup{replicationGroup},
			}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache ReplicationGroupId %q not found", aws.ToString(input.ReplicationGroupId))
}

func (m *ElastiCacheClient) DescribeCacheClusters(context.Context, *elasticache.DescribeCacheClustersInput, ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return nil, trace.NotImplemented("elasticache:DescribeCacheClusters is not implemented")
}

func (m *ElastiCacheClient) DescribeCacheSubnetGroups(context.Context, *elasticache.DescribeCacheSubnetGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return nil, trace.NotImplemented("elasticache:DescribeCacheSubnetGroups is not implemented")
}

func (m *ElastiCacheClient) ListTagsForResource(_ context.Context, input *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return nil, trace.NotFound("no tags")
	}

	tags, ok := m.TagsByARN[aws.ToString(input.ResourceName)]
	if !ok {
		return nil, trace.NotFound("no tags")
	}

	return &elasticache.ListTagsForResourceOutput{
		TagList: tags,
	}, nil
}

func (m *ElastiCacheClient) ModifyUser(_ context.Context, input *elasticache.ModifyUserInput, opts ...func(*elasticache.Options)) (*elasticache.ModifyUserOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, user := range m.Users {
		if aws.ToString(user.UserId) == aws.ToString(input.UserId) {
			return &elasticache.ModifyUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache UserId %q not found", aws.ToString(input.UserId))
}

func (m *ElastiCacheClient) DescribeServerlessCaches(_ context.Context, input *elasticache.DescribeServerlessCachesInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeServerlessCachesOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if input.ServerlessCacheName == nil {
		if !m.PaginateCaches {
			return &elasticache.DescribeServerlessCachesOutput{
				ServerlessCaches: m.Caches,
			}, nil
		}
		token := aws.ToString(input.NextToken)
		for i, cache := range m.Caches {
			if token != "" && aws.ToString(cache.ServerlessCacheName) != token {
				continue
			}
			out := &elasticache.DescribeServerlessCachesOutput{
				ServerlessCaches: []ectypes.ServerlessCache{cache},
			}
			if i+1 < len(m.Caches) {
				out.NextToken = m.Caches[i+1].ServerlessCacheName
			}
			return out, nil
		}
		return &elasticache.DescribeServerlessCachesOutput{}, nil
	}
	for _, cache := range m.Caches {
		if aws.ToString(cache.ServerlessCacheName) == aws.ToString(input.ServerlessCacheName) {
			return &elasticache.DescribeServerlessCachesOutput{
				ServerlessCaches: []ectypes.ServerlessCache{cache},
			}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache Serverless Cache %q not found", aws.ToString(input.ServerlessCacheName))
}

// ElastiCacheCluster returns a sample ectypes.ReplicationGroup.
func ElastiCacheCluster(name, region string, opts ...func(*ectypes.ReplicationGroup)) *ectypes.ReplicationGroup {
	cluster := &ectypes.ReplicationGroup{
		ARN:                      aws.String(fmt.Sprintf("arn:aws:elasticache:%s:123456789012:replicationgroup:%s", region, name)),
		Engine:                   aws.String("redis"),
		ReplicationGroupId:       aws.String(name),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),

		// Default has one primary endpoint in the only node group.
		NodeGroups: []ectypes.NodeGroup{{
			PrimaryEndpoint: &ectypes.Endpoint{
				Address: aws.String(fmt.Sprintf("master.%v-cluster.xxxxxx.use1.cache.amazonaws.com", name)),
				Port:    aws.Int32(6379),
			},
		}},
	}

	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}

// WithElastiCacheReaderEndpoint is an option function for
// MakeElastiCacheCluster to set a reader endpoint.
func WithElastiCacheReaderEndpoint(cluster *ectypes.ReplicationGroup) {
	cluster.NodeGroups = append(cluster.NodeGroups, ectypes.NodeGroup{
		ReaderEndpoint: &ectypes.Endpoint{
			Address: aws.String(fmt.Sprintf("replica.%v-cluster.xxxxxx.use1.cache.amazonaws.com", aws.ToString(cluster.ReplicationGroupId))),
			Port:    aws.Int32(6379),
		},
	})
}

// WithElastiCacheConfigurationEndpoint in an option function for
// MakeElastiCacheCluster to set a configuration endpoint.
func WithElastiCacheConfigurationEndpoint(cluster *ectypes.ReplicationGroup) {
	cluster.ClusterEnabled = aws.Bool(true)
	cluster.ConfigurationEndpoint = &ectypes.Endpoint{
		Address: aws.String(fmt.Sprintf("clustercfg.%v-shards.xxxxxx.use1.cache.amazonaws.com", aws.ToString(cluster.ReplicationGroupId))),
		Port:    aws.Int32(6379),
	}
}

// ElastiCacheServerless returns a sample ectypes.ServerlessCache.
func ElastiCacheServerless(name, region string, opts ...func(*ectypes.ServerlessCache)) *ectypes.ServerlessCache {
	addr := fmt.Sprintf("%s-abc123.serverless.%s.cache.amazonaws.com", name, regionToShortRegion(region))
	arn := fmt.Sprintf("arn:aws:elasticache:%s:123456789012:serverlesscache:%s", region, name)
	cache := &ectypes.ServerlessCache{
		ARN:                 aws.String(arn),
		Engine:              aws.String("redis"),
		ServerlessCacheName: aws.String(name),
		Status:              aws.String("available"),
		Endpoint: &ectypes.Endpoint{
			Address: aws.String(addr),
			Port:    aws.Int32(6379),
		},
		ReaderEndpoint: &ectypes.Endpoint{
			Address: aws.String(addr),
			Port:    aws.Int32(6380),
		},
		SubnetIds: []string{"subnet-a", "subnet-b"},
	}

	for _, opt := range opts {
		opt(cache)
	}
	return cache
}

func regionToShortRegion(region string) string {
	switch region {
	case "us-east-1":
		return "use1"
	case "us-east-2":
		return "use2"
	case "us-west-1":
		return "usw1"
	case "us-west-2":
		return "usw2"
	case "ca-central-1":
		return "cac1"
	case "eu-west-1":
		return "euw1"
	case "eu-west-2":
		return "euw2"
	case "eu-west-3":
		return "euw3"
	case "eu-central-1":
		return "euc1"
	case "eu-south-1":
		return "eus1"
	case "eu-north-1":
		return "eun1"
	case "ap-southeast-1":
		return "apse1"
	case "ap-southeast-2":
		return "apse2"
	case "ap-south-1":
		return "aps1"
	case "ap-northeast-1":
		return "apne1"
	case "ap-northeast-2":
		return "apne2"
	case "ap-east-1":
		return "ape1"
	case "sa-east-1":
		return "sae1"
	case "af-south-1":
		return "afs1"
	case "us-gov-west-1":
		return "usgw1"
	case "us-gov-east-1":
		return "usge1"
	case "cn-north-1":
		return "cnn1"
	case "cn-northwest-1":
		return "cnnw1"
	default:
		panic(fmt.Sprintf("unknown region %s", region))
	}
}
