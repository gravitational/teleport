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
)

// Attestation holds the results of the attestation process carried out on a
// PID by the attestor.
type Attestation struct {
	Unix       UnixAttestation
	Kubernetes KubernetesAttestation
}

// LogValue implements slog.LogValue to provide a nicely formatted set of
// log keys for a given attestation.
func (a Attestation) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Attr{
			Key:   "unix",
			Value: a.Unix.LogValue(),
		},
		slog.Attr{
			Key:   "kubernetes",
			Value: a.Kubernetes.LogValue(),
		},
	)
}

type attestor[T any] interface {
	Attest(ctx context.Context, pid int) (T, error)
}

// Attestor runs the workload attestation process on a given PID to determine
// key information about the process.
type Attestor struct {
	log        *slog.Logger
	kubernetes attestor[KubernetesAttestation]
	unix       attestor[UnixAttestation]
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

func (a *Attestor) Attest(ctx context.Context, pid int) (Attestation, error) {
	a.log.DebugContext(ctx, "Starting workload attestation", "pid", pid)
	defer a.log.DebugContext(ctx, "Finished workload attestation complete", "pid", pid)

	att := Attestation{}
	var err error

	// We always perform the unix attestation first
	att.Unix, err = a.unix.Attest(ctx, pid)
	if err != nil {
		return att, err
	}

	// Then we can perform the optionally configured attestations
	// For these, failure is soft. If it fails, we log, but still return the
	// successfully attested data.
	if a.kubernetes != nil {
		att.Kubernetes, err = a.kubernetes.Attest(ctx, pid)
		if err != nil {
			a.log.WarnContext(ctx, "Failed to perform Kubernetes workload attestation", "error", err)
		}
	}

	return att, nil
}
