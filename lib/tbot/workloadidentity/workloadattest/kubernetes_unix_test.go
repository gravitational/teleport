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
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestKubernetesAttestor_Attest(t *testing.T) {
	t.Parallel()

	mockToken := "FOOBARBUZZ"
	mockPID := 1234
	// Values from k8s-real-gcp-v1.29.5-gke.1091002
	mockPodID := "61c266b0-6f75-4490-8d92-3c9ae4d02787"
	mockContainerID := "9da25af0b548c8c60aa60f77f299ba727bf72d58248bd7528eb5390ffcce555a"

	restartPolicy := func(s v1.ContainerRestartPolicy) *v1.ContainerRestartPolicy {
		return &s
	}

	testCases := map[string]struct {
		containerStatusFunc     func(callCount int) []v1.ContainerStatus
		initContainerStatusFunc func(callCount int) []v1.ContainerStatus
		initContainerSpecsFunc  func(callCount int) []v1.Container

		wantRequests int
		wantType     workloadidentityv1pb.WorkloadAttrsKubernetesContainer_Type
		wantError    string
	}{
		"regular container": {
			wantRequests: 2,
			wantType:     workloadidentityv1pb.WorkloadAttrsKubernetesContainer_TYPE_REGULAR,
			containerStatusFunc: func(callCount int) []v1.ContainerStatus {
				// Don't return the container status in the first response, to simulate
				// the kubelet API's eventual consistency.
				var containerStatuses []v1.ContainerStatus
				switch {
				case callCount == 1:
					containerStatuses = append(containerStatuses, v1.ContainerStatus{
						ContainerID: "docker://totally-wrong-container-id",
					})
				case callCount > 1:
					containerStatuses = append(containerStatuses, v1.ContainerStatus{
						ContainerID: "docker://" + mockContainerID,
						Name:        "container-1",
						Image:       "my.registry.io/my-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-app@sha256:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					})
				}

				return containerStatuses
			},

			initContainerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://1231455ksasdffgdk923klsklsdghkld0235wehlsdgjsd3i50sekfgort0235ui",
						Name:        "another-container-1",
						Image:       "my.registry.io/my-other-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-other-app:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerSpecsFunc: func(_ int) []v1.Container {
				return []v1.Container{
					{
						Name:          "another-container-1",
						RestartPolicy: restartPolicy(v1.ContainerRestartPolicyAlways),
					},
				}
			},
		},

		"init container: native sidecar": {
			wantRequests: 1,
			wantType:     workloadidentityv1pb.WorkloadAttrsKubernetesContainer_TYPE_INIT,
			containerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://1231455ksasdffgdk923klsklsdghkld0235wehlsdgjsd3i50sekfgort0235ui",
						Name:        "another-container-1",
						Image:       "my.registry.io/my-other-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-other-app:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://" + mockContainerID,
						Name:        "container-1",
						Image:       "my.registry.io/my-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-app@sha256:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerSpecsFunc: func(_ int) []v1.Container {
				return []v1.Container{
					{
						Name:          "container-1",
						RestartPolicy: restartPolicy(v1.ContainerRestartPolicyAlways),
					},
				}
			},
		},

		"init container: not a native sidecar": {
			wantRequests: 1,
			wantType:     workloadidentityv1pb.WorkloadAttrsKubernetesContainer_TYPE_INIT,
			wantError:    `"container-1" is not a native sidecar`,
			containerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://1231455ksasdffgdk923klsklsdghkld0235wehlsdgjsd3i50sekfgort0235ui",
						Name:        "another-container-1",
						Image:       "my.registry.io/my-other-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-other-app:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://" + mockContainerID,
						Name:        "container-1",
						Image:       "my.registry.io/my-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-app@sha256:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerSpecsFunc: func(_ int) []v1.Container {
				return []v1.Container{
					{
						Name:          "container-1",
						RestartPolicy: restartPolicy(v1.ContainerRestartPolicyOnFailure),
					},
				}
			},
		},

		"init container: spec not found": {
			wantRequests: 1,
			wantType:     workloadidentityv1pb.WorkloadAttrsKubernetesContainer_TYPE_INIT,
			wantError:    `"container-1" is not a native sidecar`,
			containerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://1231455ksasdffgdk923klsklsdghkld0235wehlsdgjsd3i50sekfgort0235ui",
						Name:        "another-container-1",
						Image:       "my.registry.io/my-other-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-other-app:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerStatusFunc: func(_ int) []v1.ContainerStatus {
				return []v1.ContainerStatus{
					{
						ContainerID: "docker://" + mockContainerID,
						Name:        "container-1",
						Image:       "my.registry.io/my-app:v1",
						ImageID:     "docker-pullable://my.registry.io/my-app@sha256:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					},
				}
			},

			initContainerSpecsFunc: func(_ int) []v1.Container {
				return []v1.Container{
					{
						Name:          "container-2",
						RestartPolicy: restartPolicy(v1.ContainerRestartPolicyAlways),
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			log := logtest.NewLogger()

			// Setup mock Kubelet Secure API
			var requests atomic.Int32
			mockKubeletAPI := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/pods" {
					http.NotFound(w, req)
					return
				}

				requests.Add(1)
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
								InitContainers:     tc.initContainerSpecsFunc(int(requests.Load())),
							},
							Status: v1.PodStatus{
								ContainerStatuses:     tc.containerStatusFunc(int(requests.Load())),
								InitContainerStatuses: tc.initContainerStatusFunc(int(requests.Load())),
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
			require.NoError(t, os.WriteFile(tokenPath, []byte(mockToken), 0o644))
			procPath := filepath.Join(tmpDir, "proc")
			procPIDPath := filepath.Join(procPath, strconv.Itoa(mockPID))
			pidMountInfoPath := filepath.Join(procPIDPath, "mountinfo")
			require.NoError(t, os.MkdirAll(procPIDPath, 0o755))
			require.NoError(t, utils.CopyFile(
				filepath.Join("container", "testdata", "mountfile", "k8s-real-gcp-v1.29.5-gke.1091002"),
				pidMountInfoPath,
				0o755),
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
			attestor.clock = clockwork.NewRealClock()
			attestor.kubeletClient.getEnv = func(s string) string {
				env := map[string]string{
					"TELEPORT_NODE_NAME": host,
				}
				return env[s]
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			att, err := attestor.Attest(ctx, mockPID)
			if tc.wantError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.wantError)
				return
			}

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(workloadidentityv1pb.WorkloadAttrsKubernetes_builder{
				Attested:       true,
				ServiceAccount: "my-service-account",
				Namespace:      "default",
				PodName:        "my-pod",
				PodUid:         mockPodID,
				Labels: map[string]string{
					"my-label": "my-label-value",
				},
				Container: workloadidentityv1pb.WorkloadAttrsKubernetesContainer_builder{
					Name:        "container-1",
					Image:       "my.registry.io/my-app:v1",
					ImageDigest: "sha256:84c998f7610b356a5eed24f801c01b273cf3e83f081f25c9b16aa8136c2cafb1",
					Type:        tc.wantType,
				}.Build(),
			}.Build(), att, protocmp.Transform()))
			require.EqualValues(t, tc.wantRequests, requests.Load())
			require.NoError(t, ctx.Err())
		})
	}
}
