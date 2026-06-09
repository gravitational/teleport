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

package expression_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	traitv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
)

func TestEvaluate(t *testing.T) {
	// True result.
	result, err := expression.Evaluate(
		`workload.podman.attested && workload.podman.container.image == "ubuntu"`,
		&expression.Environment{
			Attrs: workloadidentityv1.Attrs_builder{
				Workload: workloadidentityv1.WorkloadAttrs_builder{
					Podman: workloadidentityv1.WorkloadAttrsPodman_builder{
						Attested: true,
						Container: workloadidentityv1.WorkloadAttrsPodmanContainer_builder{
							Image: "ubuntu",
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
		},
	)
	require.NoError(t, err)
	require.True(t, result)

	// False result.
	result, err = expression.Evaluate(
		`user.name != user.name`,
		&expression.Environment{
			Attrs: workloadidentityv1.Attrs_builder{
				User: workloadidentityv1.UserAttrs_builder{
					Name: "Bobby",
				}.Build(),
			}.Build(),
		},
	)
	require.NoError(t, err)
	require.False(t, result)

	// Unset field (allowed in boolean expressions).
	result, err = expression.Evaluate(
		`user.name == ""`,
		&expression.Environment{
			Attrs: workloadidentityv1.Attrs_builder{
				User: workloadidentityv1.UserAttrs_builder{
					Name: "",
				}.Build(),
			}.Build(),
		},
	)
	require.NoError(t, err)
	require.True(t, result)
}

func TestEvaluate_Errors(t *testing.T) {
	testCases := map[string]struct {
		expr  string
		attrs *workloadidentityv1.Attrs
		err   string
	}{
		"unset sub-message": {
			expr: `workload.podman.pod.labels["foo"] == "bar"`,
			attrs: workloadidentityv1.Attrs_builder{
				Workload: workloadidentityv1.WorkloadAttrs_builder{
					Podman: workloadidentityv1.WorkloadAttrsPodman_builder{
						Pod: nil,
					}.Build(),
				}.Build(),
			}.Build(),
			err: "workload.podman.pod is unset",
		},
		"unset map key": {
			expr: `workload.podman.pod.labels["foo"] == "bar"`,
			attrs: workloadidentityv1.Attrs_builder{
				Workload: workloadidentityv1.WorkloadAttrs_builder{
					Podman: workloadidentityv1.WorkloadAttrsPodman_builder{
						Pod: workloadidentityv1.WorkloadAttrsPodmanPod_builder{
							Labels: map[string]string{"bar": "baz"},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			err: `no value for key: "foo"`,
		},
		"non-boolean expression": {
			expr:  `"chunky bacon"`,
			attrs: &workloadidentityv1.Attrs{},
			err:   "evaluated to string instead of boolean",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := expression.Evaluate(tc.expr, &expression.Environment{Attrs: tc.attrs})
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestEvaluate_Traits(t *testing.T) {
	result, err := expression.Evaluate(
		`user.traits.logins.contains("root")`,
		&expression.Environment{
			Attrs: workloadidentityv1.Attrs_builder{
				User: workloadidentityv1.UserAttrs_builder{
					Traits: []*traitv1.Trait{
						traitv1.Trait_builder{
							Key:    "logins",
							Values: []string{"root", "alice", "bob"},
						}.Build(),
					},
				}.Build(),
			}.Build(),
		},
	)
	require.NoError(t, err)
	require.True(t, result)

	// Unset trait.
	result, err = expression.Evaluate(
		`is_empty(user.traits.logins)`,
		&expression.Environment{
			Attrs: workloadidentityv1.Attrs_builder{
				User: workloadidentityv1.UserAttrs_builder{
					Traits: []*traitv1.Trait{},
				}.Build(),
			}.Build(),
		},
	)
	require.NoError(t, err)
	require.True(t, result)
}

func TestEvaluate_SigstorePolicies(t *testing.T) {
	results := map[string]error{
		"github-provenance":      nil,
		"security-team-approval": errors.New("missing artifact signature"),
	}
	env := &expression.Environment{
		SigstorePolicyEvaluator: expression.SigstorePolicyEvaluatorFunc(func(names []string) (bool, error) {
			for _, name := range names {
				err, ok := results[name]
				if !ok {
					return false, fmt.Errorf("unknown policy: %q", name)
				}
				if err != nil {
					return false, nil
				}
			}
			return true, nil
		}),
	}

	result, err := expression.Evaluate(
		`sigstore.policy_satisfied("github-provenance")`,
		env,
	)
	require.NoError(t, err)
	require.True(t, result)

	result, err = expression.Evaluate(
		`sigstore.policy_satisfied("security-team-approval")`,
		env,
	)
	require.NoError(t, err)
	require.False(t, result)

	_, err = expression.Evaluate(
		`sigstore.policy_satisfied("does-not-exist")`,
		env,
	)
	require.ErrorContains(t, err, `unknown policy: "does-not-exist"`)
}
