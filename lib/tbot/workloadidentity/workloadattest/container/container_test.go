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

package container_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/container"
	"github.com/gravitational/teleport/lib/utils"
)

func TestLookupPID(t *testing.T) {
	tests := map[string]struct {
		parser   container.Parser
		expected *container.Info
		error    string
	}{
		"k8s-real-docker-desktop": {
			parser: container.KubernetesParser,
			expected: &container.Info{
				PodID:       "941f292f-a62d-48ab-b9a8-eec84d87b928",
				ID:          "3f79e718744418736d0f6b9958e08d44e969c6577068c33de1cc400d35aacec8",
				Rootfulness: container.RootfulnessUnknown,
			},
		},
		"k8s-real-orbstack": {
			parser: container.KubernetesParser,
			expected: &container.Info{
				PodID:       "36827f77-691f-45aa-a470-0989cf3749c4",
				ID:          "64dd9bf5199ff782835247cb072e4842dc3d0135ef02f6498cb6bb6f37a320d2",
				Rootfulness: container.RootfulnessUnknown,
			},
		},
		"k8s-real-k3s-ubuntu-v1.28.6+k3s2": {
			parser: container.KubernetesParser,
			expected: &container.Info{
				PodID:       "fecd2321-17b5-49b9-9f75-8c5be777fbfb",
				ID:          "397529d07efebd566f15dbc7e8af9f3ef586033f5e753adfa96b2bf730102c64",
				Rootfulness: container.RootfulnessUnknown,
			},
		},
		"k8s-real-gcp-v1.29.5-gke.1091002": {
			parser: container.KubernetesParser,
			expected: &container.Info{
				PodID:       "61c266b0-6f75-4490-8d92-3c9ae4d02787",
				ID:          "9da25af0b548c8c60aa60f77f299ba727bf72d58248bd7528eb5390ffcce555a",
				Rootfulness: container.RootfulnessUnknown,
			},
		},
		"podman-real-4.3.1-rootful-systemd-pod": {
			parser: container.PodmanParser,
			expected: &container.Info{
				PodID:       "88c57f699ea2c137d7f19b7a6aaa5828072cf12207b56d7155f02d4ecade4510",
				ID:          "4f6f96595778a052ebbd8e783156e347143cd79f81348d0995a0ffd5718c3393",
				Rootfulness: container.Rootful,
			},
		},
		"podman-real-4.3.1-rootful-systemd-container": {
			parser: container.PodmanParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "12519ca1a57b8f58bc2a44f4e33e37eaf07c55a8d468ffb3db33f29d8d869186",
				Rootfulness: container.Rootful,
			},
		},
		"podman-real-4.3.1-rootless-systemd-pod": {
			parser: container.PodmanParser,
			expected: &container.Info{
				PodID:       "5ffc3df0af9a6dd0f92668fc949734aad2ad41a5670b7218196d377d55ca32c5",
				ID:          "d54768c18894b931db6f6876f6be2178d8a8b34fc3485659fda78fe86af3e08b",
				Rootfulness: container.Rootless,
			},
		},
		"podman-real-4.3.1-rootless-systemd-container": {
			parser: container.PodmanParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "f89494c4c00e68029e176eb60c5be675f9b076b9ca63190678b27a2ef0d09d13",
				Rootfulness: container.Rootless,
			},
		},
		"podman-real-4.3.1-rootful-cgroupfs-container": {
			parser: container.PodmanParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "1861a57278895fe0165c953c04e6c1082bcd73428776f5209616061d0022e881",
				Rootfulness: container.Rootful,
			},
		},
		"podman-real-4.3.1-rootless-cgroupfs-systemd-enabled-container": {
			parser: container.PodmanParser,
			error:  "--cgroup-manager cgroupfs",
		},
		"docker-real-27.5.1-rootful-systemd": {
			parser: container.DockerParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "9125fbc01fb958c33eb2fda134db64e2c01ec456181fb5def541d6485ea810ba",
				Rootfulness: container.Rootful,
			},
		},
		"docker-real-27.5.1-rootful-cgroupfs": {
			parser: container.DockerParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "51509b0e049a2251892b0825bd393e1cffb9320ca96325bb372086dc97f30774",
				Rootfulness: container.Rootful,
			},
		},
		"docker-real-27.5.1-rootless-systemd": {
			parser: container.DockerParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "6ed20f39d4e73785851dfefef19762dbdca0a7d797b780b28ecd857fa4b29a45",
				Rootfulness: container.Rootless,
			},
		},
		"docker-real-27.5.1-rootless-cgroupfs-systemd-enabled": {
			parser: container.DockerParser,
			expected: &container.Info{
				PodID:       "",
				ID:          "1b8df7744e53956aa2cba289fd56da1b9caf0ee4c3d2294287020ba7e21885fb",
				Rootfulness: container.Rootless,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "proc", "1234"), 0755))
			require.NoError(t, utils.CopyFile(
				filepath.Join("testdata", "mountfile", name),
				filepath.Join(tempDir, "proc", "1234", "mountinfo"),
				0755),
			)

			info, err := container.LookupPID(tempDir, 1234, tc.parser)
			if tc.error != "" {
				require.ErrorContains(t, err, tc.error)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, info)
			}
		})
	}
}
