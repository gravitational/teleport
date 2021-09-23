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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
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
	attachedRolePolicies map[string][]string
	attachedUserPolicies map[string][]string
}

func (m *IAMMock) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	if m.attachedRolePolicies == nil {
		m.attachedRolePolicies = make(map[string][]string)
	}
	m.attachedRolePolicies[aws.StringValue(input.RoleName)] = append(
		m.attachedRolePolicies[aws.StringValue(input.RoleName)],
		aws.StringValue(input.PolicyName))
	return &iam.PutRolePolicyOutput{}, nil
}

func (m *IAMMock) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	if m.attachedUserPolicies == nil {
		m.attachedUserPolicies = make(map[string][]string)
	}
	m.attachedUserPolicies[aws.StringValue(input.UserName)] = append(
		m.attachedUserPolicies[aws.StringValue(input.UserName)],
		aws.StringValue(input.PolicyName))
	return &iam.PutUserPolicyOutput{}, nil
}

func (m *IAMMock) DeleteRolePolicyWithContext(ctx aws.Context, input *iam.DeleteRolePolicyInput, options ...request.Option) (*iam.DeleteRolePolicyOutput, error) {
	for i, policy := range m.attachedRolePolicies[aws.StringValue(input.RoleName)] {
		if policy == aws.StringValue(input.PolicyName) {
			m.attachedRolePolicies[aws.StringValue(input.RoleName)] = append(
				m.attachedRolePolicies[aws.StringValue(input.RoleName)][:i],
				m.attachedRolePolicies[aws.StringValue(input.RoleName)][i+1:]...)
		}
	}
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (m *IAMMock) DeleteUserPolicyWithContext(ctx aws.Context, input *iam.DeleteUserPolicyInput, options ...request.Option) (*iam.DeleteUserPolicyOutput, error) {
	for i, policy := range m.attachedUserPolicies[aws.StringValue(input.UserName)] {
		if policy == aws.StringValue(input.PolicyName) {
			m.attachedUserPolicies[aws.StringValue(input.UserName)] = append(
				m.attachedUserPolicies[aws.StringValue(input.UserName)][:i],
				m.attachedUserPolicies[aws.StringValue(input.UserName)][i+1:]...)
		}
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

// rdsMockUnath is a mock RDS client that returns access denied to each call.
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

func (m *IAMMockUnauth) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) DeleteRolePolicyWithContext(ctx aws.Context, input *iam.DeleteRolePolicyInput, options ...request.Option) (*iam.DeleteRolePolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMMockUnauth) DeleteUserPolicyWithContext(ctx aws.Context, input *iam.DeleteUserPolicyInput, options ...request.Option) (*iam.DeleteUserPolicyOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}
