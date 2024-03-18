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

package aws

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
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
	Actions SliceOrString `json:"Action"`
	// Resources is a list of resources.
	Resources SliceOrString `json:"Resource,omitempty"`
	// Principals is a list of principals.
	// It can be a single string (eg "*") or a map.
	Principals StringOrMap `json:"Principal,omitempty"`
	// Conditions is a list of conditions that must be satisfied for the action to be allowed.
	// Example:
	// Condition:
	//    StringEquals:
	//        "proxy.example.com:aud": "discover.teleport"
	Conditions map[string]map[string]SliceOrString `json:"Condition,omitempty"`
	// StatementID is an optional identifier for the statement.
	StatementID string `json:"Sid,omitempty"`
}

// ensureResource ensures that the statement contains the specified resource.
//
// Returns true if the resource was already a part of the statement.
func (s *Statement) ensureResource(resource string) bool {
	if slices.Contains(s.Resources, resource) {
		return true
	}
	s.Resources = append(s.Resources, resource)
	return false
}
func (s *Statement) ensureResources(resources []string) {
	for _, resource := range resources {
		s.ensureResource(resource)
	}
}

// EqualStatement returns whether the receive statement is the same.
func (s *Statement) EqualStatement(other *Statement) bool {
	if s.Effect != other.Effect {
		return false
	}

	if !slices.Equal(s.Actions, other.Actions) {
		return false
	}

	if len(s.Principals) != len(other.Principals) {
		return false
	}

	for principalKind, principalList := range s.Principals {
		expectedPrincipalList := other.Principals[principalKind]
		if !slices.Equal(principalList, expectedPrincipalList) {
			return false
		}
	}

	if !slices.Equal(s.Resources, other.Resources) {
		return false
	}

	if len(s.Conditions) != len(other.Conditions) {
		return false
	}
	for conditionKind, conditionOp := range s.Conditions {
		expectedConditionOp := other.Conditions[conditionKind]

		if len(conditionOp) != len(expectedConditionOp) {
			return false
		}

		for conditionOpKind, conditionOpList := range conditionOp {
			expectedConditionOpList := expectedConditionOp[conditionOpKind]
			if !slices.Equal(conditionOpList, expectedConditionOpList) {
				return false
			}
		}
	}

	return true
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
func NewPolicyDocument(statements ...*Statement) *PolicyDocument {
	return &PolicyDocument{
		Version:    PolicyVersion,
		Statements: statements,
	}
}

// Ensure ensures that the policy document contains the specified resource
// action.
//
// Returns true if the resource action was already a part of the policy and
// false otherwise.
func (p *PolicyDocument) Ensure(effect, action, resource string) bool {
	if existingStatement := p.findStatement(effect, action); existingStatement != nil {
		return existingStatement.ensureResource(resource)
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

// EnsureStatements ensures that the policy document contains all resource
// actions from the provided statements.
//
// The main benefit of using this function (versus appending to p.Statements
// directly) is to avoid duplications.
func (p *PolicyDocument) EnsureStatements(statements ...*Statement) {
	for _, statement := range statements {
		if statement == nil {
			continue
		}

		// Try to find an existing statement by the action, and add the resources there.
		var newActions []string
		for _, action := range statement.Actions {
			if existingStatement := p.findStatement(statement.Effect, action); existingStatement != nil {
				existingStatement.ensureResources(statement.Resources)
			} else {
				newActions = append(newActions, action)
			}
		}

		// Add the leftover actions as a new statement.
		if len(newActions) > 0 {
			p.Statements = append(p.Statements, &Statement{
				Effect:    statement.Effect,
				Actions:   newActions,
				Resources: statement.Resources,
			})
		}
	}
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

// ForEach loops through each action and resource of each statement.
func (p *PolicyDocument) ForEach(fn func(effect, action, resource string)) {
	for _, statement := range p.Statements {
		for _, action := range statement.Actions {
			for _, resource := range statement.Resources {
				fn(statement.Effect, action, resource)
			}
		}
	}
}

func (p *PolicyDocument) findStatement(effect, action string) *Statement {
	for _, s := range p.Statements {
		if s.Effect != effect {
			continue
		}
		if slices.Contains(s.Actions, action) {
			return s
		}
	}
	return nil

}

// SliceOrString defines a type that can be either a single string or a slice.
//
// For example, these types can be either a single string or a slice:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_action.html
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_resource.html
type SliceOrString []string

// UnmarshalJSON implements json.Unmarshaller.
func (s *SliceOrString) UnmarshalJSON(bytes []byte) error {
	// Check if input is a slice of strings.
	var slice []string
	sliceErr := json.Unmarshal(bytes, &slice)
	if sliceErr == nil {
		*s = slice
		return nil
	}

	// Check if input is a single string.
	var str string
	strErr := json.Unmarshal(bytes, &str)
	if strErr == nil {
		*s = []string{str}
		return nil
	}

	// Failed both format.
	return trace.NewAggregate(sliceErr, strErr)
}

// MarshalJSON implements json.Marshaler.
func (s SliceOrString) MarshalJSON() ([]byte, error) {
	switch len(s) {
	case 0:
		return json.Marshal([]string{})
	case 1:
		return json.Marshal(s[0])
	default:
		return json.Marshal([]string(s))
	}
}

// StringOrMap defines a type that can be either a single string or a map.
//
// For almost every use case a map is used. Example:
// "Principal": { "Service": ["ecs.amazonaws.com", "elasticloadbalancing.amazonaws.com"]}
//
// For special use cases, like public/anonynous access, a "*" can be used:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html#principal-anonymous
type StringOrMap map[string]SliceOrString

// UnmarshalJSON implements json.Unmarshaller.
// If it contains a string and not a map, it will create a map with a single entry:
// { "str": [] }
// The only known example is for allowing anything, by using the "*"
// (See examples here // https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html#principal-anonymous)
func (s *StringOrMap) UnmarshalJSON(bytes []byte) error {
	// Check if input is a map.
	var mapInput map[string]SliceOrString
	mapErr := json.Unmarshal(bytes, &mapInput)
	if mapErr == nil {
		*s = mapInput
		return nil
	}

	// Check if input is a single string.
	var str string
	strErr := json.Unmarshal(bytes, &str)
	if strErr == nil {
		*s = StringOrMap{
			str: SliceOrString{},
		}
		return nil
	}

	// Failed both format.
	return trace.NewAggregate(mapErr, strErr)
}

// MarshalJSON implements json.Marshaler.
// It returns "*" if the map has a single key and that key has 0 items.
// The only known example is for allowing anything, by using the "*"
// (See examples here // https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html#principal-anonymous)
// The regular Marshal method is used otherwise.
func (s StringOrMap) MarshalJSON() ([]byte, error) {
	switch len(s) {
	case 0:
		return json.Marshal(map[string]SliceOrString{})
	case 1:
		if values, isWildcard := s[wildcard]; isWildcard && len(values) == 0 {
			return json.Marshal(wildcard)
		}
		fallthrough
	default:
		return json.Marshal(map[string]SliceOrString(s))
	}
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
	// partitionID is the partition ID.
	partitionID string
	// accountID current AWS account ID.
	accountID string
	// iamClient already initialized IAM client.
	iamClient iamiface.IAMAPI
}

// NewPolicies creates new instance of Policies using the provided
// identity, partitionID and IAM client.
func NewPolicies(partitionID string, accountID string, iamClient iamiface.IAMAPI) Policies {
	return &policies{
		partitionID: partitionID,
		accountID:   accountID,
		iamClient:   iamClient,
	}
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
	policyARN := awsutils.PolicyARN(p.partitionID, p.accountID, policy.Name)
	encodedPolicyDocument, err := json.Marshal(policy.Document)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Retrieve policy versions.
	_, versions, err := p.Retrieve(ctx, policyARN, policy.Tags)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
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
			return "", trace.Wrap(ConvertIAMError(err))
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
			return "", trace.Wrap(ConvertIAMError(err))
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
		return "", trace.Wrap(ConvertIAMError(err))
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
		return nil, nil, trace.Wrap(ConvertIAMError(err))
	}

	for tagName, tagValue := range tags {
		if !matchTag(getPolicyResp.Policy.Tags, tagName, tagValue) {
			return nil, nil, trace.AlreadyExists("policy %q doesn't have the tag %s=%q", arn, tagName, tagValue)
		}
	}

	resp, err := p.iamClient.ListPolicyVersionsWithContext(ctx, &iam.ListPolicyVersionsInput{PolicyArn: aws.String(arn)})
	if err != nil {
		return nil, nil, trace.Wrap(ConvertIAMError(err))
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
			return trace.Wrap(ConvertIAMError(err))
		}
	case Role, *Role:
		_, err := p.iamClient.AttachRolePolicyWithContext(ctx, &iam.AttachRolePolicyInput{
			PolicyArn: aws.String(arn),
			RoleName:  aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertIAMError(err))
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
			return trace.Wrap(ConvertIAMError(err))
		}
	case Role, *Role:
		_, err := p.iamClient.PutRolePermissionsBoundaryWithContext(ctx, &iam.PutRolePermissionsBoundaryInput{
			PermissionsBoundary: aws.String(arn),
			RoleName:            aws.String(identity.GetName()),
		})
		if err != nil {
			return trace.Wrap(ConvertIAMError(err))
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
