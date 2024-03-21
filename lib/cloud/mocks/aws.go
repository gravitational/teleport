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
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
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

// EKSMock is a mock EKS client.
type EKSMock struct {
	eksiface.EKSAPI
	Clusters           []*eks.Cluster
	AccessEntries      []*eks.AccessEntry
	AssociatedPolicies []*eks.AssociatedAccessPolicy
	Notify             chan struct{}
}

func (e *EKSMock) DescribeClusterWithContext(_ aws.Context, req *eks.DescribeClusterInput, _ ...request.Option) (*eks.DescribeClusterOutput, error) {
	defer func() {
		if e.Notify != nil {
			e.Notify <- struct{}{}
		}
	}()
	for _, cluster := range e.Clusters {
		if aws.StringValue(req.Name) == aws.StringValue(cluster.Name) {
			return &eks.DescribeClusterOutput{Cluster: cluster}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(req.Name))
}

func (e *EKSMock) ListClustersPagesWithContext(_ aws.Context, _ *eks.ListClustersInput, f func(*eks.ListClustersOutput, bool) bool, _ ...request.Option) error {
	defer func() {
		if e.Notify != nil {
			e.Notify <- struct{}{}
		}
	}()
	clusters := make([]*string, 0, len(e.Clusters))
	for _, cluster := range e.Clusters {
		clusters = append(clusters, cluster.Name)
	}
	f(&eks.ListClustersOutput{
		Clusters: clusters,
	}, true)
	return nil
}

func (e *EKSMock) ListAccessEntriesPagesWithContext(_ aws.Context, _ *eks.ListAccessEntriesInput, f func(*eks.ListAccessEntriesOutput, bool) bool, _ ...request.Option) error {
	defer func() {
		if e.Notify != nil {
			e.Notify <- struct{}{}
		}
	}()
	accessEntries := make([]*string, 0, len(e.Clusters))
	for _, a := range e.AccessEntries {
		accessEntries = append(accessEntries, a.PrincipalArn)
	}
	f(&eks.ListAccessEntriesOutput{
		AccessEntries: accessEntries,
	}, true)
	return nil
}

func (e *EKSMock) DescribeAccessEntryWithContext(_ aws.Context, req *eks.DescribeAccessEntryInput, _ ...request.Option) (*eks.DescribeAccessEntryOutput, error) {
	defer func() {
		if e.Notify != nil {
			e.Notify <- struct{}{}
		}
	}()
	for _, a := range e.AccessEntries {
		if aws.StringValue(req.PrincipalArn) == aws.StringValue(a.PrincipalArn) && aws.StringValue(a.ClusterName) == aws.StringValue(req.ClusterName) {
			return &eks.DescribeAccessEntryOutput{AccessEntry: a}, nil
		}
	}
	return nil, trace.NotFound("access entry %v not found", aws.StringValue(req.PrincipalArn))
}

func (e *EKSMock) ListAssociatedAccessPoliciesPagesWithContext(_ aws.Context, _ *eks.ListAssociatedAccessPoliciesInput, f func(*eks.ListAssociatedAccessPoliciesOutput, bool) bool, _ ...request.Option) error {
	defer func() {
		if e.Notify != nil {
			e.Notify <- struct{}{}
		}
	}()

	f(&eks.ListAssociatedAccessPoliciesOutput{
		AssociatedAccessPolicies: e.AssociatedPolicies,
	}, true)
	return nil

}
