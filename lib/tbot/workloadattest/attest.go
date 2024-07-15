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
)

type Attestation struct {
	Unix       UnixAttestation
	Kubernetes KubernetesAttestation
}

type Attestor struct {
	log        *slog.Logger
	kubernetes *KubernetesAttestor
	unix       *UnixAttestor
}

type Config struct {
}

func NewAttestor(log *slog.Logger, cfg Config) *Attestor {
	// TODO: Setup Unix/Kubernetes attestators
	return &Attestor{
		log:  log,
		unix: &UnixAttestor{},
	}
}

func (a *Attestor) Attest(ctx context.Context, pid int) (Attestation, error) {
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
