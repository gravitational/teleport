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
package attrs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/attrs"
)

func TestWorkloadAttrs(t *testing.T) {
	attrs := attrs.FromWorkloadAttrs(&workloadidentityv1.WorkloadAttrs{
		Podman: &workloadidentityv1.WorkloadAttrsPodman{
			Attested: true,
		},
		Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
			Payloads: []*workloadidentityv1.SigstoreVerificationPayload{
				{Bundle: []byte(`BUNDLE`)},
				{Bundle: []byte(`BUNDLE`)},
			},
		},
	})

	output := attrs.LogValue().String()
	require.Contains(t, output, "sigstore:{payloads:{count:2}}")
	require.NotContains(t, output, "BUNDLE")
	require.NotNil(t, attrs.Sigstore)
}
