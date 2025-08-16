//go:build !linux

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
)

// UnsupportedSystemdAttestor is the non-linux stub for SystemdAttestor.
type UnsupportedSystemdAttestor struct{}

// NewSystemdAttestor creates a new SystemdAttestor with the given configuration.
func NewSystemdAttestor(_ SystemdAttestorConfig, _ *slog.Logger) *UnsupportedSystemdAttestor {
	return &UnsupportedSystemdAttestor{}
}

// Attest the identity of the given systemd workload.
func (a *UnsupportedSystemdAttestor) Attest(_ context.Context, _ int) (*workloadidentityv1pb.WorkloadAttrsSystemd, error) {
	return nil, trace.NotImplemented("systemd attestation is only supported on linux")
}
