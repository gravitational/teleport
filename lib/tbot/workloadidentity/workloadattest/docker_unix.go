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
	"log/slog"
	"time"

	docker "github.com/docker/docker/client"
	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/container"
)

// DockerAttestor attests the identity of a Docker container.
type DockerAttestor struct {
	cfg      DockerAttestorConfig
	log      *slog.Logger
	rootPath string
}

// NewDockerAttestor creates a new Docker with the given configuration.
func NewDockerAttestor(cfg DockerAttestorConfig, log *slog.Logger) *DockerAttestor {
	return &DockerAttestor{
		cfg: cfg,
		log: log,
	}
}

// Attest the workload with the given PID.
func (a *DockerAttestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrsDocker, error) {
	a.log.DebugContext(ctx, "Starting Docker workload attestation", "pid", pid)

	ctr, err := container.LookupPID(a.rootPath, pid, container.DockerParser)
	if err != nil {
		return nil, trace.Wrap(err, "determining container ID")
	}

	client, err := docker.NewClientWithOpts(
		docker.WithHost(a.cfg.Addr),
		docker.WithAPIVersionNegotiation(),
		docker.WithTimeout(1*time.Second),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating docker client")
	}

	container, err := client.ContainerInspect(ctx, ctr.ID)
	if err != nil {
		return nil, trace.Wrap(err, "inspecting container")
	}

	return &workloadidentityv1pb.WorkloadAttrsDocker{
		Attested: true,
		Container: &workloadidentityv1pb.WorkloadAttrsDockerContainer{
			Name:   container.Name,
			Image:  container.Config.Image,
			Labels: container.Config.Labels,
		},
	}, nil
}
