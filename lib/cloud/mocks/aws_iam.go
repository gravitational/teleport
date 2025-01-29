/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"
)

// IAMMock mocks AWS IAM API.
type IAMMock struct {
	// Error can be set to make all API calls return that error.
	// Takes precedence over Unauth, but it doesn't make sense to set both.
	Error error
	// Unauth can be set to make all API calls return access denied.
	Unauth bool

	mu sync.RWMutex
	// attachedRolePolicies maps roleName -> policyName -> policyDocument
	attachedRolePolicies map[string]map[string]string
	// attachedUserPolicies maps userName -> policyName -> policyDocument
	attachedUserPolicies map[string]map[string]string
	// SAMLProviders maps saml provider ARN -> samlProvider
	SAMLProviders map[string]*iam.GetSAMLProviderOutput
	// OIDCProviders maps saml provider ARN -> oidcProvider
	OIDCProviders map[string]*iam.GetOpenIDConnectProviderOutput
}

func (m *IAMMock) GetRolePolicy(ctx context.Context, input *iam.GetRolePolicyInput, options ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

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

func (m *IAMMock) PutRolePolicy(ctx context.Context, input *iam.PutRolePolicyInput, options ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

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

func (m *IAMMock) DeleteRolePolicy(ctx context.Context, input *iam.DeleteRolePolicyInput, options ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attachedRolePolicies[*input.RoleName]; ok {
		delete(m.attachedRolePolicies[*input.RoleName], *input.PolicyName)
	}
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (m *IAMMock) GetUserPolicy(ctx context.Context, input *iam.GetUserPolicyInput, options ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

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

func (m *IAMMock) PutUserPolicy(ctx context.Context, input *iam.PutUserPolicyInput, options ...func(*iam.Options)) (*iam.PutUserPolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

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

func (m *IAMMock) DeleteUserPolicy(ctx context.Context, input *iam.DeleteUserPolicyInput, options ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attachedUserPolicies[*input.UserName]; ok {
		delete(m.attachedUserPolicies[*input.UserName], *input.PolicyName)
	}
	return &iam.DeleteUserPolicyOutput{}, nil
}

func (m *IAMMock) ListSAMLProviders(ctx context.Context, input *iam.ListSAMLProvidersInput, options ...func(*iam.Options)) (*iam.ListSAMLProvidersOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	resp := &iam.ListSAMLProvidersOutput{}
	for arn := range m.SAMLProviders {
		resp.SAMLProviderList = append(resp.SAMLProviderList,
			iamtypes.SAMLProviderListEntry{
				Arn: aws.String(arn),
			},
		)
	}
	return resp, nil
}

func (m *IAMMock) GetSAMLProvider(ctx context.Context, input *iam.GetSAMLProviderInput, options ...func(*iam.Options)) (*iam.GetSAMLProviderOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if input.SAMLProviderArn == nil {
		return nil, trace.BadParameter("SAMLProviderARN must not be nil")
	}
	provider, ok := m.SAMLProviders[*input.SAMLProviderArn]
	if !ok {
		return nil, trace.BadParameter("SAML provider %q not found", *input.SAMLProviderArn)
	}
	return provider, nil
}

func (m *IAMMock) ListOpenIDConnectProviders(ctx context.Context, input *iam.ListOpenIDConnectProvidersInput, options ...func(*iam.Options)) (*iam.ListOpenIDConnectProvidersOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	resp := &iam.ListOpenIDConnectProvidersOutput{}
	for arn := range m.OIDCProviders {
		resp.OpenIDConnectProviderList = append(resp.OpenIDConnectProviderList,
			iamtypes.OpenIDConnectProviderListEntry{
				Arn: aws.String(arn),
			},
		)
	}
	return resp, nil
}

func (m *IAMMock) GetOpenIDConnectProvider(ctx context.Context, input *iam.GetOpenIDConnectProviderInput, options ...func(*iam.Options)) (*iam.GetOpenIDConnectProviderOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if input.OpenIDConnectProviderArn == nil {
		return nil, trace.BadParameter("OpenIDConnectProviderARN must not be nil")
	}
	provider, ok := m.OIDCProviders[*input.OpenIDConnectProviderArn]
	if !ok {
		return nil, trace.BadParameter("OIDC provider %q not found", *input.OpenIDConnectProviderArn)
	}
	return provider, nil
}

func (m *IAMMock) GetGroupPolicy(ctx context.Context, params *iam.GetGroupPolicyInput, optFns ...func(*iam.Options)) (*iam.GetGroupPolicyOutput, error) {
	return nil, trace.NotImplemented("GetGroupPolicy not implemented")
}

func (m *IAMMock) GetPolicyVersion(context.Context, *iam.GetPolicyVersionInput, ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return nil, trace.NotImplemented("GetPolicyVersion not implemented")
}

func (m *IAMMock) ListAttachedGroupPolicies(context.Context, *iam.ListAttachedGroupPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return nil, trace.NotImplemented("ListAttachedGroupPolicies not implemented")
}

func (m *IAMMock) ListAttachedRolePolicies(context.Context, *iam.ListAttachedRolePoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return nil, trace.NotImplemented("ListAttachedRolePolicies not implemented")
}

func (m *IAMMock) ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return nil, trace.NotImplemented("ListAttachedUserPolicies not implemented")
}

func (m *IAMMock) ListGroupPolicies(context.Context, *iam.ListGroupPoliciesInput, ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return nil, trace.NotImplemented("ListGroupPolicies not implemented")
}

func (m *IAMMock) ListGroups(context.Context, *iam.ListGroupsInput, ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return nil, trace.NotImplemented("ListGroups not implemented")
}

func (m *IAMMock) ListGroupsForUser(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return nil, trace.NotImplemented("ListGroupsForUser not implemented")
}

func (m *IAMMock) ListInstanceProfiles(context.Context, *iam.ListInstanceProfilesInput, ...func(*iam.Options)) (*iam.ListInstanceProfilesOutput, error) {
	return nil, trace.NotImplemented("ListInstanceProfiles not implemented")
}

func (m *IAMMock) ListPolicies(context.Context, *iam.ListPoliciesInput, ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return nil, trace.NotImplemented("ListPolicies not implemented")
}

func (m *IAMMock) ListRolePolicies(context.Context, *iam.ListRolePoliciesInput, ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return nil, trace.NotImplemented("ListRolePolicies not implemented")
}

func (m *IAMMock) ListRoles(context.Context, *iam.ListRolesInput, ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return nil, trace.NotImplemented("ListRoles not implemented")
}

func (m *IAMMock) ListUserPolicies(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	return nil, trace.NotImplemented("ListUserPolicies not implemented")
}

func (m *IAMMock) ListUsers(context.Context, *iam.ListUsersInput, ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return nil, trace.NotImplemented("ListUsers not implemented")
}
