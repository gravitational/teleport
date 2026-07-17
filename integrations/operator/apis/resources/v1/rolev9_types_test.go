/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestRoleV9UnknownAppResourceFieldRejected locks in the reconciler-side
// half of the fail-closed behavior for app_resources rules. A rule may
// contain only allow_all, so the strict unstructured conversion must
// reject an unknown field rather than drop it and accept a rule the user
// never wrote.
func TestRoleV9UnknownAppResourceFieldRejected(t *testing.T) {
	u := map[string]any{
		"apiVersion": "resources.teleport.dev/v1",
		"kind":       "TeleportRoleV9",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"allow": map[string]any{
				"app_resources": []any{
					map[string]any{"allow_all": true, "paths": []any{"/x"}},
				},
			},
		},
	}
	obj := &TeleportRoleV9{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(u, obj, true /* returnUnknownFields */)
	require.ErrorContains(t, err, `unknown field "spec.allow.app_resources[0].paths"`)
}
