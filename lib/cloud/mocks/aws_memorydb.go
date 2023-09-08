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
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/gravitational/trace"
)

// MemoryDBMock mocks AWS MemoryDB API.
type MemoryDBMock struct {
	memorydbiface.MemoryDBAPI

	Unauth    bool
	Clusters  []*memorydb.Cluster
	Users     []*memorydb.User
	TagsByARN map[string][]*memorydb.Tag
}

func (m *MemoryDBMock) AddMockUser(user *memorydb.User, tagsMap map[string]string) {
	m.Users = append(m.Users, user)
	m.addTags(aws.StringValue(user.ARN), tagsMap)
}

func (m *MemoryDBMock) addTags(arn string, tagsMap map[string]string) {
	if m.TagsByARN == nil {
		m.TagsByARN = make(map[string][]*memorydb.Tag)
	}

	var tags []*memorydb.Tag
	for key, value := range tagsMap {
		tags = append(tags, &memorydb.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	m.TagsByARN[arn] = tags
}

func (m *MemoryDBMock) DescribeSubnetGroupsWithContext(aws.Context, *memorydb.DescribeSubnetGroupsInput, ...request.Option) (*memorydb.DescribeSubnetGroupsOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *MemoryDBMock) DescribeClustersWithContext(_ aws.Context, input *memorydb.DescribeClustersInput, _ ...request.Option) (*memorydb.DescribeClustersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if aws.StringValue(input.ClusterName) == "" {
		return &memorydb.DescribeClustersOutput{
			Clusters: m.Clusters,
		}, nil
	}

	for _, cluster := range m.Clusters {
		if aws.StringValue(input.ClusterName) == aws.StringValue(cluster.Name) {
			return &memorydb.DescribeClustersOutput{
				Clusters: []*memorydb.Cluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.ClusterName))
}

func (m *MemoryDBMock) ListTagsWithContext(_ aws.Context, input *memorydb.ListTagsInput, _ ...request.Option) (*memorydb.ListTagsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return nil, trace.NotFound("no tags")
	}

	tags, ok := m.TagsByARN[aws.StringValue(input.ResourceArn)]
	if !ok {
		return nil, trace.NotFound("no tags")
	}

	return &memorydb.ListTagsOutput{
		TagList: tags,
	}, nil
}

func (m *MemoryDBMock) DescribeUsersWithContext(aws.Context, *memorydb.DescribeUsersInput, ...request.Option) (*memorydb.DescribeUsersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return &memorydb.DescribeUsersOutput{
		Users: m.Users,
	}, nil
}

func (m *MemoryDBMock) UpdateUserWithContext(_ aws.Context, input *memorydb.UpdateUserInput, opts ...request.Option) (*memorydb.UpdateUserOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, user := range m.Users {
		if aws.StringValue(user.Name) == aws.StringValue(input.UserName) {
			return &memorydb.UpdateUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("user %s not found", aws.StringValue(input.UserName))
}

// MemoryDBCluster returns a sample memorydb.Cluster.
func MemoryDBCluster(name, region string, opts ...func(*memorydb.Cluster)) *memorydb.Cluster {
	cluster := &memorydb.Cluster{
		ARN:        aws.String(fmt.Sprintf("arn:aws:memorydb:%s:123456789012:cluster:%s", region, name)),
		Name:       aws.String(name),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ClusterEndpoint: &memorydb.Endpoint{
			Address: aws.String(fmt.Sprintf("clustercfg.%s.xxxxxx.memorydb.%s.amazonaws.com", name, region)),
			Port:    aws.Int64(6379),
		},
	}

	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}
