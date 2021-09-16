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
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
)

type stsMock struct {
	stsiface.STSAPI
	arn string
}

func (m *stsMock) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.arn),
	}, nil
}

type rdsMock struct {
	rdsiface.RDSAPI
	dbInstances []*rds.DBInstance
	dbClusters  []*rds.DBCluster
}

func (m *rdsMock) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	if aws.StringValue(input.DBInstanceIdentifier) == "" {
		return &rds.DescribeDBInstancesOutput{
			DBInstances: m.dbInstances,
		}, nil
	}
	for _, instance := range m.dbInstances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{instance},
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *rdsMock) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	if aws.StringValue(input.DBClusterIdentifier) == "" {
		return &rds.DescribeDBClustersOutput{
			DBClusters: m.dbClusters,
		}, nil
	}
	for _, cluster := range m.dbClusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			return &rds.DescribeDBClustersOutput{
				DBClusters: []*rds.DBCluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

func (m *rdsMock) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	for i, instance := range m.dbInstances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.dbInstances[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBInstanceOutput{
				DBInstance: m.dbInstances[i],
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *rdsMock) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	for i, cluster := range m.dbClusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.dbClusters[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBClusterOutput{
				DBCluster: m.dbClusters[i],
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

type iamMock struct {
	iamiface.IAMAPI
	attachedRolePolicies map[string][]string
	attachedUserPolicies map[string][]string
}

func (m *iamMock) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	if m.attachedRolePolicies == nil {
		m.attachedRolePolicies = make(map[string][]string)
	}
	m.attachedRolePolicies[aws.StringValue(input.RoleName)] = append(
		m.attachedRolePolicies[aws.StringValue(input.RoleName)],
		aws.StringValue(input.PolicyName))
	return &iam.PutRolePolicyOutput{}, nil
}

func (m *iamMock) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	if m.attachedUserPolicies == nil {
		m.attachedUserPolicies = make(map[string][]string)
	}
	m.attachedUserPolicies[aws.StringValue(input.UserName)] = append(
		m.attachedUserPolicies[aws.StringValue(input.UserName)],
		aws.StringValue(input.PolicyName))
	return &iam.PutUserPolicyOutput{}, nil
}

func (m *iamMock) DeleteRolePolicyWithContext(ctx aws.Context, input *iam.DeleteRolePolicyInput, options ...request.Option) (*iam.DeleteRolePolicyOutput, error) {
	for i, policy := range m.attachedRolePolicies[aws.StringValue(input.RoleName)] {
		if policy == aws.StringValue(input.PolicyName) {
			m.attachedRolePolicies[aws.StringValue(input.RoleName)] = append(
				m.attachedRolePolicies[aws.StringValue(input.RoleName)][:i],
				m.attachedRolePolicies[aws.StringValue(input.RoleName)][i+1:]...)
		}
	}
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (m *iamMock) DeleteUserPolicyWithContext(ctx aws.Context, input *iam.DeleteUserPolicyInput, options ...request.Option) (*iam.DeleteUserPolicyOutput, error) {
	for i, policy := range m.attachedUserPolicies[aws.StringValue(input.UserName)] {
		if policy == aws.StringValue(input.PolicyName) {
			m.attachedUserPolicies[aws.StringValue(input.UserName)] = append(
				m.attachedUserPolicies[aws.StringValue(input.UserName)][:i],
				m.attachedUserPolicies[aws.StringValue(input.UserName)][i+1:]...)
		}
	}
	return &iam.DeleteUserPolicyOutput{}, nil
}
