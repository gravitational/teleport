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
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestDockerAttestor(t *testing.T) {
	lis, err := net.Listen("unix", filepath.Join(t.TempDir(), "docker.sock"))
	require.NoError(t, err)

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/_ping":
				w.Header().Set("Api-Version", "1.47")
			case "/v1.47/containers/9125fbc01fb958c33eb2fda134db64e2c01ec456181fb5def541d6485ea810ba/json":
				var rsp struct {
					Name   string
					Config struct {
						Image  string
						Labels map[string]string
					}
				}
				rsp.Name = "web-server"
				rsp.Config.Image = "nginx:latest"
				rsp.Config.Labels = map[string]string{"region": "eu"}
				_ = json.NewEncoder(w).Encode(rsp)
			default:
				http.NotFound(w, r)
			}
		}),
	}
	go func() { _ = httpServer.Serve(lis) }()

	t.Cleanup(func() {
		if err := httpServer.Close(); err != nil {
			t.Logf("failed to close http server: %v", err)
		}
	})

	attestor := NewDockerAttestor(
		DockerAttestorConfig{
			Enabled: true,
			Addr:    "unix://" + lis.Addr().String(),
		},
		utils.NewSlogLoggerForTests(),
	)

	attestor.rootPath = t.TempDir()
	procPath := filepath.Join(attestor.rootPath, "proc", "1234")
	require.NoError(t, os.MkdirAll(procPath, 0755))

	require.NoError(t,
		utils.CopyFile(
			filepath.Join("container", "testdata", "mountfile", "docker-real-27.5.1-rootful-systemd"),
			filepath.Join(procPath, "mountinfo"),
			0755,
		),
	)

	attrs, err := attestor.Attest(context.Background(), 1234)
	require.NoError(t, err)

	expected := &workloadidentityv1.WorkloadAttrsDocker{
		Attested: true,
		Container: &workloadidentityv1.WorkloadAttrsDockerContainer{
			Name:   "web-server",
			Image:  "nginx:latest",
			Labels: map[string]string{"region": "eu"},
		},
	}
	require.Empty(t, cmp.Diff(expected, attrs, protocmp.Transform()))
}
