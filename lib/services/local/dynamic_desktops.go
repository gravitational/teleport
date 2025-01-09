/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// DynamicWindowsDesktopService manages dynamic Windows desktop resources in the backend.
type DynamicWindowsDesktopService struct {
	service *generic.Service[types.DynamicWindowsDesktop]
}

// NewDynamicWindowsDesktopService creates a new WindowsDesktopsService.
func NewDynamicWindowsDesktopService(b backend.Backend) (*DynamicWindowsDesktopService, error) {
	service, err := generic.NewService(&generic.ServiceConfig[types.DynamicWindowsDesktop]{
		Backend:       b,
		ResourceKind:  types.KindDynamicWindowsDesktop,
		PageLimit:     defaults.MaxIterationLimit,
		BackendPrefix: backend.NewKey(dynamicWindowsDesktopsPrefix),
		MarshalFunc:   services.MarshalDynamicWindowsDesktop,
		UnmarshalFunc: services.UnmarshalDynamicWindowsDesktop,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DynamicWindowsDesktopService{
		service: service,
	}, nil
}

// GetDynamicWindowsDesktop returns dynamic Windows desktops by name.
func (s *DynamicWindowsDesktopService) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	desktop, err := s.service.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return desktop, err
}

// CreateDynamicWindowsDesktop creates a dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) CreateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	d, err := s.service.CreateResource(ctx, desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d, err
}

// UpdateDynamicWindowsDesktop updates a dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) UpdateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	// ConditionalUpdateResource can return invalid revision instead of not found, so we'll check if resource exists first
	if _, err := s.service.GetResource(ctx, desktop.GetName()); trace.IsNotFound(err) {
		return nil, err
	}
	d, err := s.service.ConditionalUpdateResource(ctx, desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d, err
}

// UpsertDynamicWindowsDesktop updates a dynamic Windows desktop resource, creating it if it doesn't exist.
func (s *DynamicWindowsDesktopService) UpsertDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	d, err := s.service.UpsertResource(ctx, desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d, err
}

// DeleteDynamicWindowsDesktop removes the specified dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) DeleteDynamicWindowsDesktop(ctx context.Context, name string) error {
	if err := s.service.DeleteResource(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllDynamicWindowsDesktops removes all dynamic Windows desktop resources.
func (s *DynamicWindowsDesktopService) DeleteAllDynamicWindowsDesktops(ctx context.Context) error {
	if err := s.service.DeleteAllResources(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListDynamicWindowsDesktops returns all dynamic Windows desktops matching filter.
func (s *DynamicWindowsDesktopService) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error) {
	desktops, next, err := s.service.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return desktops, next, nil
}

const (
	dynamicWindowsDesktopsPrefix = "dynamicWindowsDesktop"
)
