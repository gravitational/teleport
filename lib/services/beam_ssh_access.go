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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// beamTargetOwner inspects the target's labels and reports whether it is a
// beam SSH server and, if so, who owns it.
//
// Beam identity and ownership are read from the server-controlled static
// labels rather than the combined label set which includes dynamic labels.
func beamTargetOwner(target types.Server) (owner string, isBeam bool, err error) {
	hasBeamPrefix := func(labels map[string]string) bool {
		for key := range labels {
			if strings.HasPrefix(key, types.BeamsInternalLabelPrefix) {
				return true
			}
		}
		return false
	}

	static := target.GetStaticLabels()
	if !hasBeamPrefix(target.GetAllLabels()) && !hasBeamPrefix(static) {
		return "", false, nil
	}

	if _, ok := static[types.BeamIDLabel]; !ok {
		return "", true, trace.AccessDenied(
			"node %q carries beam labels but no beam ID; denying access to inconsistently marked node",
			target.GetName(),
		)
	}
	owner = static[types.BeamOwnerLabel]
	if owner == "" {
		return "", true, trace.AccessDenied("beam node %q is missing owner label", target.GetName())
	}
	return owner, true, nil
}

// CheckBeamSSHOwnership enforces that only a beam's owner can SSH into it.
func CheckBeamSSHOwnership(username, impersonator string, target types.Server) error {
	if target == nil {
		// The target is unknown when proxying is evaluated for a bare SSH
		// subsystem dial. The node access evaluation that follows runs with a
		// concrete target and repeats this check.
		return nil
	}

	owner, isBeam, err := beamTargetOwner(target)
	if err != nil {
		return trace.Wrap(err)
	}
	if !isBeam {
		return nil
	}

	if impersonator != "" && impersonator != username {
		return trace.AccessDenied(
			"impersonated identities cannot access beams (user %q impersonated by %q)",
			username, impersonator,
		)
	}
	if username == "" || username != owner {
		return trace.AccessDenied("user %q cannot access beam owned by %q", username, owner)
	}
	return nil
}

// CheckBeamSSHLogin enforces that beam SSH servers are only accessed as the
// dedicated beams OS login. Even the beam's owner must not log in as any
// other logins (e.g. root).
func CheckBeamSSHLogin(osUser string, target types.Server) error {
	if target == nil {
		return nil
	}

	_, isBeam, err := beamTargetOwner(target)
	if err != nil {
		return trace.Wrap(err)
	}
	if !isBeam {
		return nil
	}

	if osUser != types.BeamsLogin && osUser != teleport.SSHSessionJoinPrincipal {
		return trace.AccessDenied(
			"beams may only be accessed as the %q login, not %q",
			types.BeamsLogin, osUser,
		)
	}
	return nil
}
