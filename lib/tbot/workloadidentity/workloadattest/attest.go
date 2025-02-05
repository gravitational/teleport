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
	"log/slog"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

type attestor[T any] interface {
	Attest(ctx context.Context, pid int) (T, error)
}

// Attestor runs the workload attestation process on a given PID to determine
// key information about the process.
type Attestor struct {
	log        *slog.Logger
	kubernetes attestor[*workloadidentityv1pb.WorkloadAttrsKubernetes]
	unix       attestor[*workloadidentityv1pb.WorkloadAttrsUnix]
}

// Config is the configuration for Attestor
type Config struct {
	Kubernetes KubernetesAttestorConfig `yaml:"kubernetes"`
}

func (c *Config) CheckAndSetDefaults() error {
	return trace.Wrap(c.Kubernetes.CheckAndSetDefaults(), "validating kubernetes")
}

// NewAttestor returns an Attestor from the given config.
func NewAttestor(log *slog.Logger, cfg Config) (*Attestor, error) {
	att := &Attestor{
		log:  log,
		unix: NewUnixAttestor(),
	}
	if cfg.Kubernetes.Enabled {
		att.kubernetes = NewKubernetesAttestor(cfg.Kubernetes, log)
	}
	return att, nil
}

func (a *Attestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrs, error) {
	a.log.DebugContext(ctx, "Starting workload attestation", "pid", pid)
	defer a.log.DebugContext(ctx, "Finished workload attestation", "pid", pid)

	var err error
	attrs := &workloadidentityv1pb.WorkloadAttrs{}
	// We always perform the unix attestation first
	attrs.Unix, err = a.unix.Attest(ctx, pid)
	if err != nil {
		return attrs, err
	}

	// Then we can perform the optionally configured attestations
	// For these, failure is soft. If it fails, we log, but still return the
	// successfully attested data.
	if a.kubernetes != nil {
		attrs.Kubernetes, err = a.kubernetes.Attest(ctx, pid)
		if err != nil {
			a.log.WarnContext(ctx, "Failed to perform Kubernetes workload attestation", "error", err)
		}
	}

	return attrs, nil
}
