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
	"testing"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
	"github.com/stretchr/testify/require"
)

func TestEvaluate(t *testing.T) {
	// True result.
	result, err := expression.Evaluate(
		`workload.podman.attested && workload.podman.container.image == "ubuntu"`,
		&workloadidentityv1.Attrs{
			Workload: &workloadidentityv1.WorkloadAttrs{
				Podman: &workloadidentityv1.WorkloadAttrsPodman{
					Attested: true,
					Container: &workloadidentityv1.WorkloadAttrsPodmanContainer{
						Image: "ubuntu",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	// False result.
	result, err = expression.Evaluate(
		`user.name != user.name`,
		&workloadidentityv1.Attrs{
			User: &workloadidentityv1.UserAttrs{
				Name: "Bobby",
			},
		},
	)
	require.NoError(t, err)
	require.False(t, result)

	// Nested nil messages.
	result, err = expression.Evaluate(
		`workload.podman.pod.labels["foo"] == "bar"`,
		&workloadidentityv1.Attrs{},
	)
	require.NoError(t, err)
	require.False(t, result)

	// Non-string expression.
	_, err = expression.Evaluate(`"chunky bacon"`, &workloadidentityv1.Attrs{})
	require.ErrorContains(t, err, "expression evaluated to string instead of boolean")
}
