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

package decision

import (
	decision "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/trace"
)

// Permit is an interface that represents the common methods for all permit types.
type Permit interface {
	GetMetadata() *decision.PermitMetadata
}

// Denial is an interface that represents the common methods for all denial types.
type Denial interface {
	GetMetadata() *decision.DenialMetadata
}

// Decision is an interface that represents the common methods for all decision types.
type Decision[P, D any] interface {
	// GetPermit gets the permit from the decision.
	GetPermit() P
	// GetDenial gets the denial from the decision.
	GetDenial() D
}

var errAccessDenied = &trace.AccessDeniedError{Message: "access denied"}

var errMalformedDecision = &trace.AccessDeniedError{Message: "access denied due to malformed decision (this is a bug)"}

// static assertions that the Unwrap function works with the expected types.
var (
	_ = func(d *decision.EvaluateSSHAccessResponse) (*decision.SSHAccessPermit, error) {
		return Unwrap(d)
	}

	_ = func(d *decision.EvaluateDatabaseAccessResponse) (*decision.DatabaseAccessPermit, error) {
		return Unwrap(d)
	}
)

// Unwrap unwraps a decision into either a Permit or an AccessDenied error, discarding
// any denial details other than the user message.
func Unwrap[P interface {
	*PT
	Permit
}, D interface {
	*DT
	Denial
}, PT, DT any, DE Decision[P, D]](d DE) (P, error) {
	if denial := d.GetDenial(); denial != nil {
		// try to return the custom denial message if one exists
		if m := denial.GetMetadata(); m != nil && m.UserMessage != "" {
			return nil, trace.AccessDenied(m.UserMessage)
		}

		// fallback to a generic access denied error
		return nil, errAccessDenied
	}

	if permit := d.GetPermit(); permit != nil {
		return permit, nil
	}

	// one of the permit or denial should have been set, return an error indicating
	// malformed decision.
	return nil, errMalformedDecision
}
