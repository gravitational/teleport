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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/authz"
)

// TestGetPodForEphemeralPatch_UsesImpersonatedIdentity asserts that the pod GET
// issued by the moderated ephemeral-container flow carries the user's Impersonate-User header,
// so the Kubernetes API server evaluates the request against the requester's
// mapped kubernetes_users / kubernetes_groups rather than the proxy's admin kubeconfig.
func TestGetPodForEphemeralPatch_UsesImpersonatedIdentity(t *testing.T) {
	const (
		clusterName = "test-cluster"
		ns          = "default"
		podName     = "test-app"
		kubeUser    = "alice-k8s"
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
		log: logrus.NewEntry(logrus.New()),
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

	teleportUser, err := types.NewUser("alice")
	require.NoError(t, err)

	// One kubernetes_users entry so computeImpersonatedPrincipals
	// picks it deterministically without consulting request headers.
	authCtx := &authContext{
		Context:         authz.Context{User: teleportUser},
		kubeClusterName: clusterName,
		kubeUsers:       map[string]struct{}{kubeUser: {}},
		kubeGroups:      map[string]struct{}{},
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

// TestGetPatchedPodEvent_ReplaysStoredImpersonation asserts that
// the moderated watch path uses the kubernetes_user and kubernetes_groups
// stored on the KubernetesWaitingContainer when fetching the target pod,
// instead of relying on the original request headers (which are not in scope on the watch path).
// A user with multiple kubernetes_users values therefore still receives
// the synthetic Modified event after moderator approval, rather than hanging.
func TestGetPatchedPodEvent_ReplaysStoredImpersonation(t *testing.T) {
	const (
		clusterName  = "test-cluster"
		ns           = "default"
		podName      = "test-app"
		chosenUser   = "alice-k8s-b"
		alternateUsr = "alice-k8s-a"
		chosenGroup  = "system:authenticated"
	)

	var (
		mu             sync.Mutex
		capturedUser   string
		capturedGroups []string
	)
	fakePod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "registry.k8s.io/pause:3.10"}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pods/"+podName) {
			mu.Lock()
			capturedUser = r.Header.Get("Impersonate-User")
			capturedGroups = append([]string(nil), r.Header.Values("Impersonate-Group")...)
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
		log: logrus.NewEntry(logrus.New()),
		cfg: ForwarderConfig{tracer: otel.Tracer("test")},
		clusterDetails: map[string]*kubeDetails{
			clusterName: {
				kubeCreds: &staticKubeCreds{
					kubeClient:    adminClient,
					clientRestCfg: adminRestCfg,
				},
			},
		},
	}

	teleportUser, err := types.NewUser("alice")
	require.NoError(t, err)

	// Multi-user setup: without the stored choice on the waiting container,
	// computeImpersonatedPrincipals would refuse to pick.
	sess := &clusterSession{
		authContext: authContext{
			Context:         authz.Context{User: teleportUser},
			kubeClusterName: clusterName,
			kubeUsers:       map[string]struct{}{chosenUser: {}, alternateUsr: {}},
			kubeGroups:      map[string]struct{}{chosenGroup: {}},
		},
		codecFactory: &globalKubeCodecs,
	}

	patch := []byte(`{"spec":{"ephemeralContainers":[{"name":"debug","image":"busybox","tty":true}]}}`)
	waitingCont, err := kubewaitingcontainer.NewKubeWaitingContainer(
		"debug",
		&kubewaitingcontainerpb.KubernetesWaitingContainerSpec{
			Username:         "alice",
			Cluster:          clusterName,
			Namespace:        ns,
			PodName:          podName,
			ContainerName:    "debug",
			Patch:            patch,
			PatchType:        "application/strategic-merge-patch+json",
			KubernetesUser:   chosenUser,
			KubernetesGroups: []string{chosenGroup},
		},
	)
	require.NoError(t, err)

	_, err = fwd.getPatchedPodEvent(context.Background(), sess, waitingCont)
	require.NoError(t, err)
	require.Equal(t, chosenUser, capturedUser, "upstream GET must replay the stored Impersonate-User")
	require.Equal(t, []string{chosenGroup}, capturedGroups, "upstream GET must replay the stored Impersonate-Group")
}
