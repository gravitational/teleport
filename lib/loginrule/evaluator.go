// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loginrule

import "context"

// EvaluationInput holds the inputs to a login rule evaluation.
type EvaluationInput struct {
	// Traits should be set to the external IDP-provided traits (for SSO users)
	// or the internal static traits (for local users) which will be input to
	// the login rule evaluation.
	Traits map[string][]string
}

// EvaluationOutput holds the output of a login rule evaluation.
type EvaluationOutput struct {
	// Traits holds the final output traits.
	Traits map[string][]string
	// AppliedRules holds a list of names of rules that were applied.
	AppliedRules []string
}

// Evaluator can be used to evaluate login rules currently present in the
// backend with given inputs.
type Evaluator interface {
	Evaluate(context.Context, *EvaluationInput) (*EvaluationOutput, error)
}

// NullEvaluator is an Evaluator implementation which makes no changes to the
// input and is meant to be used when login rules are not enabled in the
// cluster.
type NullEvaluator struct{}

// Evaluate returns the input traits unmodified.
func (NullEvaluator) Evaluate(ctx context.Context, input *EvaluationInput) (*EvaluationOutput, error) {
	return &EvaluationOutput{
		Traits: input.Traits,
	}, nil
}
