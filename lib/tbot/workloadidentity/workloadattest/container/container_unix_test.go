//go:build unix

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

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/container"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPID(t *testing.T) {
	tests := map[string]container.Identifiers{
		"k8s-real-docker-desktop": {
			PodID:       "941f292f-a62d-48ab-b9a8-eec84d87b928",
			ContainerID: "3f79e718744418736d0f6b9958e08d44e969c6577068c33de1cc400d35aacec8",
		},
		"k8s-real-orbstack": {
			PodID:       "36827f77-691f-45aa-a470-0989cf3749c4",
			ContainerID: "64dd9bf5199ff782835247cb072e4842dc3d0135ef02f6498cb6bb6f37a320d2",
		},
		"k8s-real-k3s-ubuntu-v1.28.6+k3s2": {
			PodID:       "fecd2321-17b5-49b9-9f75-8c5be777fbfb",
			ContainerID: "397529d07efebd566f15dbc7e8af9f3ef586033f5e753adfa96b2bf730102c64",
		},
		"k8s-real-gcp-v1.29.5-gke.1091002": {
			PodID:       "61c266b0-6f75-4490-8d92-3c9ae4d02787",
			ContainerID: "9da25af0b548c8c60aa60f77f299ba727bf72d58248bd7528eb5390ffcce555a",
		},
	}
	for name, expect := range tests {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "proc", "1234"), 0755))
			require.NoError(t, utils.CopyFile(
				filepath.Join("testdata", "mountfile", name),
				filepath.Join(tempDir, "proc", "1234", "mountinfo"),
				0755),
			)

			info, err := container.LookupPID(tempDir, 1234)
			require.NoError(t, err)
			assert.Equal(t, &expect, info)
		})
	}
}
