/*
Copyright 2021 Gravitational, Inc.

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

// WindowsDesktopService manages windows desktop resources in the backend.
type WindowsDesktopService struct {
	backend.Backend
}

// NewWindowsDesktopService creates a new WindowsDesktopsService.
func NewWindowsDesktopService(backend backend.Backend) *WindowsDesktopService {
	return &WindowsDesktopService{Backend: backend}
}

// GetWindowsDesktops returns all windows desktop resources.
func (s *WindowsDesktopService) GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error) {
	startKey := backend.Key(windowsDeskoptsPrefix, "")
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	desktops := make([]types.WindowsDesktop, len(result.Items))
	for i, item := range result.Items {
		desktop, err := services.UnmarshalWindowsDesktop(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktops[i] = desktop
	}
	return desktops, nil
}

// GetWindowsDesktop returns the specified windows desktop resource.
func (s *WindowsDesktopService) GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error) {
	item, err := s.Get(ctx, backend.Key(windowsDeskoptsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("windows desktop %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	desktop, err := services.UnmarshalWindowsDesktop(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return desktop, nil
}

// CreateWindowsDesktop creates a windows desktop resource.
func (s *WindowsDesktopService) CreateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := desktop.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalWindowsDesktop(desktop)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(windowsDeskoptsPrefix, desktop.GetName()),
		Value:   value,
		Expires: desktop.Expiry(),
		ID:      desktop.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateWindowsDesktop updates a windows desktop resource.
func (s *WindowsDesktopService) UpdateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := desktop.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalWindowsDesktop(desktop)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(windowsDeskoptsPrefix, desktop.GetName()),
		Value:   value,
		Expires: desktop.Expiry(),
		ID:      desktop.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteWindowsDesktop removes the specified windows desktop resource.
func (s *WindowsDesktopService) DeleteWindowsDesktop(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(windowsDeskoptsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("windows desktop %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllWindowsDesktops removes all windows desktop resources.
func (s *WindowsDesktopService) DeleteAllWindowsDesktops(ctx context.Context) error {
	startKey := backend.Key(windowsDeskoptsPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	windowsDeskoptsPrefix = "windowsDesktop"
)
