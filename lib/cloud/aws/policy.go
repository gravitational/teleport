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
	"encoding/json"
	"net/url"

	"github.com/gravitational/trace"
)

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

const (
	// PolicyVersion is default IAM policy version.
	PolicyVersion = "2012-10-17"
	// EffectAllow is the Allow IAM policy effect.
	EffectAllow = "Allow"
	// EffectDeny is the Deny IAM policy effect.
	EffectDeny = "Deny"
)
