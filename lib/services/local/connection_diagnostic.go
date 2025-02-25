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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
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
	if err := services.CheckAndSetDefaults(connectionDiagnostic); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalConnectionDiagnostic(connectionDiagnostic)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(connectionDiagnosticPrefix, connectionDiagnostic.GetName()),
		Value:   value,
		Expires: connectionDiagnostic.Expiry(),
	}
	_, err = s.Create(ctx, item)

	return trace.Wrap(err)
}

// UpdateConnectionDiagnostic updates a Connection Diagnostic resource.
func (s *ConnectionDiagnosticService) UpdateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	if err := services.CheckAndSetDefaults(connectionDiagnostic); err != nil {
		return trace.Wrap(err)
	}
	rev := connectionDiagnostic.GetRevision()
	value, err := services.MarshalConnectionDiagnostic(connectionDiagnostic)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(connectionDiagnosticPrefix, connectionDiagnostic.GetName()),
		Value:    value,
		Expires:  connectionDiagnostic.Expiry(),
		Revision: rev,
	}
	_, err = s.Update(ctx, item)

	return trace.Wrap(err)
}

// AppendDiagnosticTrace adds a Trace into the ConnectionDiagnostics.
func (s *ConnectionDiagnosticService) AppendDiagnosticTrace(ctx context.Context, name string, t *types.ConnectionDiagnosticTrace) (types.ConnectionDiagnostic, error) {
	existing, err := s.Get(ctx, backend.NewKey(connectionDiagnosticPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("connection diagnostic %q doesn't exist", name)
		}

		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := services.UnmarshalConnectionDiagnostic(existing.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnostic.AppendTrace(t)

	value, err := services.MarshalConnectionDiagnostic(connectionDiagnostic)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:      backend.NewKey(connectionDiagnosticPrefix, connectionDiagnostic.GetName()),
		Value:    value,
		Expires:  connectionDiagnostic.Expiry(),
		Revision: existing.Revision,
	}

	_, err = s.ConditionalUpdate(ctx, newItem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionDiagnostic, nil
}

// GetConnectionDiagnostic receives a name and returns the Connection Diagnostic matching that name
//
// If not found, a `trace.NotFound` error is returned
func (s *ConnectionDiagnosticService) GetConnectionDiagnostic(ctx context.Context, name string) (types.ConnectionDiagnostic, error) {
	item, err := s.Get(ctx, backend.NewKey(connectionDiagnosticPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("connection diagnostic %q doesn't exist", name)
		}

		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := services.UnmarshalConnectionDiagnostic(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionDiagnostic, nil
}
