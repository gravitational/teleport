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

package workloadattest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/podman"
	"github.com/gravitational/teleport/lib/utils"
)

func TestPodmanAttestor(t *testing.T) {
	server := podman.NewTestServer(t,
		podman.WithContainer(podman.Container{
			ID:   "d54768c18894b931db6f6876f6be2178d8a8b34fc3485659fda78fe86af3e08b",
			Name: "web-server",
			Config: podman.ContainerConfig{
				Image:  "nginx:latest",
				Labels: map[string]string{"region": "eu"},
			},
		}),
		podman.WithPod(podman.Pod{
			ID:     "5ffc3df0af9a6dd0f92668fc949734aad2ad41a5670b7218196d377d55ca32c5",
			Name:   "billing-system",
			Labels: map[string]string{"department": "marketing"},
		}),
	)

	attestor := NewPodmanAttestor(
		PodmanAttestorConfig{
			Enabled: true,
			Addr:    server.Addr(),
		},
		utils.NewSlogLoggerForTests(),
	)

	attestor.rootPath = t.TempDir()
	procPath := filepath.Join(attestor.rootPath, "proc", "1234")
	require.NoError(t, os.MkdirAll(procPath, 0755))

	require.NoError(t,
		utils.CopyFile(
			filepath.Join("container", "testdata", "mountfile", "podman-real-4.3.1-rootless-systemd-pod"),
			filepath.Join(procPath, "mountinfo"),
			0755,
		),
	)

	attrs, err := attestor.Attest(context.Background(), 1234)
	require.NoError(t, err)

	expected := &workloadidentityv1.WorkloadAttrsPodman{
		Attested: true,
		Container: &workloadidentityv1.WorkloadAttrsPodmanContainer{
			Name:   "web-server",
			Image:  "nginx:latest",
			Labels: map[string]string{"region": "eu"},
		},
		Pod: &workloadidentityv1.WorkloadAttrsPodmanPod{
			Name:   "billing-system",
			Labels: map[string]string{"department": "marketing"},
		},
	}
	require.Empty(t, cmp.Diff(expected, attrs, protocmp.Transform()))
}
