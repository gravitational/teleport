/*
Copyright 2023 Gravitational, Inc.

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

package mocks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/gravitational/trace"
)

// ElastiCache mocks AWS ElastiCache API.
type ElastiCacheMock struct {
	elasticacheiface.ElastiCacheAPI
	// Unauth set to true will make API calls return unauthorized errors.
	Unauth bool

	ReplicationGroups []*elasticache.ReplicationGroup
	Users             []*elasticache.User
	TagsByARN         map[string][]*elasticache.Tag
}

func (m *ElastiCacheMock) AddMockUser(user *elasticache.User, tagsMap map[string]string) {
	m.Users = append(m.Users, user)
	m.addTags(aws.StringValue(user.ARN), tagsMap)
}

func (m *ElastiCacheMock) addTags(arn string, tagsMap map[string]string) {
	if m.TagsByARN == nil {
		m.TagsByARN = make(map[string][]*elasticache.Tag)
	}

	var tags []*elasticache.Tag
	for key, value := range tagsMap {
		tags = append(tags, &elasticache.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	m.TagsByARN[arn] = tags
}

func (m *ElastiCacheMock) DescribeUsersWithContext(_ aws.Context, input *elasticache.DescribeUsersInput, opts ...request.Option) (*elasticache.DescribeUsersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if input.UserId == nil {
		return &elasticache.DescribeUsersOutput{Users: m.Users}, nil
	}
	for _, user := range m.Users {
		if aws.StringValue(user.UserId) == aws.StringValue(input.UserId) {
			return &elasticache.DescribeUsersOutput{Users: []*elasticache.User{user}}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache UserId %v not found", aws.StringValue(input.UserId))
}

func (m *ElastiCacheMock) DescribeReplicationGroupsWithContext(_ aws.Context, input *elasticache.DescribeReplicationGroupsInput, opts ...request.Option) (*elasticache.DescribeReplicationGroupsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, replicationGroup := range m.ReplicationGroups {
		if aws.StringValue(replicationGroup.ReplicationGroupId) == aws.StringValue(input.ReplicationGroupId) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []*elasticache.ReplicationGroup{replicationGroup},
			}, nil
		}
	}
	return nil, trace.NotFound("ElastiCache %v not found", aws.StringValue(input.ReplicationGroupId))
}

func (m *ElastiCacheMock) DescribeReplicationGroupsPagesWithContext(_ aws.Context, _ *elasticache.DescribeReplicationGroupsInput, fn func(*elasticache.DescribeReplicationGroupsOutput, bool) bool, _ ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	fn(&elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: m.ReplicationGroups,
	}, true)
	return nil
}

func (m *ElastiCacheMock) DescribeUsersPagesWithContext(_ aws.Context, _ *elasticache.DescribeUsersInput, fn func(*elasticache.DescribeUsersOutput, bool) bool, _ ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	fn(&elasticache.DescribeUsersOutput{
		Users: m.Users,
	}, true)
	return nil
}

func (m *ElastiCacheMock) DescribeCacheClustersPagesWithContext(aws.Context, *elasticache.DescribeCacheClustersInput, func(*elasticache.DescribeCacheClustersOutput, bool) bool, ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	return trace.NotImplemented("elasticache:DescribeCacheClustersPagesWithContext is not implemented")
}

func (m *ElastiCacheMock) DescribeCacheSubnetGroupsPagesWithContext(aws.Context, *elasticache.DescribeCacheSubnetGroupsInput, func(*elasticache.DescribeCacheSubnetGroupsOutput, bool) bool, ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	return trace.NotImplemented("elasticache:DescribeCacheSubnetGroupsPagesWithContext is not implemented")
}

func (m *ElastiCacheMock) ListTagsForResourceWithContext(_ aws.Context, input *elasticache.ListTagsForResourceInput, _ ...request.Option) (*elasticache.TagListMessage, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return nil, trace.NotFound("no tags")
	}

	tags, ok := m.TagsByARN[aws.StringValue(input.ResourceName)]
	if !ok {
		return nil, trace.NotFound("no tags")
	}

	return &elasticache.TagListMessage{
		TagList: tags,
	}, nil
}

func (m *ElastiCacheMock) ModifyUserWithContext(_ aws.Context, input *elasticache.ModifyUserInput, opts ...request.Option) (*elasticache.ModifyUserOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, user := range m.Users {
		if aws.StringValue(user.UserId) == aws.StringValue(input.UserId) {
			return &elasticache.ModifyUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("user %s not found", aws.StringValue(input.UserId))
}

// ElastiCacheCluster returns a sample elasticache.ReplicationGroup.
func ElastiCacheCluster(name, region string, opts ...func(*elasticache.ReplicationGroup)) *elasticache.ReplicationGroup {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String(fmt.Sprintf("arn:aws:elasticache:%s:123456789012:replicationgroup:%s", region, name)),
		ReplicationGroupId:       aws.String(name),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),

		// Default has one primary endpoint in the only node group.
		NodeGroups: []*elasticache.NodeGroup{{
			PrimaryEndpoint: &elasticache.Endpoint{
				Address: aws.String(fmt.Sprintf("master.%v-cluster.xxxxxx.use1.cache.amazonaws.com", name)),
				Port:    aws.Int64(6379),
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
func WithElastiCacheReaderEndpoint(cluster *elasticache.ReplicationGroup) {
	cluster.NodeGroups = append(cluster.NodeGroups, &elasticache.NodeGroup{
		ReaderEndpoint: &elasticache.Endpoint{
			Address: aws.String(fmt.Sprintf("replica.%v-cluster.xxxxxx.use1.cache.amazonaws.com", aws.StringValue(cluster.ReplicationGroupId))),
			Port:    aws.Int64(6379),
		},
	})
}

// WithElastiCacheConfigurationEndpoint in an option function for
// MakeElastiCacheCluster to set a configuration endpoint.
func WithElastiCacheConfigurationEndpoint(cluster *elasticache.ReplicationGroup) {
	cluster.ClusterEnabled = aws.Bool(true)
	cluster.ConfigurationEndpoint = &elasticache.Endpoint{
		Address: aws.String(fmt.Sprintf("clustercfg.%v-shards.xxxxxx.use1.cache.amazonaws.com", aws.StringValue(cluster.ReplicationGroupId))),
		Port:    aws.Int64(6379),
	}
}
