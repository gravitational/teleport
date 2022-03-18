/*
Copyright 2022 Gravitational, Inc.

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

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// InlinePolicyClient defines an interface for an AWS IAM inline policy client.
type InlinePolicyClient interface {
	// GetPolicyName returns the inline policy name.
	GetPolicyName() string

	// Get fetches and returns the policy.
	Get(ctx context.Context) (*PolicyDocument, error)
	// Put updates the policy and creates if not exists.
	Put(ctx context.Context, policy *PolicyDocument) error
	// Delete detaches the policy from the identity.
	Delete(ctx context.Context) error
}

// NewInlinePolicyClientForRole creates an inline policy client for provided
// policy name and IAM role.
func NewInlinePolicyClientForRole(policyName, roleName string, iam iamiface.IAMAPI) (InlinePolicyClient, error) {
	if iam == nil {
		return nil, trace.BadParameter("missing IAM client")
	}
	if policyName == "" {
		return nil, trace.BadParameter("policy name cannot be empty")
	}
	if roleName == "" {
		return nil, trace.BadParameter("role name cannot be empty")
	}
	return &inlinePolicyClientForRole{
		policyName: policyName,
		roleName:   roleName,
		iam:        iam,
	}, nil
}

// NewInlinePolicyClientForUser creates an inline policy client for provided
// policy name and IAM user.
func NewInlinePolicyClientForUser(policyName, userName string, iam iamiface.IAMAPI) (InlinePolicyClient, error) {
	if iam == nil {
		return nil, trace.BadParameter("missing IAM client")
	}
	if policyName == "" {
		return nil, trace.BadParameter("policy name cannot be empty")
	}
	if userName == "" {
		return nil, trace.BadParameter("user name cannot be empty")
	}
	return &inlinePolicyClientForUser{
		policyName: policyName,
		userName:   userName,
		iam:        iam,
	}, nil
}

// NewInlinePolicyClientForIdentity creates an inline policy client from
// provided policy name and identity.
func NewInlinePolicyClientForIdentity(policyName string, iam iamiface.IAMAPI, identity Identity) (InlinePolicyClient, error) {
	switch identity.(type) {
	case Role:
		return NewInlinePolicyClientForRole(policyName, identity.GetName(), iam)

	case User:
		return NewInlinePolicyClientForUser(policyName, identity.GetName(), iam)
	}
	return nil, trace.BadParameter("unsupported identity: %v", identity)
}

// inlinePolicyClientForRole represents an inline policy client for an AWS IAM
// role.
type inlinePolicyClientForRole struct {
	policyName string
	roleName   string
	iam        iamiface.IAMAPI
}

// GetPolicyName returns the inline policy name.
func (p *inlinePolicyClientForRole) GetPolicyName() string {
	return p.policyName
}

// Get fetches and returns the policy.
func (p *inlinePolicyClientForRole) Get(ctx context.Context) (*PolicyDocument, error) {
	out, err := p.iam.GetRolePolicyWithContext(ctx, &iam.GetRolePolicyInput{
		PolicyName: aws.String(p.policyName),
		RoleName:   aws.String(p.roleName),
	})
	if err != nil {
		return nil, ConvertRequestFailureError(err)
	}

	return ParsePolicyDocument(aws.StringValue(out.PolicyDocument))
}

// Put updates the policy and creates if not exists.
func (p *inlinePolicyClientForRole) Put(ctx context.Context, policyDocument *PolicyDocument) error {
	log.Debugf("Putting IAM policy %v for role %v.", p.policyName, p.roleName)

	document, err := policyDocument.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.iam.PutRolePolicyWithContext(ctx, &iam.PutRolePolicyInput{
		PolicyName:     aws.String(p.policyName),
		PolicyDocument: aws.String(string(document)),
		RoleName:       aws.String(p.roleName),
	})
	return ConvertRequestFailureError(err)
}

// Delete detaches the policy from the identity.
func (p *inlinePolicyClientForRole) Delete(ctx context.Context) error {
	log.Debugf("Deleting IAM policy %v from role %v.", p.policyName, p.roleName)
	_, err := p.iam.DeleteRolePolicyWithContext(ctx, &iam.DeleteRolePolicyInput{
		PolicyName: aws.String(p.policyName),
		RoleName:   aws.String(p.roleName),
	})
	return ConvertRequestFailureError(err)
}

// inlinePolicyClientForUser represents an inline policy client for an AWS IAM
// user.
type inlinePolicyClientForUser struct {
	policyName string
	userName   string
	iam        iamiface.IAMAPI
}

// GetPolicyName returns the inline policy name.
func (p *inlinePolicyClientForUser) GetPolicyName() string {
	return p.policyName
}

// Get fetches and returns the policy.
func (p *inlinePolicyClientForUser) Get(ctx context.Context) (*PolicyDocument, error) {
	out, err := p.iam.GetUserPolicyWithContext(ctx, &iam.GetUserPolicyInput{
		PolicyName: aws.String(p.policyName),
		UserName:   aws.String(p.userName),
	})
	if err != nil {
		return nil, ConvertRequestFailureError(err)
	}

	return ParsePolicyDocument(aws.StringValue(out.PolicyDocument))
}

// Put updates the policy and creates if not exists.
func (p *inlinePolicyClientForUser) Put(ctx context.Context, policyDocument *PolicyDocument) error {
	log.Debugf("Putting IAM policy %v for user %v.", p.policyName, p.userName)

	document, err := policyDocument.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.iam.PutUserPolicyWithContext(ctx, &iam.PutUserPolicyInput{
		PolicyName:     aws.String(p.policyName),
		PolicyDocument: aws.String(string(document)),
		UserName:       aws.String(p.userName),
	})
	return ConvertRequestFailureError(err)
}

// Delete detaches the policy from the identity.
func (p *inlinePolicyClientForUser) Delete(ctx context.Context) error {
	log.Debugf("Deleting IAM policy %v from user %v.", p.policyName, p.userName)
	_, err := p.iam.DeleteUserPolicyWithContext(ctx, &iam.DeleteUserPolicyInput{
		PolicyName: aws.String(p.policyName),
		UserName:   aws.String(p.userName),
	})
	return ConvertRequestFailureError(err)
}
