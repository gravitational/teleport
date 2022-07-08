/*
Copyright 2021 Gravitational, Inc.

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

package cloud

import (
	"context"
	"crypto/tls"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/teleport/lib/cloud/clients"
	"github.com/gravitational/trace"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// STSMock mocks AWS STS API.
type STSMock struct {
	stsiface.STSAPI
	ARN string
}

func (m *STSMock) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.ARN),
	}, nil
}

// RDSMock mocks AWS RDS API.
type RDSMock struct {
	rdsiface.RDSAPI
	DBInstances []*rds.DBInstance
	DBClusters  []*rds.DBCluster
}

func (m *RDSMock) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	if aws.StringValue(input.DBInstanceIdentifier) == "" {
		return &rds.DescribeDBInstancesOutput{
			DBInstances: m.DBInstances,
		}, nil
	}
	for _, instance := range m.DBInstances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{instance},
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *RDSMock) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBInstancesOutput{
		DBInstances: m.DBInstances,
	}, true)
	return nil
}

func (m *RDSMock) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	if aws.StringValue(input.DBClusterIdentifier) == "" {
		return &rds.DescribeDBClustersOutput{
			DBClusters: m.DBClusters,
		}, nil
	}
	for _, cluster := range m.DBClusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			return &rds.DescribeDBClustersOutput{
				DBClusters: []*rds.DBCluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

func (m *RDSMock) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBClustersOutput{
		DBClusters: m.DBClusters,
	}, true)
	return nil
}

func (m *RDSMock) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	for i, instance := range m.DBInstances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.DBInstances[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBInstanceOutput{
				DBInstance: m.DBInstances[i],
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *RDSMock) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	for i, cluster := range m.DBClusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.DBClusters[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBClusterOutput{
				DBCluster: m.DBClusters[i],
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

// IAMMock mocks AWS IAM API.
type IAMMock struct {
	iamiface.IAMAPI
	mu sync.RWMutex
	// attachedRolePolicies maps roleName -> policyName -> policyDocument
	attachedRolePolicies map[string]map[string]string
	// attachedUserPolicies maps userName -> policyName -> policyDocument
	attachedUserPolicies map[string]map[string]string
}

func (m *IAMMock) GetRolePolicyWithContext(ctx aws.Context, input *iam.GetRolePolicyInput, options ...request.Option) (*iam.GetRolePolicyOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy, ok := m.attachedRolePolicies[*input.RoleName]
	if !ok {
		return nil, trace.NotFound("policy not found")
	}
	policyDocument, ok := policy[*input.PolicyName]
	if !ok {
		return nil, trace.NotFound("policy not found")
	}
	return &iam.GetRolePolicyOutput{
		PolicyDocument: &policyDocument,
		PolicyName:     input.PolicyName,
		RoleName:       input.RoleName,
	}, nil
}

func (m *IAMMock) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.attachedRolePolicies == nil {
		m.attachedRolePolicies = make(map[string]map[string]string)
	}
	if m.attachedRolePolicies[*input.RoleName] == nil {
		m.attachedRolePolicies[*input.RoleName] = make(map[string]string)
	}
	m.attachedRolePolicies[*input.RoleName][*input.PolicyName] = *input.PolicyDocument
	return &iam.PutRolePolicyOutput{}, nil
}

func (m *IAMMock) DeleteRolePolicyWithContext(ctx aws.Context, input *iam.DeleteRolePolicyInput, options ...request.Option) (*iam.DeleteRolePolicyOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attachedRolePolicies[*input.RoleName]; ok {
		delete(m.attachedRolePolicies[*input.RoleName], *input.PolicyName)
	}
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (m *IAMMock) GetUserPolicyWithContext(ctx aws.Context, input *iam.GetUserPolicyInput, options ...request.Option) (*iam.GetUserPolicyOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy, ok := m.attachedUserPolicies[*input.UserName]
	if !ok {
		return nil, trace.NotFound("policy not found")
	}
	policyDocument, ok := policy[*input.PolicyName]
	if !ok {
		return nil, trace.NotFound("policy not found")
	}
	return &iam.GetUserPolicyOutput{
		PolicyDocument: &policyDocument,
		PolicyName:     input.PolicyName,
		UserName:       input.UserName,
	}, nil
}

func (m *IAMMock) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.attachedUserPolicies == nil {
		m.attachedUserPolicies = make(map[string]map[string]string)
	}
	if m.attachedUserPolicies[*input.UserName] == nil {
		m.attachedUserPolicies[*input.UserName] = make(map[string]string)
	}
	m.attachedUserPolicies[*input.UserName][*input.PolicyName] = *input.PolicyDocument
	return &iam.PutUserPolicyOutput{}, nil
}

func (m *IAMMock) DeleteUserPolicyWithContext(ctx aws.Context, input *iam.DeleteUserPolicyInput, options ...request.Option) (*iam.DeleteUserPolicyOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attachedUserPolicies[*input.UserName]; ok {
		delete(m.attachedUserPolicies[*input.UserName], *input.PolicyName)
	}
	return &iam.DeleteUserPolicyOutput{}, nil
}

// RedshiftMock mocks AWS Redshift API.
type RedshiftMock struct {
	redshiftiface.RedshiftAPI
	Clusters []*redshift.Cluster
}

func (m *RedshiftMock) DescribeClustersWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, options ...request.Option) (*redshift.DescribeClustersOutput, error) {
	if aws.StringValue(input.ClusterIdentifier) == "" {
		return &redshift.DescribeClustersOutput{
			Clusters: m.Clusters,
		}, nil
	}
	for _, cluster := range m.Clusters {
		if aws.StringValue(cluster.ClusterIdentifier) == aws.StringValue(input.ClusterIdentifier) {
			return &redshift.DescribeClustersOutput{
				Clusters: []*redshift.Cluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.ClusterIdentifier))
}

func (m *RedshiftMock) DescribeClustersPagesWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, fn func(*redshift.DescribeClustersOutput, bool) bool, options ...request.Option) error {
	fn(&redshift.DescribeClustersOutput{
		Clusters: m.Clusters,
	}, true)
	return nil
}

// RDSMockUnauth is a mock RDS client that returns access denied to each call.
type RDSMockUnauth struct {
	rdsiface.RDSAPI
}

func (m *RDSMockUnauth) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

// RDSMockByDBType is a mock RDS client that mocks API calls by DB type
type RDSMockByDBType struct {
	rdsiface.RDSAPI
	DBInstances rdsiface.RDSAPI
	DBClusters  rdsiface.RDSAPI
}

func (m *RDSMockByDBType) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	return m.DBInstances.DescribeDBInstancesWithContext(ctx, input, options...)
}
func (m *RDSMockByDBType) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	return m.DBInstances.ModifyDBInstanceWithContext(ctx, input, options...)
}
func (m *RDSMockByDBType) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	return m.DBInstances.DescribeDBInstancesPagesWithContext(ctx, input, fn, options...)
}

func (m *RDSMockByDBType) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	return m.DBClusters.DescribeDBClustersWithContext(ctx, input, options...)
}
func (m *RDSMockByDBType) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	return m.DBClusters.ModifyDBClusterWithContext(ctx, input, options...)
}
func (m *RDSMockByDBType) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	return m.DBClusters.DescribeDBClustersPagesWithContext(aws, input, fn, options...)
}

// RedshiftMockUnauth is a mock Redshift client that returns access denied to each call.
type RedshiftMockUnauth struct {
	redshiftiface.RedshiftAPI
}

func (m *RedshiftMockUnauth) DescribeClustersWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, options ...request.Option) (*redshift.DescribeClustersOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

// IAMMockUnauth is a mock IAM client that returns access denied to each call.
type IAMMockUnauth struct {
	iamiface.IAMAPI
}

func (m *IAMMockUnauth) GetRolePolicyWithContext(ctx aws.Context, input *iam.GetRolePolicyInput, options ...request.Option) (*iam.GetRolePolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) GetUserPolicyWithContext(ctx aws.Context, input *iam.GetUserPolicyInput, options ...request.Option) (*iam.GetUserPolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

// GCPSQLAdminClientMock implements the clients.GCPSQLAdminClient interface for tests.
type GCPSQLAdminClientMock struct {
	// DatabaseInstance is returned from GetDatabaseInstance.
	DatabaseInstance *sqladmin.DatabaseInstance
	// EphemeralCert is returned from GenerateEphemeralCert.
	EphemeralCert *tls.Certificate
}

func (g *GCPSQLAdminClientMock) UpdateUser(ctx context.Context, sessionCtx *clients.Session, user *sqladmin.User) error {
	return nil
}

func (g *GCPSQLAdminClientMock) GetDatabaseInstance(ctx context.Context, sessionCtx *clients.Session) (*sqladmin.DatabaseInstance, error) {
	return g.DatabaseInstance, nil
}

func (g *GCPSQLAdminClientMock) GenerateEphemeralCert(ctx context.Context, sessionCtx *clients.Session) (*tls.Certificate, error) {
	return g.EphemeralCert, nil
}

// ElastiCache mocks AWS ElastiCache API.
type ElastiCacheMock struct {
	elasticacheiface.ElastiCacheAPI

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

func (m *ElastiCacheMock) DescribeReplicationGroupsWithContext(_ aws.Context, input *elasticache.DescribeReplicationGroupsInput, opts ...request.Option) (*elasticache.DescribeReplicationGroupsOutput, error) {
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
	fn(&elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: m.ReplicationGroups,
	}, true)
	return nil
}
func (m *ElastiCacheMock) DescribeUsersPagesWithContext(_ aws.Context, _ *elasticache.DescribeUsersInput, fn func(*elasticache.DescribeUsersOutput, bool) bool, _ ...request.Option) error {
	fn(&elasticache.DescribeUsersOutput{
		Users: m.Users,
	}, true)
	return nil
}

func (m *ElastiCacheMock) DescribeCacheClustersPagesWithContext(aws.Context, *elasticache.DescribeCacheClustersInput, func(*elasticache.DescribeCacheClustersOutput, bool) bool, ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}
func (m *ElastiCacheMock) DescribeCacheSubnetGroupsPagesWithContext(aws.Context, *elasticache.DescribeCacheSubnetGroupsInput, func(*elasticache.DescribeCacheSubnetGroupsOutput, bool) bool, ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}
func (m *ElastiCacheMock) ListTagsForResourceWithContext(_ aws.Context, input *elasticache.ListTagsForResourceInput, _ ...request.Option) (*elasticache.TagListMessage, error) {
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
	for _, user := range m.Users {
		if aws.StringValue(user.UserId) == aws.StringValue(input.UserId) {
			return &elasticache.ModifyUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("user %s not found", aws.StringValue(input.UserId))
}

// MemoryDBMock mocks AWS MemoryDB API.
type MemoryDBMock struct {
	memorydbiface.MemoryDBAPI

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
	return &memorydb.DescribeUsersOutput{
		Users: m.Users,
	}, nil
}
func (m *MemoryDBMock) UpdateUserWithContext(_ aws.Context, input *memorydb.UpdateUserInput, opts ...request.Option) (*memorydb.UpdateUserOutput, error) {
	for _, user := range m.Users {
		if aws.StringValue(user.Name) == aws.StringValue(input.UserName) {
			return &memorydb.UpdateUserOutput{}, nil
		}
	}
	return nil, trace.NotFound("user %s not found", aws.StringValue(input.UserName))
}

type EC2Mock struct {
	ec2iface.EC2API
	Instances []*ec2.Instance
}

func (m *EC2Mock) DescribeInstancesPagesWithContext(
	ctx context.Context, input *ec2.DescribeInstancesInput,
	f func(dio *ec2.DescribeInstancesOutput, b bool) bool, opts ...request.Option) error {

	var instances []*ec2.Instance

	for _, inst := range m.Instances {
		tagMatch := false
		stateMatch := false
		for _, tag := range inst.Tags {
			for _, filter := range input.Filters {
				if strings.HasPrefix(aws.StringValue(filter.Name), "tag:") && !tagMatch {
					tagMatch =
						aws.StringValue(filter.Name)[4:] == aws.StringValue(tag.Key) &&
							aws.StringValue(tag.Value) == aws.StringValueSlice(filter.Values)[0]
				}
				if aws.StringValue(filter.Name) == "instance-state-name" && !stateMatch {
					stateMatch =
						aws.StringValue(inst.State.Name) == ec2.InstanceStateNameRunning
				}

				if stateMatch && tagMatch {
					instances = append(instances, inst)
				}
			}
		}

	}

	filtered := &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: instances}},
	}
	f(filtered, true)
	return nil
}
