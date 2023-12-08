/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/okta"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Compile time checks for the Okta client.
var _ OktaImportRules = (*okta.Client)(nil)
var _ OktaAssignments = (*okta.Client)(nil)

// Okta is an Okta interface for both the rules and assignments.
type Okta interface {
	OktaImportRules
	OktaAssignments
}

// OktaImportRules defines an interface for managing OktaImportRules.
type OktaImportRules interface {
	// ListOktaImportRules returns a paginated list of all Okta import rule resources.
	ListOktaImportRules(context.Context, int, string) ([]types.OktaImportRule, string, error)
	// GetOktaImportRule returns the specified Okta import rule resources.
	GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error)
	// CreateOktaImportRule creates a new Okta import rule resource.
	CreateOktaImportRule(context.Context, types.OktaImportRule) (types.OktaImportRule, error)
	// UpdateOktaImportRule updates an existing Okta import rule resource.
	UpdateOktaImportRule(context.Context, types.OktaImportRule) (types.OktaImportRule, error)
	// DeleteOktaImportRule removes the specified Okta import rule resource.
	DeleteOktaImportRule(ctx context.Context, name string) error
	// DeleteAllOktaImportRules removes all Okta import rules.
	DeleteAllOktaImportRules(context.Context) error
}

// OktaAssignmentsGetter defines an interface for reading OktaAssignments.
type OktaAssignmentsGetter interface {
	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
	// GetOktaAssignment returns the specified Okta assignment resources.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)
}

// OktaAssignments defines an interface for managing OktaAssignments.
type OktaAssignments interface {
	OktaAssignmentsGetter

	// CreateOktaAssignment creates a new Okta assignment resource.
	CreateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
	// since the last transition.
	UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
	// DeleteAllOktaAssignments removes all Okta assignments.
	DeleteAllOktaAssignments(context.Context) error
}

// MarshalOktaImportRule marshals the Okta import rule resource to JSON.
func MarshalOktaImportRule(importRule types.OktaImportRule, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch i := importRule.(type) {
	case *types.OktaImportRuleV1:
		if err := i.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, i))
	default:
		return nil, trace.BadParameter("unsupported Okta import rule resource %T", i)
	}
}

// UnmarshalOktaImportRule unmarshals Okta import rule resource from JSON.
func UnmarshalOktaImportRule(data []byte, opts ...MarshalOption) (types.OktaImportRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing Okta import rule data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var i types.OktaImportRuleV1
		if err := utils.FastUnmarshal(data, &i); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := i.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			i.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			i.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			i.SetExpiry(cfg.Expires)
		}
		return &i, nil
	}
	return nil, trace.BadParameter("unsupported Okta import rule resource version %q", h.Version)
}

// MarshalOktaAssignment marshals the Okta assignment resource to JSON.
func MarshalOktaAssignment(assignment types.OktaAssignment, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch a := assignment.(type) {
	case *types.OktaAssignmentV1:
		if err := a.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, a))
	default:
		return nil, trace.BadParameter("unsupported Okta assignment resource %T", a)
	}
}

// UnmarshalOktaAssignment unmarshals the Okta assignment resource from JSON.
func UnmarshalOktaAssignment(data []byte, opts ...MarshalOption) (types.OktaAssignment, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing Okta assignment data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var a types.OktaAssignmentV1
		if err := utils.FastUnmarshal(data, &a); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := a.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			a.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			a.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			a.SetExpiry(cfg.Expires)
		}
		return &a, nil
	}
	return nil, trace.BadParameter("unsupported Okta assignment resource version %q", h.Version)
}
