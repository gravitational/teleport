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

package reconcilers

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
)

// TestUpsertRejectsUnknownAppResourceField feeds the generic upsert an
// unstructured role whose app_resources rule carries an unknown field beside
// allow_all. A rule may contain only allow_all, so the upsert must decode
// with unknown-field validation.
func TestUpsertRejectsUnknownAppResourceField(t *testing.T) {
	const (
		name      = "test-role"
		namespace = "default"
	)
	cr := &resourcesv1.TeleportRoleV9{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resourcesv1.GroupVersion.String(),
			Kind:       "TeleportRoleV9",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	kubeClient := fake.NewClientBuilder().
		WithScheme(controllers.Scheme).
		WithStatusSubresource(&resourcesv1.TeleportRoleV9{}).
		WithObjects(cr).
		Build()
	reconciler := resourceReconciler[types.Role, *resourcesv1.TeleportRoleV9]{
		kubeClient: kubeClient,
	}

	role := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": resourcesv1.GroupVersion.String(),
		"kind":       "TeleportRoleV9",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec": map[string]any{
			"allow": map[string]any{
				"app_resources": []any{
					map[string]any{"allow_all": true, "paths": []any{"/x"}},
				},
			},
		},
	}}

	err := reconciler.Upsert(t.Context(), role)
	require.ErrorContains(t, err, `unknown field "spec.allow.app_resources[0].paths"`)
}
