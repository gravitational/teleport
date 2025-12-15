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
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/attrs"
)

type attestor[T any] interface {
	Attest(ctx context.Context, pid int) (T, error)
}

// Attestor runs the workload attestation process on a given PID to determine
// key information about the process.
type Attestor struct {
	log        *slog.Logger
	kubernetes attestor[*workloadidentityv1pb.WorkloadAttrsKubernetes]
	podman     attestor[*workloadidentityv1pb.WorkloadAttrsPodman]
	docker     attestor[*workloadidentityv1pb.WorkloadAttrsDocker]
	systemd    attestor[*workloadidentityv1pb.WorkloadAttrsSystemd]
	unix       attestor[*workloadidentityv1pb.WorkloadAttrsUnix]
	sigstore   *SigstoreAttestor
}

// Config is the configuration for Attestor
type Config struct {
	Kubernetes KubernetesAttestorConfig `yaml:"kubernetes"`
	Podman     PodmanAttestorConfig     `yaml:"podman"`
	Docker     DockerAttestorConfig     `yaml:"docker"`
	Systemd    SystemdAttestorConfig    `yaml:"systemd"`
	Unix       UnixAttestorConfig       `yaml:"unix"`
	Sigstore   SigstoreAttestorConfig   `yaml:"sigstore"`
}

func (c *Config) CheckAndSetDefaults() error {
	if err := c.Kubernetes.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating kubernetes")
	}
	if err := c.Podman.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating podman")
	}
	if err := c.Docker.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating docker")
	}
	if err := c.Unix.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating unix")
	}
	if err := c.Sigstore.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating sigstore")
	}
	return nil
}

// NewAttestor returns an Attestor from the given config.
func NewAttestor(log *slog.Logger, cfg Config) (*Attestor, error) {
	att := &Attestor{
		log:  log,
		unix: NewUnixAttestor(cfg.Unix, log),
	}
	if cfg.Kubernetes.Enabled {
		att.kubernetes = NewKubernetesAttestor(cfg.Kubernetes, log)
	}
	if cfg.Podman.Enabled {
		att.podman = NewPodmanAttestor(cfg.Podman, log)
	}
	if cfg.Docker.Enabled {
		att.docker = NewDockerAttestor(cfg.Docker, log)
	}
	if cfg.Systemd.Enabled {
		att.systemd = NewSystemdAttestor(cfg.Systemd, log)
	}
	if cfg.Sigstore.Enabled {
		sigstore, err := NewSigstoreAttestor(cfg.Sigstore, log)
		if err != nil {
			return nil, trace.Wrap(err, "creating sigstore attestor")
		}
		att.sigstore = sigstore
	}
	return att, nil
}

func (a *Attestor) Attest(ctx context.Context, pid int) (*attrs.WorkloadAttrs, error) {
	a.log.DebugContext(ctx, "Starting workload attestation", "pid", pid)
	defer a.log.DebugContext(ctx, "Finished workload attestation", "pid", pid)

	var err error
	attrs := attrs.NewWorkloadAttrs()
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
	if a.podman != nil {
		attrs.Podman, err = a.podman.Attest(ctx, pid)
		if err != nil {
			a.log.WarnContext(ctx, "Failed to perform Podman workload attestation", "error", err)
		}
	}
	if a.docker != nil {
		attrs.Docker, err = a.docker.Attest(ctx, pid)
		if err != nil {
			a.log.WarnContext(ctx, "Failed to perform Docker workload attestation", "error", err)
		}
	}
	if a.systemd != nil {
		attrs.Systemd, err = a.systemd.Attest(ctx, pid)
		if err != nil {
			a.log.WarnContext(ctx, "Failed to perform Systemd workload attestation", "error", err)
		}
	}

	if a.sigstore != nil {
		if ctr := a.containerAttributes(attrs); ctr != nil {
			attrs.Sigstore, err = a.sigstore.Attest(ctx, ctr)
			if err != nil {
				a.log.WarnContext(ctx, "Failed to perform Sigstore workload attestation", "error", err)
			}
		}
	}

	return attrs, nil
}

// Failed is called when getting a workload identity with the supplied workload
// attributes failed. It's used to clear any caches before the client tries again.
func (a *Attestor) Failed(ctx context.Context, attrs *attrs.WorkloadAttrs) {
	if a.sigstore == nil {
		return
	}
	if ctr := a.containerAttributes(attrs); ctr != nil {
		a.sigstore.MarkFailed(ctx, ctr)
	}
}

// containerAttributes returns the attested container information. It assumes
// that only one attestor (i.e. Kubernetes, Podman, or Docker) will be in use
// which is a relatively safe assumption because they each depend on differently
// structured cgroup names.
func (a *Attestor) containerAttributes(attrs *attrs.WorkloadAttrs) Container {
	if ctr := attrs.GetKubernetes().GetContainer(); ctr != nil {
		return ctr
	}
	if ctr := attrs.GetPodman().GetContainer(); ctr != nil {
		return ctr
	}
	if ctr := attrs.GetDocker().GetContainer(); ctr != nil {
		return ctr
	}
	return nil
}
