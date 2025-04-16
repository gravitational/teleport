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
	memorydb "github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/gravitational/trace"
)

// MemoryDBClient mocks AWS MemoryDB API.
type MemoryDBClient struct {
	Unauth    bool
	Clusters  []memorydbtypes.Cluster
	Users     []memorydbtypes.User
	TagsByARN map[string][]memorydbtypes.Tag
}

func (m *MemoryDBClient) AddMockUser(user memorydbtypes.User, tagsMap map[string]string) {
	m.Users = append(m.Users, user)
	m.addTags(aws.ToString(user.ARN), tagsMap)
}

func (m *MemoryDBClient) addTags(arn string, tagsMap map[string]string) {
	if m.TagsByARN == nil {
		m.TagsByARN = make(map[string][]memorydbtypes.Tag)
	}

	var tags []memorydbtypes.Tag
	for key, value := range tagsMap {
		tags = append(tags, memorydbtypes.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	m.TagsByARN[arn] = tags
}

func (m *MemoryDBClient) DescribeSubnetGroups(context.Context, *memorydb.DescribeSubnetGroupsInput, ...func(*memorydb.Options)) (*memorydb.DescribeSubnetGroupsOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *MemoryDBClient) DescribeClusters(_ context.Context, input *memorydb.DescribeClustersInput, _ ...func(*memorydb.Options)) (*memorydb.DescribeClustersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if aws.ToString(input.ClusterName) == "" {
		return &memorydb.DescribeClustersOutput{
			Clusters: m.Clusters,
		}, nil
	}

	for _, cluster := range m.Clusters {
		if aws.ToString(input.ClusterName) == aws.ToString(cluster.Name) {
			return &memorydb.DescribeClustersOutput{
				Clusters: []memorydbtypes.Cluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("MemoryDB cluster %q not found", aws.ToString(input.ClusterName))
}

func (m *MemoryDBClient) ListTags(_ context.Context, input *memorydb.ListTagsInput, _ ...func(*memorydb.Options)) (*memorydb.ListTagsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return nil, trace.NotFound("no tags")
	}

	tags, ok := m.TagsByARN[aws.ToString(input.ResourceArn)]
	if !ok {
		return nil, trace.NotFound("no tags")
	}

	return &memorydb.ListTagsOutput{
		TagList: tags,
	}, nil
}

func (m *MemoryDBClient) DescribeUsers(_ context.Context, input *memorydb.DescribeUsersInput, _ ...func(*memorydb.Options)) (*memorydb.DescribeUsersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if aws.ToString(input.UserName) == "" {
		return &memorydb.DescribeUsersOutput{Users: m.Users}, nil
	}
	for _, u := range m.Users {
		if aws.ToString(u.Name) == aws.ToString(input.UserName) {
			return &memorydb.DescribeUsersOutput{Users: []memorydbtypes.User{u}}, nil
		}
	}
	return nil, trace.NotFound("MemoryDB UserName %q not found", aws.ToString(input.UserName))
}

func (m *MemoryDBClient) UpdateUser(_ context.Context, input *memorydb.UpdateUserInput, opts ...func(*memorydb.Options)) (*memorydb.UpdateUserOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, user := range m.Users {
		if aws.ToString(user.Name) == aws.ToString(input.UserName) {
			return &memorydb.UpdateUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("MemoryDB user %q not found", aws.ToString(input.UserName))
}

// MemoryDBCluster returns a sample memorydbtypes.Cluster.
func MemoryDBCluster(name, region string, opts ...func(*memorydbtypes.Cluster)) *memorydbtypes.Cluster {
	cluster := &memorydbtypes.Cluster{
		ARN:        aws.String(fmt.Sprintf("arn:aws:memorydb:%s:123456789012:cluster:%s", region, name)),
		Name:       aws.String(name),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ClusterEndpoint: &memorydbtypes.Endpoint{
			Address: aws.String(fmt.Sprintf("clustercfg.%s.xxxxxx.memorydb.%s.amazonaws.com", name, region)),
			Port:    6379,
		},
		Engine: aws.String("redis"),
	}

	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}
