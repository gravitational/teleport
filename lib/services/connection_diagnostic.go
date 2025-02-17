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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ConnectionsDiagnostic defines an interface for managing Connection Diagnostics.
type ConnectionsDiagnostic interface {
	// CreateConnectionDiagnostic creates a new Connection Diagnostic
	CreateConnectionDiagnostic(context.Context, types.ConnectionDiagnostic) error

	// UpdateConnectionDiagnostic updates a Connection Diagnostic
	UpdateConnectionDiagnostic(context.Context, types.ConnectionDiagnostic) error

	// GetConnectionDiagnostic receives a name and returns the Connection Diagnostic matching that name
	//
	// If not found, a `trace.NotFound` error is returned
	GetConnectionDiagnostic(ctx context.Context, name string) (types.ConnectionDiagnostic, error)

	// ConnectionDiagnosticTraceAppender adds a method to append traces into ConnectionDiagnostics.
	ConnectionDiagnosticTraceAppender
}

// ConnectionDiagnosticTraceAppender specifies methods to add Traces into a DiagnosticConnection
type ConnectionDiagnosticTraceAppender interface {
	// AppendDiagnosticTrace atomically adds a new trace into the ConnectionDiagnostic.
	AppendDiagnosticTrace(ctx context.Context, name string, t *types.ConnectionDiagnosticTrace) (types.ConnectionDiagnostic, error)
}

// MarshalConnectionDiagnostic marshals the ConnectionDiagnostic resource to JSON.
func MarshalConnectionDiagnostic(s types.ConnectionDiagnostic, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch s := s.(type) {
	case *types.ConnectionDiagnosticV1:
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, s))
	}

	return nil, trace.BadParameter("unrecognized connection diagnostic version %T", s)
}

// UnmarshalConnectionDiagnostic unmarshals the ConnectionDiagnostic resource from JSON.
func UnmarshalConnectionDiagnostic(data []byte, opts ...MarshalOption) (types.ConnectionDiagnostic, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing connection diagnostic data")
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
		var s types.ConnectionDiagnosticV1
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter("%s", err)
		}

		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}

		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}

		return &s, nil
	}

	return nil, trace.BadParameter("connection diagnostic resource version %q is not supported", h.Version)
}
