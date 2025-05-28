// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package scopes

import (
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

type scopedResource interface {
	GetMetadata() *headerv1.Metadata
	GetKind() string
	GetSubKind() string
	GetVersion() string
	GetScope() string
}

// ValidateScopedResource performs the subset of resource validation common to both weak and strong validation.
func ValidateScopedResource(r scopedResource, kind, version string) error {
	if r.GetMetadata().GetName() == "" {
		return trace.BadParameter("scoped resource %q is missing metadata.name", kind)
	}

	if r.GetKind() == "" {
		return trace.BadParameter("scoped resource %q %q is missing kind", kind, r.GetMetadata().GetName())
	}

	if r.GetKind() != kind {
		return trace.BadParameter("scoped resource %q %q has invalid kind %q, expected %q", kind, r.GetMetadata().GetName(), r.GetKind(), kind)
	}

	if r.GetSubKind() != "" {
		return trace.BadParameter("scoped resource %q %q has unknown sub_kind %q", kind, r.GetMetadata().GetName(), r.GetSubKind())
	}

	if r.GetVersion() == "" {
		return trace.BadParameter("scoped resource %q %q is missing version", kind, r.GetMetadata().GetName())
	}

	if r.GetVersion() != version {
		return trace.BadParameter("scoped resource %q %q has unsupported version %q (expected %q)", kind, r.GetMetadata().GetName(), r.GetVersion(), version)
	}

	if r.GetScope() == "" {
		return trace.BadParameter("scoped resource %q %q is missing scope", kind, r.GetMetadata().GetName())
	}

	return nil
}

// WeakValidateResource valides a resource to ensure it is free of obvious issues that would render it unusable and/or
// induce serious unintended behavior. Prefer using this function for validating resources loaded from "internal" sources
// (e.g. backend/control-plane), and stronger means for validating resources loaded from "external" sources (e.g. user input).
func WeakValidateResource(r scopedResource, kind, version string) error {
	if err := ValidateScopedResource(r, kind, version); err != nil {
		return trace.Wrap(err)
	}

	if err := WeakValidate(r.GetScope()); err != nil {
		return trace.BadParameter("scoped resource %q %q has invalid scope: %v", kind, r.GetMetadata().GetName(), err)
	}

	// NOTE: in strong validation, this is where we'd check that the assignable scopes are valid. In weak validation
	// we don't do that and instead rely on invalid assignable scopes being filtered out
	// and excluded during runtime assignment validation checks. This helps us ensure that outdated agents continue
	// to be able to understand and process the subset of assignments that they are able to reason about.
	return nil
}
