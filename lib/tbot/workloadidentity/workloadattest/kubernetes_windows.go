//go:build windows

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

// WindowsKubernetesAttestor is the windows stub for KubernetesAttestor.
type WindowsKubernetesAttestor struct {
}

func (a WindowsKubernetesAttestor) Attest(_ context.Context, _ int) (*workloadidentityv1pb.WorkloadAttrsKubernetes, error) {
	return nil, trace.NotImplemented("kubernetes attestation is not supported on windows")
}

// NewKubernetesAttestor creates a new KubernetesAttestor.
func NewKubernetesAttestor(_ KubernetesAttestorConfig, _ *slog.Logger) *WindowsKubernetesAttestor {
	return &WindowsKubernetesAttestor{}
}
