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
