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
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	tkm "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

// TestListResourcesAcceptEncodingIdentity verifies that when RBAC filtering
// is needed, the upstream Kubernetes API request receives Accept-Encoding: identity
// so the response comes back uncompressed and avoids the decompress/recompress cycle.
func TestListResourcesAcceptEncodingIdentity(t *testing.T) {
	t.Parallel()

	const usernameFiltered = "filtered_user"

	// Track Accept-Encoding headers received by the upstream mock K8s API
	// for pod list requests only.
	var mu sync.Mutex
	var capturedAcceptEncodings []string

	kubeMock, err := tkm.NewKubeAPIMock(
		tkm.WithRequestCallback(func(r *http.Request) {
			// Only capture pod list requests, not discovery/version/etc.
			if !strings.Contains(r.URL.Path, "/pods") {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			capturedAcceptEncodings = append(capturedAcceptEncodings, r.Header.Get("Accept-Encoding"))
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	testCtx := SetupTestContext(
		t.Context(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// Simulate in-cluster mode so the optimization fires.
	testCtx.KubeServer.fwd.cfg.KubeconfigPath = ""

	// User with namespace-scoped access triggers RBAC filtering.
	userFiltered, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameFiltered,
		RoleSpec{
			Name:       usernameFiltered,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow, []types.KubernetesResource{
					{
						Kind:      "pods",
						Name:      "nginx-*",
						Namespace: metav1.NamespaceDefault,
						Verbs:     []string{types.Wildcard},
						APIGroup:  types.Wildcard,
					},
				})
			},
		},
	)

	_, restConfig := testCtx.GenTestKubeClientTLSCert(
		t,
		userFiltered.GetName(),
		kubeCluster,
	)

	// Disable transport-level gzip so the test client sends requests
	// without Accept-Encoding and receives uncompressed responses.
	// Recompression for gzip-requesting clients is tested separately
	// in TestCompressMemBuffer.
	restConfig.DisableCompression = true
	client, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	_, err = client.CoreV1().Pods(metav1.NamespaceDefault).List(
		testCtx.Context,
		metav1.ListOptions{},
	)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, capturedAcceptEncodings, "expected at least one pod list request to upstream")
	for _, ae := range capturedAcceptEncodings {
		assert.Equal(t, "identity", ae,
			"upstream request should have Accept-Encoding: identity when RBAC filtering is needed")
	}
}

func TestHeaderAcceptsEncoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		header   string
		encoding string
		want     bool
	}{
		{name: "plain gzip", header: "gzip", encoding: "gzip", want: true},
		{name: "gzip with deflate", header: "deflate, gzip", encoding: "gzip", want: true},
		{name: "quality 1", header: "gzip;q=1", encoding: "gzip", want: true},
		{name: "quality 0.5", header: "gzip;q=0.5", encoding: "gzip", want: true},
		{name: "quality 0 rejects", header: "gzip;q=0", encoding: "gzip", want: false},
		{name: "quality 0.0 rejects", header: "gzip;q=0.0", encoding: "gzip", want: false},
		{name: "empty header", header: "", encoding: "gzip", want: false},
		{name: "no gzip", header: "deflate, br", encoding: "gzip", want: false},
		{name: "spaces around", header: " gzip ; q=1 ", encoding: "gzip", want: true},
		{name: "uppercase token GZIP", header: "GZIP", encoding: "gzip", want: true},
		{name: "mixed case token Gzip", header: "Gzip", encoding: "gzip", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			if tt.header != "" {
				h.Set("Accept-Encoding", tt.header)
			}
			assert.Equal(t, tt.want, headerAcceptsEncoding(h, tt.encoding))
		})
	}

	// Verify that random capitalizations of the header key work.
	for _, key := range []string{
		"accept-encoding",
		"ACCEPT-ENCODING",
		"aCcEpT-eNcOdInG",
	} {
		t.Run("key_"+key, func(t *testing.T) {
			h := http.Header{}
			h.Set(key, "gzip")
			assert.True(t, headerAcceptsEncoding(h, "gzip"))
		})
	}
}

// TestCompressMemBuffer verifies that compressMemBuffer gzip-compresses the
// buffer contents and sets Content-Encoding: gzip.
func TestCompressMemBuffer(t *testing.T) {
	t.Parallel()

	original := []byte(`{"kind":"PodList","apiVersion":"v1","items":[]}`)
	mem := responsewriters.NewMemoryResponseWriter()
	mem.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)
	_, err := mem.Write(original)
	require.NoError(t, err)

	require.NoError(t, compressMemBuffer(mem))
	assert.Equal(t, "gzip", mem.Header().Get(contentEncodingHeader))

	// Decompress and verify round-trip.
	gr, err := gzip.NewReader(mem.Buffer())
	require.NoError(t, err)
	defer gr.Close()
	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
}
