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

package mocks

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/opensearchservice/opensearchserviceiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/exp/slices"
)

// STSMock mocks AWS STS API.
type STSMock struct {
	stsiface.STSAPI
	ARN                    string
	URL                    *url.URL
	assumedRoleARNs        []string
	assumedRoleExternalIDs []string
	mu                     sync.Mutex
}

func (m *STSMock) GetAssumedRoleARNs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleARNs
}

func (m *STSMock) GetAssumedRoleExternalIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleExternalIDs
}

func (m *STSMock) ResetAssumeRoleHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assumedRoleARNs = nil
	m.assumedRoleExternalIDs = nil
}

func (m *STSMock) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.ARN),
	}, nil
}

func (m *STSMock) AssumeRole(in *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return m.AssumeRoleWithContext(context.Background(), in)
}

func (m *STSMock) AssumeRoleWithContext(ctx aws.Context, in *sts.AssumeRoleInput, _ ...request.Option) (*sts.AssumeRoleOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.assumedRoleARNs, aws.StringValue(in.RoleArn)) {
		m.assumedRoleARNs = append(m.assumedRoleARNs, aws.StringValue(in.RoleArn))
		m.assumedRoleExternalIDs = append(m.assumedRoleExternalIDs, aws.StringValue(in.ExternalId))
	}
	expiry := time.Now().Add(60 * time.Minute)
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     in.RoleArn,
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
		},
	}, nil
}

func (m *STSMock) GetCallerIdentityRequest(req *sts.GetCallerIdentityInput) (*request.Request, *sts.GetCallerIdentityOutput) {
	return &request.Request{
		HTTPRequest: &http.Request{
			Header: http.Header{},
			URL:    m.URL,
		},
		Operation: &request.Operation{
			Name:       "GetCallerIdentity",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		Handlers: request.Handlers{},
	}, nil
}

// RDSMock mocks AWS RDS API.
type RDSMock struct {
	rdsiface.RDSAPI
	DBInstances       []*rds.DBInstance
	DBClusters        []*rds.DBCluster
	DBProxies         []*rds.DBProxy
	DBProxyEndpoints  []*rds.DBProxyEndpoint
	DBEngineVersions  []*rds.DBEngineVersion
	DBProxyTargetPort int64
}

func (m *RDSMock) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	instances, err := applyInstanceFilters(m.DBInstances, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.StringValue(input.DBInstanceIdentifier) == "" {
		return &rds.DescribeDBInstancesOutput{
			DBInstances: instances,
		}, nil
	}
	for _, instance := range instances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{instance},
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *RDSMock) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return trace.Wrap(err)
	}
	instances, err := applyInstanceFilters(m.DBInstances, input.Filters)
	if err != nil {
		return trace.Wrap(err)
	}
	fn(&rds.DescribeDBInstancesOutput{
		DBInstances: instances,
	}, true)
	return nil
}

func (m *RDSMock) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := applyClusterFilters(m.DBClusters, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.StringValue(input.DBClusterIdentifier) == "" {
		return &rds.DescribeDBClustersOutput{
			DBClusters: clusters,
		}, nil
	}
	for _, cluster := range clusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			return &rds.DescribeDBClustersOutput{
				DBClusters: []*rds.DBCluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

func (m *RDSMock) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return trace.Wrap(err)
	}
	clusters, err := applyClusterFilters(m.DBClusters, input.Filters)
	if err != nil {
		return trace.Wrap(err)
	}
	fn(&rds.DescribeDBClustersOutput{
		DBClusters: clusters,
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

func (m *RDSMock) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	if aws.StringValue(input.DBProxyName) == "" {
		return &rds.DescribeDBProxiesOutput{
			DBProxies: m.DBProxies,
		}, nil
	}
	for _, dbProxy := range m.DBProxies {
		if aws.StringValue(dbProxy.DBProxyName) == aws.StringValue(input.DBProxyName) {
			return &rds.DescribeDBProxiesOutput{
				DBProxies: []*rds.DBProxy{dbProxy},
			}, nil
		}
	}
	return nil, trace.NotFound("proxy %v not found", aws.StringValue(input.DBProxyName))
}

func (m *RDSMock) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	inputProxyName := aws.StringValue(input.DBProxyName)
	inputProxyEndpointName := aws.StringValue(input.DBProxyEndpointName)

	if inputProxyName == "" && inputProxyEndpointName == "" {
		return &rds.DescribeDBProxyEndpointsOutput{
			DBProxyEndpoints: m.DBProxyEndpoints,
		}, nil
	}

	var endpoints []*rds.DBProxyEndpoint
	for _, dbProxyEndpoiont := range m.DBProxyEndpoints {
		if inputProxyEndpointName != "" &&
			inputProxyEndpointName != aws.StringValue(dbProxyEndpoiont.DBProxyEndpointName) {
			continue
		}

		if inputProxyName != "" &&
			inputProxyName != aws.StringValue(dbProxyEndpoiont.DBProxyName) {
			continue
		}

		endpoints = append(endpoints, dbProxyEndpoiont)
	}
	if len(endpoints) == 0 {
		return nil, trace.NotFound("proxy endpoint %v not found", aws.StringValue(input.DBProxyEndpointName))
	}
	return &rds.DescribeDBProxyEndpointsOutput{DBProxyEndpoints: endpoints}, nil
}

func (m *RDSMock) DescribeDBProxyTargetsWithContext(ctx aws.Context, input *rds.DescribeDBProxyTargetsInput, options ...request.Option) (*rds.DescribeDBProxyTargetsOutput, error) {
	// only mocking to return a port here
	return &rds.DescribeDBProxyTargetsOutput{
		Targets: []*rds.DBProxyTarget{{
			Port: aws.Int64(m.DBProxyTargetPort),
		}},
	}, nil
}

func (m *RDSMock) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBProxiesOutput{
		DBProxies: m.DBProxies,
	}, true)
	return nil
}

func (m *RDSMock) DescribeDBProxyEndpointsPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, fn func(*rds.DescribeDBProxyEndpointsOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBProxyEndpointsOutput{
		DBProxyEndpoints: m.DBProxyEndpoints,
	}, true)
	return nil
}

func (m *RDSMock) ListTagsForResourceWithContext(ctx aws.Context, input *rds.ListTagsForResourceInput, options ...request.Option) (*rds.ListTagsForResourceOutput, error) {
	return &rds.ListTagsForResourceOutput{}, nil
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
		return nil, trace.NotFound("role policy %v not found", *input.RoleName)
	}
	policyDocument, ok := policy[*input.PolicyName]
	if !ok {
		return nil, trace.NotFound("role %v policy name %v not found", *input.RoleName, *input.PolicyName)
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
		return nil, trace.NotFound("user policy %v not found", *input.UserName)
	}
	policyDocument, ok := policy[*input.PolicyName]
	if !ok {
		return nil, trace.NotFound("user %v policy name %v not found", *input.UserName, *input.PolicyName)
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
	Clusters                    []*redshift.Cluster
	GetClusterCredentialsOutput *redshift.GetClusterCredentialsOutput
}

func (m *RedshiftMock) GetClusterCredentialsWithContext(aws.Context, *redshift.GetClusterCredentialsInput, ...request.Option) (*redshift.GetClusterCredentialsOutput, error) {
	if m.GetClusterCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsOutput, nil
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

func (m *RDSMockUnauth) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

// RDSMockByDBType is a mock RDS client that mocks API calls by DB type
type RDSMockByDBType struct {
	rdsiface.RDSAPI
	DBInstances rdsiface.RDSAPI
	DBClusters  rdsiface.RDSAPI
	DBProxies   rdsiface.RDSAPI
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

func (m *RDSMockByDBType) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	return m.DBProxies.DescribeDBProxiesWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	return m.DBProxies.DescribeDBProxyEndpointsWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	return m.DBProxies.DescribeDBProxiesPagesWithContext(ctx, input, fn, options...)
}

// RedshiftMockUnauth is a mock Redshift client that returns access denied to each call.
type RedshiftMockUnauth struct {
	redshiftiface.RedshiftAPI
}

func (m *RedshiftMockUnauth) DescribeClustersWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, options ...request.Option) (*redshift.DescribeClustersOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

// IAMErrorMock is a mock IAM client that returns the provided Error to all
// APIs. If Error is not provided, all APIs returns trace.AccessDenied by
// default.
type IAMErrorMock struct {
	iamiface.IAMAPI
	Error error
}

func (m *IAMErrorMock) GetRolePolicyWithContext(ctx aws.Context, input *iam.GetRolePolicyInput, options ...request.Option) (*iam.GetRolePolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMErrorMock) PutRolePolicyWithContext(ctx aws.Context, input *iam.PutRolePolicyInput, options ...request.Option) (*iam.PutRolePolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMErrorMock) GetUserPolicyWithContext(ctx aws.Context, input *iam.GetUserPolicyInput, options ...request.Option) (*iam.GetUserPolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return nil, trace.AccessDenied("unauthorized")
}

func (m *IAMErrorMock) PutUserPolicyWithContext(ctx aws.Context, input *iam.PutUserPolicyInput, options ...request.Option) (*iam.PutUserPolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return nil, trace.AccessDenied("unauthorized")
}

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

type OpenSearchMock struct {
	opensearchserviceiface.OpenSearchServiceAPI

	Domains   []*opensearchservice.DomainStatus
	TagsByARN map[string][]*opensearchservice.Tag
}

func (o *OpenSearchMock) ListDomainNamesWithContext(aws.Context, *opensearchservice.ListDomainNamesInput, ...request.Option) (*opensearchservice.ListDomainNamesOutput, error) {
	out := &opensearchservice.ListDomainNamesOutput{}
	for _, domain := range o.Domains {
		out.DomainNames = append(out.DomainNames, &opensearchservice.DomainInfo{
			DomainName: domain.DomainName,
			EngineType: aws.String("OpenSearch"),
		})
	}

	return out, nil
}

func (o *OpenSearchMock) DescribeDomainsWithContext(aws.Context, *opensearchservice.DescribeDomainsInput, ...request.Option) (*opensearchservice.DescribeDomainsOutput, error) {
	out := &opensearchservice.DescribeDomainsOutput{DomainStatusList: o.Domains}
	return out, nil
}

func (o *OpenSearchMock) ListTagsWithContext(_ aws.Context, request *opensearchservice.ListTagsInput, _ ...request.Option) (*opensearchservice.ListTagsOutput, error) {
	tags, found := o.TagsByARN[aws.StringValue(request.ARN)]
	if !found {
		return nil, trace.NotFound("tags not found")
	}
	return &opensearchservice.ListTagsOutput{TagList: tags}, nil
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

// checkEngineFilters checks RDS filters to detect unrecognized engine filters.
func checkEngineFilters(filters []*rds.Filter, engineVersions []*rds.DBEngineVersion) error {
	if len(filters) == 0 {
		return nil
	}
	recognizedEngines := make(map[string]struct{})
	for _, e := range engineVersions {
		recognizedEngines[aws.StringValue(e.Engine)] = struct{}{}
	}
	for _, f := range filters {
		if aws.StringValue(f.Name) != "engine" {
			continue
		}
		for _, v := range f.Values {
			if _, ok := recognizedEngines[aws.StringValue(v)]; !ok {
				return trace.Errorf("unrecognized engine name %q", aws.StringValue(v))
			}
		}
	}
	return nil
}

// applyInstanceFilters filters RDS DBInstances using the provided RDS filters.
func applyInstanceFilters(in []*rds.DBInstance, filters []*rds.Filter) ([]*rds.DBInstance, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []*rds.DBInstance
	efs := engineFilterSet(filters)
	for _, instance := range in {
		if instanceEngineMatches(instance, efs) {
			out = append(out, instance)
		}
	}
	return out, nil
}

// applyClusterFilters filters RDS DBClusters using the provided RDS filters.
func applyClusterFilters(in []*rds.DBCluster, filters []*rds.Filter) ([]*rds.DBCluster, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []*rds.DBCluster
	efs := engineFilterSet(filters)
	for _, cluster := range in {
		if clusterEngineMatches(cluster, efs) {
			out = append(out, cluster)
		}
	}
	return out, nil
}

// engineFilterSet builds a string set of engine names from a list of RDS filters.
func engineFilterSet(filters []*rds.Filter) map[string]struct{} {
	out := make(map[string]struct{})
	for _, f := range filters {
		if aws.StringValue(f.Name) != "engine" {
			continue
		}
		for _, v := range f.Values {
			out[aws.StringValue(v)] = struct{}{}
		}
	}
	return out
}

// instanceEngineMatches returns whether an RDS DBInstance engine matches any engine name in a filter set.
func instanceEngineMatches(instance *rds.DBInstance, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.StringValue(instance.Engine)]
	return ok
}

// clusterEngineMatches returns whether an RDS DBCluster engine matches any engine name in a filter set.
func clusterEngineMatches(cluster *rds.DBCluster, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.StringValue(cluster.Engine)]
	return ok
}

// RedshiftGetClusterCredentialsOutput return a sample redshift.GetClusterCredentialsOutput.
func RedshiftGetClusterCredentialsOutput(user, password string, clock clockwork.Clock) *redshift.GetClusterCredentialsOutput {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &redshift.GetClusterCredentialsOutput{
		DbUser:     aws.String(user),
		DbPassword: aws.String(password),
		Expiration: aws.Time(clock.Now().Add(15 * time.Minute)),
	}
}

// EKSMock is a mock EKS client.
type EKSMock struct {
	eksiface.EKSAPI
	Clusters []*eks.Cluster
	Notify   chan struct{}
}

func (e *EKSMock) DescribeClusterWithContext(_ aws.Context, req *eks.DescribeClusterInput, _ ...request.Option) (*eks.DescribeClusterOutput, error) {
	defer func() {
		e.Notify <- struct{}{}
	}()
	for _, cluster := range e.Clusters {
		if aws.StringValue(req.Name) == aws.StringValue(cluster.Name) {
			return &eks.DescribeClusterOutput{Cluster: cluster}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(req.Name))
}
