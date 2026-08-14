/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// CheckBeamSSHOwnership enforces that only a beam's owner can SSH into it.
//
// A beam holds a delegation of its owner's identity, so SSH access to a beam
// is equivalent to impersonating the owner. This check is therefore a
// mandatory invariant, not role policy: it runs regardless of how permissive
// the connecting identity's roles are, and no role configuration can grant
// access to another user's beam.
//
// fromRemoteCluster must be true for identities issued by another cluster.
// Beams are root-cluster resources and ownership is matched by username, so
// cross-cluster identities are always denied: a leaf-cluster user who happens
// to share the owner's username must not match.
//
// Non-beam targets are always allowed through to ordinary RBAC evaluation.
func CheckBeamSSHOwnership(username string, fromRemoteCluster bool, target types.Server) error {
	if target == nil {
		// The target is unknown when proxying is evaluated for a bare SSH
		// subsystem dial. The node access evaluation that follows runs with a
		// concrete target and repeats this check.
		return nil
	}

	labels := target.GetAllLabels()
	if _, isBeam := labels[types.BeamIDLabel]; !isBeam {
		return nil
	}

	if fromRemoteCluster {
		return trace.AccessDenied("cross-cluster access to beams is not permitted")
	}

	owner, hasOwner := labels[types.BeamOwnerLabel]
	if !hasOwner || owner == "" {
		return trace.AccessDenied("beam node %q is missing owner label", target.GetName())
	}
	if username == "" || username != owner {
		return trace.AccessDenied("user %q cannot access beam owned by %q", username, owner)
	}
	return nil
}
