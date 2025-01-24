//go:build unix

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package workloadattest

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestKubernetesAttestor_getContainerAndPodID(t *testing.T) {
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		name            string
		wantPodID       string
		wantContainerID string
	}{
		{
			name:            "k8s-real-docker-desktop",
			wantPodID:       "941f292f-a62d-48ab-b9a8-eec84d87b928",
			wantContainerID: "3f79e718744418736d0f6b9958e08d44e969c6577068c33de1cc400d35aacec8",
		},
		{
			name:            "k8s-real-orbstack",
			wantPodID:       "36827f77-691f-45aa-a470-0989cf3749c4",
			wantContainerID: "64dd9bf5199ff782835247cb072e4842dc3d0135ef02f6498cb6bb6f37a320d2",
		},
		{
			name:            "k8s-real-k3s-ubuntu-v1.28.6+k3s2",
			wantPodID:       "fecd2321-17b5-49b9-9f75-8c5be777fbfb",
			wantContainerID: "397529d07efebd566f15dbc7e8af9f3ef586033f5e753adfa96b2bf730102c64",
		},
		{
			name:            "k8s-real-gcp-v1.29.5-gke.1091002",
			wantPodID:       "61c266b0-6f75-4490-8d92-3c9ae4d02787",
			wantContainerID: "9da25af0b548c8c60aa60f77f299ba727bf72d58248bd7528eb5390ffcce555a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "proc", "1234"), 0755))
			require.NoError(t, utils.CopyFile(
				filepath.Join("testdata", "mountfile", tt.name),
				filepath.Join(tempDir, "proc", "1234", "mountinfo"),
				0755),
			)
			attestor := &KubernetesAttestor{
				rootPath: tempDir,
				log:      log,
			}
			gotPodID, gotContainerID, err := attestor.getContainerAndPodID(1234)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPodID, gotPodID)
			assert.Equal(t, tt.wantContainerID, gotContainerID)
		})
	}
}

func TestKubernetesAttestor_Attest(t *testing.T) {
	t.Parallel()
	log := utils.NewSlogLoggerForTests()
	ctx := context.Background()

	mockToken := "FOOBARBUZZ"
	mockPID := 1234
	// Value from k8s-real-gcp-v1.29.5-gke.1091002
	mockPodID := "61c266b0-6f75-4490-8d92-3c9ae4d02787"

	// Setup mock Kubelet Secure API
	mockKubeletAPI := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/pods" {
			http.NotFound(w, req)
			return
		}
		out := v1.PodList{
			Items: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pod",
						Namespace: "default",
						UID:       types.UID(mockPodID),
						Labels: map[string]string{
							"my-label": "my-label-value",
						},
					},
					Spec: v1.PodSpec{
						ServiceAccountName: "my-service-account",
					},
				},
			},
		}
		w.WriteHeader(200)
		assert.NoError(t, json.NewEncoder(w).Encode(out))
	}))
	t.Cleanup(mockKubeletAPI.Close)
	kubeletAddr := mockKubeletAPI.Listener.Addr().String()
	host, port, err := net.SplitHostPort(kubeletAddr)
	require.NoError(t, err)
	portInt, err := strconv.Atoi(port)
	require.NoError(t, err)

	// Setup mock filesystem
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte(mockToken), 0644))
	procPath := filepath.Join(tmpDir, "proc")
	procPIDPath := filepath.Join(procPath, strconv.Itoa(mockPID))
	pidMountInfoPath := filepath.Join(procPIDPath, "mountinfo")
	require.NoError(t, os.MkdirAll(procPIDPath, 0755))
	require.NoError(t, utils.CopyFile(
		filepath.Join("testdata", "mountfile", "k8s-real-gcp-v1.29.5-gke.1091002"),
		pidMountInfoPath,
		0755),
	)

	// Setup Attestor for mocks
	attestor := NewKubernetesAttestor(KubernetesAttestorConfig{
		Enabled: true,
		Kubelet: KubeletClientConfig{
			TokenPath:  tokenPath,
			SkipVerify: true,
			SecurePort: portInt,
		},
	}, log)
	attestor.rootPath = tmpDir
	attestor.kubeletClient.getEnv = func(s string) string {
		env := map[string]string{
			"TELEPORT_NODE_NAME": host,
		}
		return env[s]
	}

	att, err := attestor.Attest(ctx, mockPID)
	assert.NoError(t, err)
	assert.Empty(t, cmp.Diff(&workloadidentityv1pb.WorkloadAttrsKubernetes{
		Attested:       true,
		ServiceAccount: "my-service-account",
		Namespace:      "default",
		PodName:        "my-pod",
		PodUid:         mockPodID,
		Labels: map[string]string{
			"my-label": "my-label-value",
		},
	}, att, protocmp.Transform()))
}
