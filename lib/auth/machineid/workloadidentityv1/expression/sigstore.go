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

package expression

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// SigstorePolicyEvaluator implements Sigstore policy evaluation.
type SigstorePolicyEvaluator interface {
	// PolicySatisfied determines whether the named Sigstore policies are all
	// satisfied by the signatures and attestations presented in the workload
	// attributes.
	PolicySatisfied(names []string) (bool, error)
}

// SigstorePolicyEvaluatorFunc wraps a function to implement SigstorePolicyEvaluator.
type SigstorePolicyEvaluatorFunc func([]string) (bool, error)

// PolicySatisfied implements SigstorePolicyEvaluator
func (fn SigstorePolicyEvaluatorFunc) PolicySatisfied(names []string) (bool, error) {
	return fn(names)
}

var _ SigstorePolicyEvaluator = (SigstorePolicyEvaluatorFunc)(nil)

const funcNameSigstorePolicySatisfied = "sigstore.policy_satisfied"

var funcSigstorePolicySatisfied = typical.UnaryVariadicFunctionWithEnv(func(env *Environment, names ...string) (bool, error) {
	if env.SigstorePolicyEvaluator == nil {
		return false, trace.BadParameter("no SigstorePolicyEvaluator was provided")
	}
	return env.SigstorePolicyEvaluator.PolicySatisfied(names)
})
