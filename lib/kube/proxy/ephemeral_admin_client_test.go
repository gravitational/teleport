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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/set"
)

// TestGetPodForEphemeralPatch_UsesImpersonatedIdentity is a regression test
// for the bug where the moderated ephemeral-container flow at
// `lib/kube/proxy/ephemeral_containers.go` fetched the target pod with the
// proxy's static admin kubeconfig (`details.getKubeClient()`) instead of an
// impersonated client carrying the user's `kubernetes_users` /
// `kubernetes_groups`. Because the synthesised pod is then encoded back to
// the caller, this bypassed Kubernetes RBAC on the user's mapped identity
// and leaked pod spec/env/annotations.
//
// The fix moves the pod GET into `getPodForEphemeralPatch`, which uses
// `impersonatedKubeClient` so the Kubernetes API server enforces RBAC on the
// user's mapped identity. This test stands up a fake Kubernetes API server,
// drives `getPodForEphemeralPatch` with an authContext mapped to a single
// `kubernetes_users` entry of `victim-k8s`, and asserts the upstream GET
// carried `Impersonate-User: victim-k8s`.
//
// Revert the fix (have `getPodForEphemeralPatch` call
// `findKubeDetailsByClusterName(...).getKubeClient()` instead of
// `impersonatedKubeClient`) and this test fails with an empty captured
// header value.
//
// See workspace/f3-ephemeral-admin-client.md for the full root-cause writeup
// and end-to-end PoC.
func TestGetPodForEphemeralPatch_UsesImpersonatedIdentity(t *testing.T) {
	const (
		clusterName = "test-cluster"
		ns          = "prod"
		podName     = "secretive-app"
		kubeUser    = "victim-k8s"
	)

	// Fake Kubernetes API server. We only need it to answer GET on the target
	// pod and capture whatever Impersonate-User header arrives on that GET.
	var (
		mu                  sync.Mutex
		capturedImpersonate string
	)
	fakePod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "registry.k8s.io/pause:3.10",
				Env: []corev1.EnvVar{
					{Name: "DB_PASSWORD", Value: "REDACTED-must-not-be-disclosed"},
				},
			}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pods/"+podName) {
			mu.Lock()
			capturedImpersonate = r.Header.Get("Impersonate-User")
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakePod)
	}))
	t.Cleanup(srv.Close)

	// Admin REST config and clientset, mirroring how a real kube agent wires
	// staticKubeCreds.kubeClient.
	adminRestCfg := &rest.Config{Host: srv.URL}
	adminClient, err := kubernetes.NewForConfig(adminRestCfg)
	require.NoError(t, err)

	fwd := &Forwarder{
		log: logtest.NewLogger(),
		cfg: ForwarderConfig{
			tracer: otel.Tracer("test"),
		},
		clusterDetails: map[string]*kubeDetails{
			clusterName: {
				kubeCreds: &staticKubeCreds{
					kubeClient:    adminClient,
					clientRestCfg: adminRestCfg,
				},
			},
		},
	}

	teleportUser, err := types.NewUser("victim")
	require.NoError(t, err)

	// authContext with exactly one kubernetes_users entry so
	// computeAndValidateImpersonatedPrincipals picks it deterministically.
	authCtx := &authContext{
		ScopedContext:   &authz.ScopedContext{User: teleportUser},
		kubeClusterName: clusterName,
		kubeUsers:       set.New(kubeUser),
		kubeGroups:      set.New[string](),
	}

	_, err = fwd.getPodForEphemeralPatch(
		context.Background(),
		authCtx,
		http.Header{},
		ns,
		podName,
	)
	require.NoError(t, err)

	mu.Lock()
	got := capturedImpersonate
	mu.Unlock()

	require.Equal(t, kubeUser, got,
		"GET to upstream Kubernetes during moderated ephemeral-container patch "+
			"must use the user's impersonated identity (kubernetes_users[0]), not the "+
			"proxy's admin kubeconfig. lib/kube/proxy/ephemeral_containers.go:204 currently "+
			"calls details.getKubeClient() which carries no Impersonate-User header. "+
			"Captured header value: %q", got)

	// Suppress "unused" warnings if a future refactor inlines kubeUser.
	_ = kubeUser
}
