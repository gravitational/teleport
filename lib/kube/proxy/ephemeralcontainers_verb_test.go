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

package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

// TestEphemeralContainersRequiredVerbs asserts the set of kubernetes_resources verbs that gate each pods request.
// Adding an ephemeral container (PATCH/PUT to pods/{name}/ephemeralcontainers) runs code in the target pod
// the same way exec does, and also mutates the pod,
// so it must require both the exec verb and the mutation verb the HTTP method maps to.
// A plain pod patch and pods/exec must each require only their own single verb,
// so that neither verb on its own is enough to add an ephemeral container.
func TestEphemeralContainersRequiredVerbs(t *testing.T) {
	// Minimal kubeDetails with just the "pods" resource registered for RBAC.
	// Mirrors what newClusterSchemaBuilder populates from real cluster
	// discovery for the core/v1 pods resource.
	kd := &kubeDetails{
		rbacSupportedTypes: rbacSupportedResources{
			allowedResourcesKey{apiGroup: "", resourceKind: "pods"}: metav1.APIResource{
				Name:       "pods",
				Namespaced: true,
				Kind:       "Pod",
			},
		},
	}

	verbsFor := func(t *testing.T, method, path string) []string {
		u, err := url.Parse(path)
		require.NoError(t, err)
		mr, err := getResourceFromRequest(&http.Request{Method: method, URL: u}, kd)
		require.NoError(t, err)

		var verbs []string
		for _, r := range mr.requiredRBACResources() {
			verbs = append(verbs, r.Verbs...)
		}
		return verbs
	}

	// Control: pods/exec requires only the exec verb.
	t.Run("pods/exec requires only exec", func(t *testing.T) {
		verbs := verbsFor(t, http.MethodPost, "/api/v1/namespaces/default/pods/foo/exec")
		require.Equal(t, []string{types.KubeVerbExec}, verbs)
	})

	// Control: a patch to the pod itself (e.g. a label update) requires only
	// the patch verb. It must not pull in exec, otherwise a label patch and an
	// ephemeral container would be indistinguishable in RBAC.
	t.Run("pod patch requires only patch", func(t *testing.T) {
		verbs := verbsFor(t, http.MethodPatch, "/api/v1/namespaces/default/pods/foo")
		require.Equal(t, []string{types.KubeVerbPatch}, verbs)
		require.NotContains(t, verbs, types.KubeVerbExec)
	})

	t.Run("ephemeralcontainers patch requires exec and patch", func(t *testing.T) {
		verbs := verbsFor(t, http.MethodPatch,
			"/api/v1/namespaces/default/pods/foo/ephemeralcontainers")
		require.Contains(t, verbs, types.KubeVerbExec,
			"adding an ephemeral container must require the exec verb")
		require.Contains(t, verbs, types.KubeVerbPatch,
			"adding an ephemeral container must require the mutation verb")
	})

	t.Run("ephemeralcontainers put requires exec and update", func(t *testing.T) {
		verbs := verbsFor(t, http.MethodPut,
			"/api/v1/namespaces/default/pods/foo/ephemeralcontainers")
		require.Contains(t, verbs, types.KubeVerbExec,
			"adding an ephemeral container must require the exec verb")
		require.Contains(t, verbs, types.KubeVerbUpdate,
			"adding an ephemeral container must require the mutation verb")
	})
}
