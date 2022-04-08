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
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	// attachedRolePolicies maps roleName -> policyName -> policyDocument
	attachedRolePolicies map[string]map[string]string
	// attachedUserPolicies maps userName -> policyName -> policyDocument
	attachedUserPolicies map[string]map[string]string
}

func (m *IAMMock) GetRolePolicyWithContext(ctx aws.Context, input *iam.GetRolePolicyInput, options ...request.Option) (*iam.GetRolePolicyOutput, error) {
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
	if _, ok := m.attachedRolePolicies[*input.RoleName]; ok {
		delete(m.attachedRolePolicies[*input.RoleName], *input.PolicyName)
	}
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (m *IAMMock) GetUserPolicyWithContext(ctx aws.Context, input *iam.GetUserPolicyInput, options ...request.Option) (*iam.GetUserPolicyOutput, error) {
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

// GCPSQLAdminClientMock implements the common.GCPSQLAdminClient interface for tests.
type GCPSQLAdminClientMock struct {
	// DatabaseInstance is returned from GetDatabaseInstance.
	DatabaseInstance *sqladmin.DatabaseInstance
	// EphemeralCert is returned from GenerateEphemeralCert.
	EphemeralCert *tls.Certificate
}

func (g *GCPSQLAdminClientMock) UpdateUser(ctx context.Context, sessionCtx *common.Session, user *sqladmin.User) error {
	return nil
}

func (g *GCPSQLAdminClientMock) GetDatabaseInstance(ctx context.Context, sessionCtx *common.Session) (*sqladmin.DatabaseInstance, error) {
	return g.DatabaseInstance, nil
}

func (g *GCPSQLAdminClientMock) GenerateEphemeralCert(ctx context.Context, sessionCtx *common.Session) (*tls.Certificate, error) {
	return g.EphemeralCert, nil
}
