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

// TestGetPodForEphemeralPatch_UsesImpersonatedIdentity asserts that the pod GET issued by
// the moderated ephemeral-container flow carries the user's impersonation headers,
// so that the Kubernetes API server enforces RBAC on the requester's mapped kubernetes_users/kubernetes_groups
// rather than on the proxy's admin kubeconfig.
// The synthesised "patched" pod returned to the user therefore cannot contain spec, env,
// or annotations that the user's mapped identity is not authorised to read.
//
// The test stands up a fake Kubernetes API server,
// drives getPodForEphemeralPatch with an authContext mapped to a single kubernetes_users entry,
// and checks that the upstream GET arrived with the matching Impersonate-User header.
func TestGetPodForEphemeralPatch_UsesImpersonatedIdentity(t *testing.T) {
	const (
		clusterName = "test-cluster"
		ns          = "prod"
		podName     = "secretive-app"
		kubeUser    = "victim-k8s"
	)

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

	// One kubernetes_users entry so computeAndValidateImpersonatedPrincipals
	// picks it deterministically without consulting request headers.
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

	require.Equal(t, kubeUser, got, "upstream GET must carry the user's Impersonate-User header")
}
