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
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// InlinePolicy defines an interface for an AWS IAM inline policy.
type InlinePolicy interface {
	// GetName returns the inline policy name.
	GetName() string

	// Get fetches and returns the policy.
	Get(ctx context.Context) (*PolicyDocument, error)
	// Put updates the policy and creates if not exists.
	Put(ctx context.Context, policy *PolicyDocument) error
	// Delete detaches the policy from the identity.
	Delete(ctx context.Context) error
}

// NewRoleInlinePolicy creates an inline policy for provided IAM role.
func NewRoleInlinePolicy(name, role string, iam iamiface.IAMAPI) (InlinePolicy, error) {
	if iam == nil {
		return nil, trace.BadParameter("missing IAM client")
	}
	if name == "" {
		return nil, trace.BadParameter("policy cannot be empty")
	}
	if role == "" {
		return nil, trace.BadParameter("role cannot be empty")
	}
	return &roleInlinePolicy{
		name: name,
		role: role,
		iam:  iam,
	}, nil
}

// NewUserInlinePolicy creates an inline policy for provided IAM user.
func NewUserInlinePolicy(name, user string, iam iamiface.IAMAPI) (InlinePolicy, error) {
	if iam == nil {
		return nil, trace.BadParameter("missing IAM client")
	}
	if name == "" {
		return nil, trace.BadParameter("policy cannot be empty")
	}
	if user == "" {
		return nil, trace.BadParameter("user cannot be empty")
	}
	return &userInlinePolicy{
		name: name,
		user: user,
		iam:  iam,
	}, nil
}

// NewInlinePolicyForIdentity creates an inline policy from provided identity.
func NewInlinePolicyForIdentity(name string, iam iamiface.IAMAPI, identity Identity) (InlinePolicy, error) {
	switch identity.(type) {
	case Role:
		return NewRoleInlinePolicy(name, identity.GetName(), iam)

	case User:
		return NewUserInlinePolicy(name, identity.GetName(), iam)
	}
	return nil, trace.BadParameter("unsupported identity: %v", identity)
}

// roleInlinePolicy represents an inline policy for an AWS IAM role.
type roleInlinePolicy struct {
	name string
	role string
	iam  iamiface.IAMAPI
}

// GetName returns the inline policy name.
func (p *roleInlinePolicy) GetName() string {
	return p.name
}

// Get fetches and returns the policy.
func (p *roleInlinePolicy) Get(ctx context.Context) (*PolicyDocument, error) {
	out, err := p.iam.GetRolePolicyWithContext(ctx, &iam.GetRolePolicyInput{
		PolicyName: aws.String(p.name),
		RoleName:   aws.String(p.role),
	})
	if err != nil {
		return nil, ConvertRequestFailureError(err)
	}

	return ParsePolicyDocument(aws.StringValue(out.PolicyDocument))
}

// Put updates the policy and creates if not exists.
func (p *roleInlinePolicy) Put(ctx context.Context, policyDocument *PolicyDocument) error {
	log.Debugf("Putting IAM policy for role %v.", p.role)

	document, err := json.Marshal(policyDocument)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.iam.PutRolePolicyWithContext(ctx, &iam.PutRolePolicyInput{
		PolicyName:     aws.String(p.name),
		PolicyDocument: aws.String(string(document)),
		RoleName:       aws.String(p.role),
	})
	return ConvertRequestFailureError(err)
}

// Delete detaches the policy from the identity.
func (p *roleInlinePolicy) Delete(ctx context.Context) error {
	log.Debugf("Deleting IAM policy from role %v.", p.role)
	_, err := p.iam.DeleteRolePolicyWithContext(ctx, &iam.DeleteRolePolicyInput{
		PolicyName: aws.String(p.name),
		RoleName:   aws.String(p.role),
	})
	return ConvertRequestFailureError(err)
}

// userInlinePolicy represents an inline policy for an AWS IAM user.
type userInlinePolicy struct {
	name string
	user string
	iam  iamiface.IAMAPI
}

// GetName returns the inline policy name.
func (p *userInlinePolicy) GetName() string {
	return p.name
}

// Get fetches and returns the policy.
func (p *userInlinePolicy) Get(ctx context.Context) (*PolicyDocument, error) {
	out, err := p.iam.GetUserPolicyWithContext(ctx, &iam.GetUserPolicyInput{
		PolicyName: aws.String(p.name),
		UserName:   aws.String(p.user),
	})
	if err != nil {
		return nil, ConvertRequestFailureError(err)
	}

	return ParsePolicyDocument(aws.StringValue(out.PolicyDocument))
}

// Put updates the policy and creates if not exists.
func (p *userInlinePolicy) Put(ctx context.Context, policyDocument *PolicyDocument) error {
	log.Debugf("Putting IAM policy for user %v.", p.user)

	document, err := json.Marshal(policyDocument)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.iam.PutUserPolicyWithContext(ctx, &iam.PutUserPolicyInput{
		PolicyName:     aws.String(p.name),
		PolicyDocument: aws.String(string(document)),
		UserName:       aws.String(p.user),
	})
	return ConvertRequestFailureError(err)
}

// Delete detaches the policy from the identity.
func (p *userInlinePolicy) Delete(ctx context.Context) error {
	log.Debugf("Deleting IAM policy from user %v.", p.user)
	_, err := p.iam.DeleteUserPolicyWithContext(ctx, &iam.DeleteUserPolicyInput{
		PolicyName: aws.String(p.name),
		UserName:   aws.String(p.user),
	})
	return ConvertRequestFailureError(err)
}
