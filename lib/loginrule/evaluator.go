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

package loginrule

import "context"

// EvaluationInput holds the inputs to a login rule evaluation.
type EvaluationInput struct {
	// Traits should be set to the external IDP-provided traits (for SSO users)
	// or the internal static traits (for local users) which will be input to
	// the login rule evaluation.
	Traits map[string][]string
	// Claims holds the original, unparsed provider claims. Each claim may be
	// a standard string/list, or an arbitrary json object.
	Claims map[string]any
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
