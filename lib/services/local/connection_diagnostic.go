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

package local

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// ConnectionDiagnosticService manages connection diagnostic resources in the backend.
type ConnectionDiagnosticService struct {
	backend.Backend
}

const (
	connectionDiagnosticPrefix = "connectionDiagnostic"
)

// NewConnectionsDiagnosticService creates a new ConnectionsDiagnosticService.
func NewConnectionsDiagnosticService(backend backend.Backend) *ConnectionDiagnosticService {
	return &ConnectionDiagnosticService{Backend: backend}
}

// CreateConnectionDiagnostic creates a Connection Diagnostic resource.
func (s *ConnectionDiagnosticService) CreateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	if err := connectionDiagnostic.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalConnectionDiagnostic(connectionDiagnostic)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(connectionDiagnosticPrefix, connectionDiagnostic.GetName()),
		Value:   value,
		Expires: connectionDiagnostic.Expiry(),
		ID:      connectionDiagnostic.GetResourceID(),
	}
	_, err = s.Create(ctx, item)

	return trace.Wrap(err)
}

// GetConnectionDiagnostic receives a name and returns the Connection Diagnostic matching that name
//
// If not found, a `trace.NotFound` error is returned
func (s *ConnectionDiagnosticService) GetConnectionDiagnostic(ctx context.Context, name string) (types.ConnectionDiagnostic, error) {
	item, err := s.Get(ctx, backend.Key(connectionDiagnosticPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("connection diagnostic %q doesn't exist", name)
		}

		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := services.UnmarshalConnectionDiagnostic(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionDiagnostic, nil
}
