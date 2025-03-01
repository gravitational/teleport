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

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/container"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/podman"
)

// PodmanAttestor attests the identity of a Podman container and pod.
type PodmanAttestor struct {
	cfg      PodmanAttestorConfig
	log      *slog.Logger
	rootPath string
}

// NewPodmanAttestor creates a new PodmanAttestor with the given configuration.
func NewPodmanAttestor(cfg PodmanAttestorConfig, log *slog.Logger) *PodmanAttestor {
	return &PodmanAttestor{
		cfg: cfg,
		log: log,
	}
}

// Attest the workload with the given PID.
func (a *PodmanAttestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrsPodman, error) {
	a.log.DebugContext(ctx, "Starting Podman workload attestation", "pid", pid)

	ctr, err := container.LookupPID(a.rootPath, pid, container.PodmanParser)
	if err != nil {
		return nil, trace.Wrap(err, "determining pod and container ID")
	}

	client, err := podman.NewClient(a.cfg.Addr)
	if err != nil {
		return nil, trace.Wrap(err, "creating Podman API client")
	}

	container, err := client.InspectContainer(ctx, ctr.ID)
	if err != nil {
		return nil, trace.Wrap(err, "inspecting container %q", ctr.ID)
	}

	attrs := &workloadidentityv1pb.WorkloadAttrsPodman{
		Attested: true,
		Container: &workloadidentityv1pb.WorkloadAttrsPodmanContainer{
			Name:   container.Name,
			Image:  container.Config.Image,
			Labels: container.Config.Labels,
		},
	}

	var pod *podman.Pod
	if ctr.PodID != "" {
		pod, err = client.InspectPod(ctx, ctr.PodID)
		if err != nil {
			return nil, trace.Wrap(err, "inspecting pod %q", ctr.PodID)
		}
		attrs.Pod = &workloadidentityv1pb.WorkloadAttrsPodmanPod{
			Name:   pod.Name,
			Labels: pod.Labels,
		}
	}

	return attrs, nil
}
