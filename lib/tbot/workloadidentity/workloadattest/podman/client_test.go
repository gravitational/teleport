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

package podman_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/podman"
)

func TestClient(t *testing.T) {
	server, err := podman.NewFakeServer(
		t.TempDir(),
		podman.WithContainer(podman.Container{
			ID:   "container-1234",
			Name: "web-server",
			Config: podman.ContainerConfig{
				Image:  "nginx",
				Labels: map[string]string{"team": "marketing"},
			},
			ImageDigest: "sha256:56fa17d2a7e7f168a043a2712e63aed1f8543aeafdcee47c58dcffe38ed51099",
		}),
		podman.WithPod(podman.Pod{
			ID:     "pod-1234",
			Name:   "web-app",
			Labels: map[string]string{"technology": "node.js"},
		}),
	)
	require.NoError(t, err)

	server.Start()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Logf("failed to close http server: %v", err)
		}
	})

	client, err := podman.NewClient(server.Addr())
	require.NoError(t, err)

	t.Run("inspect container success", func(t *testing.T) {
		container, err := client.InspectContainer(context.Background(), "container-1234")
		require.NoError(t, err)
		require.Equal(t, "web-server", container.Name)
	})

	t.Run("container not found", func(t *testing.T) {
		_, err := client.InspectContainer(context.Background(), "not-found")
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("inspect pod success", func(t *testing.T) {
		pod, err := client.InspectPod(context.Background(), "pod-1234")
		require.NoError(t, err)
		require.Equal(t, map[string]string{"technology": "node.js"}, pod.Labels)
	})

	t.Run("pod not found", func(t *testing.T) {
		_, err := client.InspectContainer(context.Background(), "not-found")
		require.True(t, trace.IsNotFound(err))
	})
}

func TestNewClient_TCP(t *testing.T) {
	_, err := podman.NewClient("http://localhost:1234")
	require.ErrorContains(t, err, "unix domain sockets")
}
