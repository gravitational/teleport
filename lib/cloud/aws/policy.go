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
	"log/slog"
	"net/url"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"

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
	Conditions Conditions `json:"Condition,omitempty"`
	// StatementID is an optional identifier for the statement.
	StatementID string `json:"Sid,omitempty"`
}

// Conditions is a list of conditions that must be satisfied for an action to be allowed.
type Conditions map[string]StringOrMap

// Equals returns true if conditions are equal.
func (a Conditions) Equals(b Conditions) bool {
	if len(a) != len(b) {
		return false
	}
	for conditionKindA, conditionOpA := range a {
		conditionOpB := b[conditionKindA]
		if !conditionOpA.Equals(conditionOpB) {
			return false
		}
	}
	return true
}

// ensureResource ensures that the statement contains the specified resource.
//
// Returns true if the resource was added to the statement or false if the
// resource was already part of the statement.
func (s *Statement) ensureResource(resource string) bool {
	if slices.Contains(s.Resources, resource) {
		return false
	}
	s.Resources = append(s.Resources, resource)
	return true
}
func (s *Statement) ensureResources(resources []string) bool {
	var updated bool
	for _, resource := range resources {
		updated = s.ensureResource(resource) || updated
	}
	return updated
}

// ensurePrincipal ensures that the statement contains the specified principal.
//
// Returns true if the principal was already a part of the statement.
func (s *Statement) ensurePrincipal(kind string, value string) bool {
	if len(s.Principals) == 0 {
		s.Principals = make(StringOrMap)
	}
	values := s.Principals[kind]
	if slices.Contains(values, value) {
		return false
	}
	values = append(values, value)
	s.Principals[kind] = values
	return true
}

func (s *Statement) ensurePrincipals(principals StringOrMap) bool {
	var updated bool
	for kind, values := range principals {
		for _, v := range values {
			updated = s.ensurePrincipal(kind, v) || updated
		}
	}
	return updated
}

// EqualStatement returns whether the receive statement is the same.
func (s *Statement) EqualStatement(other *Statement) bool {
	if s.Effect != other.Effect {
		return false
	}

	if !slices.Equal(s.Actions, other.Actions) {
		return false
	}

	if !s.Principals.Equals(other.Principals) {
		return false
	}

	if !slices.Equal(s.Resources, other.Resources) {
		return false
	}

	if !s.Conditions.Equals(other.Conditions) {
		return false
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

// EnsureResourceAction ensures that the policy document contains the specified
// resource action.
//
// Returns true if the resource action was added to the policy and false if it
// was already part of the policy.
func (p *PolicyDocument) EnsureResourceAction(effect, action, resource string, conditions Conditions) bool {
	if existingStatement := p.findStatement(effect, action, conditions); existingStatement != nil {
		return existingStatement.ensureResource(resource)
	}

	// No statement yet for this resource action, add it.
	p.Statements = append(p.Statements, &Statement{
		Effect:     effect,
		Actions:    []string{action},
		Resources:  []string{resource},
		Conditions: conditions,
	})
	return true
}

// Delete deletes the specified resource action from the policy.
func (p *PolicyDocument) DeleteResourceAction(effect, action, resource string, conditions Conditions) {
	var statements []*Statement
	for _, s := range p.Statements {
		if s.Effect != effect {
			statements = append(statements, s)
			continue
		}
		if !s.Conditions.Equals(conditions) {
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
func (p *PolicyDocument) EnsureStatements(statements ...*Statement) bool {
	var updated bool
	for _, statement := range statements {
		if statement == nil {
			continue
		}

		// Try to find an existing statement by the action, and add the resources there.
		var newActions []string
		for _, action := range statement.Actions {
			if existingStatement := p.findStatement(statement.Effect, action, statement.Conditions); existingStatement != nil {
				updated = existingStatement.ensureResources(statement.Resources) || updated
				updated = existingStatement.ensurePrincipals(statement.Principals) || updated
			} else {
				newActions = append(newActions, action)
			}
		}

		// Add the leftover actions as a new statement.
		if len(newActions) > 0 {
			p.Statements = append(p.Statements, &Statement{
				Effect:     statement.Effect,
				Actions:    newActions,
				Resources:  statement.Resources,
				Conditions: statement.Conditions,
				Principals: statement.Principals,
			})
			updated = true
		}
	}
	return updated
}

// IsEmpty returns whether the policy document is empty.
func (p *PolicyDocument) IsEmpty() bool {
	return len(p.Statements) == 0
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
func (p *PolicyDocument) ForEach(fn func(effect, action, resource string, conditions Conditions)) {
	for _, statement := range p.Statements {
		for _, action := range statement.Actions {
			for _, resource := range statement.Resources {
				fn(statement.Effect, action, resource, statement.Conditions)
			}
		}
	}
}

func (p *PolicyDocument) findStatement(effect, action string, conditions Conditions) *Statement {
	for _, s := range p.Statements {
		if s.Effect != effect {
			continue
		}
		if !slices.Contains(s.Actions, action) {
			continue
		}
		if !s.Conditions.Equals(conditions) {
			continue
		}
		return s
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

// Equals returns true if this StringOrMap is equal to another StringOrMap.
func (s StringOrMap) Equals(other StringOrMap) bool {
	if len(s) != len(other) {
		return false
	}

	for key, list := range s {
		otherList := other[key]
		if !slices.Equal(list, otherList) {
			return false
		}
	}
	return true
}

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
	// Attach attaches a policy with `arn` to the provided `identity`.
	Attach(ctx context.Context, arn string, identity Identity) error
}

// policies default implementation of the policies functions.
type policies struct {
	// partitionID is the partition ID.
	partitionID string
	// accountID current AWS account ID.
	accountID string
	// iamClient is an already initialized IAM client.
	iamClient IAMClient
}

// IAMClient describes the methods required to manage AWS IAM policies.
type IAMClient interface {
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
	AttachUserPolicy(ctx context.Context, params *iam.AttachUserPolicyInput, optFns ...func(*iam.Options)) (*iam.AttachUserPolicyOutput, error)
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
	CreatePolicyVersion(ctx context.Context, params *iam.CreatePolicyVersionInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyVersionOutput, error)
	DeletePolicyVersion(ctx context.Context, params *iam.DeletePolicyVersionInput, optFns ...func(*iam.Options)) (*iam.DeletePolicyVersionOutput, error)
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
	ListPolicyVersions(ctx context.Context, params *iam.ListPolicyVersionsInput, optFns ...func(*iam.Options)) (*iam.ListPolicyVersionsOutput, error)
}

var _ IAMClient = (*iam.Client)(nil)

// NewPolicies creates new instance of Policies using the provided
// identity, partitionID and IAM client.
func NewPolicies(partitionID string, accountID string, iamClient IAMClient) *policies {
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
	versions, err := p.getPolicyVersions(ctx, policyARN, policy.Tags)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}

	// If no versions were found, we need to create a new policy.
	if trace.IsNotFound(err) {
		policyARN, err := p.createPolicy(ctx, policy, encodedPolicyDocument)
		if err != nil {
			return "", trace.Wrap(err)
		}

		slog.DebugContext(ctx, "Created new policy",
			"policy_name", policy.Name,
			"policy_arn", policyARN,
		)
		return policyARN, nil
	}

	// Check number of policy versions and delete one if necessary.
	if len(versions) == maxPolicyVersions {
		// Sort versions based on create date.
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreateDate.Before(aws.ToTime(versions[j].CreateDate))
		})

		// Find the first version that is not default.
		var policyVersionID string
		for _, policyVersion := range versions {
			if !policyVersion.IsDefaultVersion {
				policyVersionID = aws.ToString(policyVersion.VersionId)
				break
			}
		}

		// Delete first non-default version.
		err = p.deletePolicyVersion(ctx, policyARN, policyVersionID)
		if err != nil {
			return "", trace.Wrap(err)
		}

		slog.DebugContext(ctx, "Max policy versions reached for policy, deleted non-default policy version",
			"policy_arn", policyARN,
			"policy_version", policyVersionID,
		)
	}

	// Create new policy version.
	versionID, err := p.createPolicyVersion(ctx, policyARN, encodedPolicyDocument)
	if err != nil {
		return "", trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Created new policy version",
		"policy_version", versionID,
		"policy_arn", policyARN,
	)
	return policyARN, nil
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
		if err := p.attachUserPolicy(ctx, arn, identity); err != nil {
			return trace.Wrap(err)
		}
	case Role, *Role:
		if err := p.attachRolePolicy(ctx, arn, identity); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("policies can be attached to users and roles, received %q.", identity.GetType())
	}

	return nil
}

// matchTag checks if tag name and value are present on the policy tags list.
func matchTag(policyTags []iamtypes.Tag, name, value string) bool {
	for _, policyTag := range policyTags {
		if aws.ToString(policyTag.Key) == name && aws.ToString(policyTag.Value) == value {
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

// CreatePolicy creates an IAM policy in AWS.
func (p *policies) createPolicy(ctx context.Context, policy *Policy, docJSON []byte) (string, error) {
	// Convert tags into IAM policy tags.
	policyTags := make([]iamtypes.Tag, 0, len(policy.Tags))
	for key, value := range policy.Tags {
		policyTags = append(policyTags, iamtypes.Tag{
			Key: aws.String(key), Value: aws.String(value),
		})
	}

	resp, err := p.iamClient.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(policy.Name),
		Description:    aws.String(policy.Description),
		PolicyDocument: aws.String(string(docJSON)),
		Tags:           policyTags,
	})
	if err != nil {
		return "", trace.Wrap(ConvertIAMError(err))
	}

	return aws.ToString(resp.Policy.Arn), nil
}

func (p *policies) deletePolicyVersion(ctx context.Context, policyARN, policyVersionID string) error {
	// Delete first non-default version.
	_, err := p.iamClient.DeletePolicyVersion(ctx, &iam.DeletePolicyVersionInput{
		PolicyArn: aws.String(policyARN),
		VersionId: aws.String(policyVersionID),
	})
	return trace.Wrap(ConvertIAMError(err))
}

func (p *policies) createPolicyVersion(ctx context.Context, policyARN string, docJSON []byte) (string, error) {
	createPolicyResp, err := p.iamClient.CreatePolicyVersion(ctx, &iam.CreatePolicyVersionInput{
		PolicyArn:      aws.String(policyARN),
		PolicyDocument: aws.String(string(docJSON)),
		SetAsDefault:   true,
	})
	if err != nil {
		return "", trace.Wrap(ConvertIAMError(err))
	}
	return aws.ToString(createPolicyResp.PolicyVersion.VersionId), nil
}

// getPolicyVersions retrieves policy versions. If the tags list is present,
// the policy should have all of them, otherwise an error is returned.
//
// Requires the following AWS permissions to be performed:
// * `iam:GetPolicy`: wildcard ("*") or the policy to be retrieved;
// * `iam.ListPolicyVersions`: wildcard ("*") or the policy to be retrieved;
func (p *policies) getPolicyVersions(ctx context.Context, policyARN string, tags map[string]string) ([]iamtypes.PolicyVersion, error) {
	getPolicyResp, err := p.iamClient.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: &policyARN})
	if err != nil {
		return nil, trace.Wrap(ConvertIAMError(err))
	}

	for tagName, tagValue := range tags {
		if !matchTag(getPolicyResp.Policy.Tags, tagName, tagValue) {
			return nil, trace.AlreadyExists("policy %q doesn't have the tag %s=%q", policyARN, tagName, tagValue)
		}
	}

	resp, err := p.iamClient.ListPolicyVersions(ctx, &iam.ListPolicyVersionsInput{PolicyArn: &policyARN})
	if err != nil {
		return nil, trace.Wrap(ConvertIAMError(err))
	}

	return resp.Versions, nil
}

func (p *policies) attachUserPolicy(ctx context.Context, policyARN string, identity Identity) error {
	_, err := p.iamClient.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{
		PolicyArn: aws.String(policyARN),
		UserName:  aws.String(identity.GetName()),
	})
	if err != nil {
		return trace.Wrap(ConvertIAMError(err))
	}
	return nil
}

func (p *policies) attachRolePolicy(ctx context.Context, policyARN string, identity Identity) error {
	_, err := p.iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: aws.String(policyARN),
		RoleName:  aws.String(identity.GetName()),
	})
	if err != nil {
		return trace.Wrap(ConvertIAMError(err))
	}
	return nil
}
