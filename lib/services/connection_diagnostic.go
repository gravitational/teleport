/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch s := s.(type) {
	case *types.ConnectionDiagnosticV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *s
			copy.SetResourceID(0)
			s = &copy
		}

		return utils.FastMarshal(s)
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
			return nil, trace.BadParameter(err.Error())
		}

		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
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
