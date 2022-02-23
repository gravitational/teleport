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

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// Policy represents an AWS IAM policy.
type Policy struct {
	// Name is the policy name.
	Name string
	// Description is the policy description.
	Description string
	// Tags is the policy tags.
	Tags map[string]string
	// PolicyDocument is the IAM policy document.
	Document *PolicyDocument
}

// NewPolicy returns a new AWS IAM Policy.
func NewPolicy(name, description string, tags map[string]string, document *PolicyDocument) *Policy {
	return &Policy{
		Name:        name,
		Description: description,
		Tags:        tags,
		Document:    document,
	}
}

// PolicyDocument represents a parsed AWS IAM policy document.
//
// Note that PolicyDocument and its Ensure/Delete methods are not currently
// goroutine-safe. To create a policy using AWS IAM API, dump the object to
// JSON format using json.Marshal.
type PolicyDocument struct {
	// Version is the policy version.
	Version string `json:"Version"`
	// Statements is a list of the policy statements.
	Statements []*Statement `json:"Statement"`
}

// Statement is a single AWS IAM policy statement.
type Statement struct {
	// Effect is the statement effect such as Allow or Deny.
	Effect string `json:"Effect"`
	// Actions is a list of actions.
	Actions []string `json:"Action"`
	// Resources is a list of resources.
	Resources []string `json:"Resource"`
}

// ParsePolicyDocument returns parsed AWS IAM policy document.
func ParsePolicyDocument(document string) (*PolicyDocument, error) {
	// Policy document returned from AWS API can be URL-encoded:
	// https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetRolePolicy.html
	decoded, err := url.QueryUnescape(document)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var parsed PolicyDocument
	if err := json.Unmarshal([]byte(decoded), &parsed); err != nil {
		return nil, trace.Wrap(err)
	}
	return &parsed, nil
}

// NewPolicyDocument returns new empty AWS IAM policy document.
func NewPolicyDocument() *PolicyDocument {
	return &PolicyDocument{
		Version: PolicyVersion,
	}
}

// Ensure ensures that the policy document contains the specified resource
// action.
//
// Returns true if the resource action was already a part of the policy and
// false otherwise.
func (p *PolicyDocument) Ensure(effect, action, resource string) bool {
	for _, s := range p.Statements {
		if s.Effect != effect {
			continue
		}
		for _, a := range s.Actions {
			if a != action {
				continue
			}
			for _, r := range s.Resources {
				// Resource action is already in the policy.
				if r == resource {
					return true
				}
			}
			// Action exists but resource is missing.
			s.Resources = append(s.Resources, resource)
			return false
		}
	}
	// No statement yet for this resource action, add it.
	p.Statements = append(p.Statements, &Statement{
		Effect:    effect,
		Actions:   []string{action},
		Resources: []string{resource},
	})
	return false
}

// Delete deletes the specified resource action from the policy.
func (p *PolicyDocument) Delete(effect, action, resource string) {
	var statements []*Statement
	for _, s := range p.Statements {
		if s.Effect != effect {
			statements = append(statements, s)
			continue
		}
		var resources []string
		for _, a := range s.Actions {
			for _, r := range s.Resources {
				if a != action || r != resource {
					resources = append(resources, r)
				}
			}
		}
		if len(resources) != 0 {
			statements = append(statements, &Statement{
				Effect:    s.Effect,
				Actions:   s.Actions,
				Resources: resources,
			})
		}
	}
	p.Statements = statements
}

// Marshal formats the PolicyDocument in a "friendly" format, which can be
// presented to end users.
func (p *PolicyDocument) Marshal() (string, error) {
	b, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(b), nil
}

// Policies set of IAM Policy helper functions defined as an interface to make
// easier for other packages to mock and test with it.
type Policies interface {
	// Upsert creates a new Policy or creates a Policy version if a policy with
	// the same name already exists.
	Upsert(ctx context.Context, policy *Policy) (arn string, err error)
	// Retrieve retrieves a policy and its versions. If the tags list is
	// present, the Policy should have all of them, otherwise an error is
	// returned.
	Retrieve(ctx context.Context, arn string, tags map[string]string) (policy *iam.Policy, policyVersions []*iam.PolicyVersion, err error)
	// Attach attaches a policy with `arn` to the provided `identity`.
	Attach(ctx context.Context, arn string, identity Identity) error
	// AttachBoundary attaches a policy boundary with `arn` to the provided
	// `identity`.
	AttachBoundary(ctx context.Context, arn string, identity Identity) error
}

// policies default implementation of the policies functions.
type policies struct {
	// accountID current AWS account ID.
	accountID string
	// iamClient already initialized IAM client.
	iamClient iamiface.IAMAPI
}

// NewPolicies creates new instance of Policies using the provided
// identity and IAM client.
func NewPolicies(accountID string, iamClient iamiface.IAMAPI) Policies {
	return &policies{accountID, iamClient}
}

// Upsert creates a new Policy or creates a Policy version if a policy with the
// same name already exists.
//
// Since policies can have a limited number of versions, we need to delete a
// policy version (if the limit is reached) and create a new version. Check the
// constant `maxPolicyVersions` for reference.
//
// Requires the following AWS permissions to be performed:
// * All permissions required to run `Retrieve` function;
// * `iam:CreatePolicy`: wildcard ("*") or policy that will be created;
// * `iam:DeletePolicyVersion`: wildcard ("*") or policy that will be created;
// * `iam:CreatePolicyVersion`: wildcard ("*") or policy that will be created;
func (p *policies) Upsert(ctx context.Context, policy *Policy) (string, error) {
	policyARN := fmt.Sprintf("arn:aws:iam::%s:policy/%s", p.accountID, policy.Name)
	encodedPolicyDocument, err := json.Marshal(policy.Document)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Retrieve policy versions.
	_, versions, err := p.Retrieve(ctx, policyARN, policy.Tags)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(ConvertRequestFailureError(err))
	}

	// Convert tags into IAM policy tags.
	policyTags := make([]*iam.Tag, 0, len(policy.Tags))
	for key, value := range policy.Tags {
		policyTags = append(policyTags, &iam.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	// If no versions were found, we need to create a new policy.
	if trace.IsNotFound(err) {
		resp, err := p.iamClient.CreatePolicyWithContext(ctx, &iam.CreatePolicyInput{
			PolicyName:     aws.String(policy.Name),
			Description:    aws.String(policy.Description),
			PolicyDocument: aws.String(string(encodedPolicyDocument)),
			Tags:           policyTags,
		})
		if err != nil {
			return "", trace.Wrap(ConvertRequestFailureError(err))
		}

		log.Debugf("Created new policy %q with ARN %q", policy.Name, aws.StringValue(resp.Policy.Arn))
		return aws.StringValue(resp.Policy.Arn), nil
	}

	// Check number of policy versions and delete one if necessary.
	if len(versions) == maxPolicyVersions {
		// Sort versions based on create date.
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreateDate.Before(aws.TimeValue(versions[j].CreateDate))
		})

		// Find the first version that is not default.
		var policyVersionID string
		for _, policyVersion := range versions {
			if !aws.BoolValue(policyVersion.IsDefaultVersion) {
				policyVersionID = *policyVersion.VersionId
				break
			}
		}

		// Delete first non-default version.
		_, err := p.iamClient.DeletePolicyVersionWithContext(ctx, &iam.DeletePolicyVersionInput{
			PolicyArn: aws.String(policyARN),
			VersionId: aws.String(policyVersionID),
		})
		if err != nil {
			return "", trace.Wrap(ConvertRequestFailureError(err))
		}

		log.Debugf("Max policy versions reached for policy %q, deleted policy version %q", policyARN, policyVersionID)
	}

	// Create new policy version.
	createPolicyResp, err := p.iamClient.CreatePolicyVersionWithContext(ctx, &iam.CreatePolicyVersionInput{
		PolicyArn:      aws.String(policyARN),
		PolicyDocument: aws.String(string(encodedPolicyDocument)),
		SetAsDefault:   aws.Bool(true),
	})
	if err != nil {
		return "", trace.Wrap(ConvertRequestFailureError(err))
	}

	log.Debugf("Created new policy version %q for %q", aws.StringValue(createPolicyResp.PolicyVersion.VersionId), policyARN)
	return policyARN, nil
}

// Retrieve retrieves a policy and its versions. If the tags list is present,
// the Policy should have all of them, otherwise an error is returned.
//
// Requires the following AWS permissions to be performed:
// * `iam:GetPolicy`: wildcard ("*") or the policy to be retrieved;
// * `iam.ListPolicyVersions`: wildcard ("*") or the policy to be retrieved;
func (p *policies) Retrieve(ctx context.Context, arn string, tags map[string]string) (*iam.Policy, []*iam.PolicyVersion, error) {
	getPolicyResp, err := p.iamClient.GetPolicyWithContext(ctx, &iam.GetPolicyInput{PolicyArn: aws.String(arn)})
	if err != nil {
		return nil, nil, trace.Wrap(ConvertRequestFailureError(err))
	}

	for tagName, tagValue := range tags {
		if !matchTag(getPolicyResp.Policy.Tags, tagName, tagValue) {
			return nil, nil, trace.AlreadyExists("policy %q doesn't have the tag %s=%q", arn, tagName, tagValue)
		}
	}

	resp, err := p.iamClient.ListPolicyVersionsWithContext(ctx, &iam.ListPolicyVersionsInput{PolicyArn: aws.String(arn)})
	if err != nil {
		return nil, nil, trace.Wrap(ConvertRequestFailureError(err))
	}

	return getPolicyResp.Policy, resp.Versions, nil
}

// Attach attaches a policy with `arn` to the provided `identity`.
//
// Only `User` and `Role` identities supported.
//
// Requires the following AWS permissions to be performed:
// * `iam:AttachUserPolicy`: wildcard ("*") or provided user identity;
// * `iam:AttachRolePolicy`: wildcard ("*") or provided role identity;
func (p *policies) Attach(ctx context.Context, arn string, identity Identity) error {
	switch identity.(type) {
	case User, *User:
		_, err := p.iamClient.AttachUserPolicyWithContext(ctx, &iam.AttachUserPolicyInput{
			PolicyArn: aws.String(arn),
			UserName:  aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertRequestFailureError(err))
		}
	case Role, *Role:
		_, err := p.iamClient.AttachRolePolicyWithContext(ctx, &iam.AttachRolePolicyInput{
			PolicyArn: aws.String(arn),
			RoleName:  aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertRequestFailureError(err))
		}
	default:
		return trace.BadParameter("policies can be attached to users and roles, received %q.", identity.GetType())
	}

	return nil
}

// AttachBoundary attaches a policy boundary with `arn` to the provided
// `identity`.
//
// Only `User` and `Role` identities supported.
//
// Requires the following AWS permissions to be performed:
// * `iam:PutUserPermissionsBoundary`: wildcard ("*") or provided user identity;
// * `iam:PutRolePermissionsBoundary`: wildcard ("*") or provided role identity;
func (p *policies) AttachBoundary(ctx context.Context, arn string, identity Identity) error {
	switch identity.(type) {
	case User, *User:
		_, err := p.iamClient.PutUserPermissionsBoundaryWithContext(ctx, &iam.PutUserPermissionsBoundaryInput{
			PermissionsBoundary: aws.String(arn),
			UserName:            aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertRequestFailureError(err))
		}
	case Role, *Role:
		_, err := p.iamClient.PutRolePermissionsBoundaryWithContext(ctx, &iam.PutRolePermissionsBoundaryInput{
			PermissionsBoundary: aws.String(arn),
			RoleName:            aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertRequestFailureError(err))
		}
	default:
		return trace.BadParameter("boundary policies can be attached to users and roles, received %q.", identity.GetType())
	}

	return nil
}

// matchTag checks if tag name and value are present on the policy tags list.
func matchTag(policyTags []*iam.Tag, name, value string) bool {
	for _, policyTag := range policyTags {
		if *policyTag.Key == name && *policyTag.Value == value {
			return true
		}
	}

	return false
}

const (
	// PolicyVersion is default IAM policy version.
	PolicyVersion = "2012-10-17"
	// EffectAllow is the Allow IAM policy effect.
	EffectAllow = "Allow"
	// EffectDeny is the Deny IAM policy effect.
	EffectDeny = "Deny"
	// maxPolicyVersions max number of policy versions a policy can have.
	// Ref: https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_managed-versioning.html#version-limits
	maxPolicyVersions = 5
)
