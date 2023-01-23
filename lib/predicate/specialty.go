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

package predicate

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/predicate/partial"
	"github.com/gravitational/trace"
)

const attemptTimeout = time.Second

// ComputeValidNodeLogins computes a list of valid logins for a given node.
func (c *PredicateAccessChecker) ComputeValidNodeLogins(ctx context.Context, node *Node, limit int) ([]string, error) {
	var constraints []partial.Constraint

	for _, policy := range c.Policies {
		if expr, ok := policy.GetAllow()[AccessNodeField]; ok {
			constraints = append(constraints, partial.Constraint{
				Allow: true,
				Expr:  expr,
			})
		}

		if expr, ok := policy.GetDeny()[AccessNodeField]; ok {
			constraints = append(constraints, partial.Constraint{
				Allow: false,
				Expr:  expr,
			})
		}
	}

	solver := partial.GetCachedSolver()
	defer partial.PutCachedSolver(solver)
	runCtx, cancelRunCtx := context.WithTimeout(ctx, attemptTimeout)
	defer cancelRunCtx()

	z3v, err := solver.PartialSolveForAll(runCtx, constraints, nil, AccessNodeLoginField, partial.TypeString, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	valid := make([]string, len(z3v))
	for i, v := range z3v {
		s := v.String()
		valid[i] = s[1 : len(s)-1]
	}

	return valid, nil
}
