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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// DynamicWindowsDesktopService manages dynamic Windows desktop resources in the backend.
type DynamicWindowsDesktopService struct {
	backend.Backend
}

// NewDynamicWindowsDesktopService creates a new WindowsDesktopsService.
func NewDynamicWindowsDesktopService(backend backend.Backend) *DynamicWindowsDesktopService {
	return &DynamicWindowsDesktopService{Backend: backend}
}

// GetDynamicWindowsDesktop returns dynamic Windows desktops by name.
func (s *DynamicWindowsDesktopService) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	item, err := s.Get(ctx, backend.ExactKey(dynamicWindowsDesktopsPrefix, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	desktop, err := services.UnmarshalDynamicWindowsDesktop(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return desktop, nil
}

// CreateDynamicWindowsDesktop creates a dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) CreateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalDynamicWindowsDesktop(desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(dynamicWindowsDesktopsPrefix, desktop.GetName()),
		Value:   value,
		Expires: desktop.Expiry(),
	}
	lease, err := s.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return nil, trace.AlreadyExists("dynamic windows desktop %q already exist", desktop.GetName())
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	newDesktop := desktop.Copy()
	newDesktop.SetRevision(lease.Revision)

	return newDesktop, nil
}

// UpdateDynamicWindowsDesktop updates a dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) UpdateDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := desktop.GetRevision()
	value, err := services.MarshalDynamicWindowsDesktop(desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(dynamicWindowsDesktopsPrefix, desktop.GetName()),
		Value:    value,
		Expires:  desktop.Expiry(),
		Revision: rev,
	}
	lease, err := s.Update(ctx, item)
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("windows desktop %q doesn't exist", desktop.GetName())
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	newDesktop := desktop.Copy()
	newDesktop.SetRevision(lease.Revision)

	return newDesktop, nil
}

// UpsertDynamicWindowsDesktop updates a dynamic Windows desktop resource, creating it if it doesn't exist.
func (s *DynamicWindowsDesktopService) UpsertDynamicWindowsDesktop(ctx context.Context, desktop types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := desktop.GetRevision()
	value, err := services.MarshalDynamicWindowsDesktop(desktop)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(dynamicWindowsDesktopsPrefix, desktop.GetName()),
		Value:    value,
		Expires:  desktop.Expiry(),
		Revision: rev,
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newDesktop := desktop.Copy()
	newDesktop.SetRevision(lease.Revision)

	return newDesktop, nil
}

// DeleteDynamicWindowsDesktop removes the specified dynamic Windows desktop resource.
func (s *DynamicWindowsDesktopService) DeleteDynamicWindowsDesktop(ctx context.Context, name string) error {
	if name == "" {
		return trace.Errorf("name must not be empty")
	}

	key := backend.NewKey(dynamicWindowsDesktopsPrefix, name)

	err := s.Delete(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("windows desktop \"%s\" doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllDynamicWindowsDesktops removes all dynamic Windows desktop resources.
func (s *DynamicWindowsDesktopService) DeleteAllDynamicWindowsDesktops(ctx context.Context) error {
	startKey := backend.ExactKey(dynamicWindowsDesktopsPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListDynamicWindowsDesktops returns all dynamic Windows desktops matching filter.
func (s *DynamicWindowsDesktopService) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error) {
	reqLimit := pageSize
	if reqLimit <= 0 {
		return nil, "", trace.BadParameter("nonpositive parameter limit")
	}

	rangeStart := backend.NewKey(dynamicWindowsDesktopsPrefix, pageToken)
	rangeEnd := backend.RangeEnd(backend.ExactKey(dynamicWindowsDesktopsPrefix))

	// Get most limit+1 results to determine if there will be a next key.
	maxLimit := reqLimit + 1
	var desktops []types.DynamicWindowsDesktop
	if err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, maxLimit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(desktops) == maxLimit {
				break
			}

			desktop, err := services.UnmarshalDynamicWindowsDesktop(item.Value,
				services.WithExpires(item.Expires), services.WithRevision(item.Revision))
			if err != nil {
				return false, trace.Wrap(err)
			}
			desktops = append(desktops, desktop)
		}

		return len(desktops) == maxLimit, nil
	}); err != nil {
		return nil, "", trace.Wrap(err)
	}

	var nextKey string
	if len(desktops) > reqLimit {
		nextKey = backend.GetPaginationKey(desktops[len(desktops)-1])
		// Truncate the last item that was used to determine next row existence.
		desktops = desktops[:reqLimit]
	}

	return desktops, nextKey, nil
}

const (
	dynamicWindowsDesktopsPrefix = "dynamicWindowsDesktop"
)
