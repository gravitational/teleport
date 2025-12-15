//go:build windows

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

// WindowsPodmanAttestor is the windows stub for PodmanAttestor.
type WindowsPodmanAttestor struct {
}

func (a WindowsPodmanAttestor) Attest(_ context.Context, _ int) (*workloadidentityv1pb.WorkloadAttrsPodman, error) {
	return nil, trace.NotImplemented("podman attestation is not supported on windows")
}

// NewPodmanAttestor creates a new PodmanAttestor.
func NewPodmanAttestor(_ PodmanAttestorConfig, _ *slog.Logger) *WindowsPodmanAttestor {
	return &WindowsPodmanAttestor{}
}
