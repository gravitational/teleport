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

// WindowsDesktopService manages windows desktop resources in the backend.
type WindowsDesktopService struct {
	backend.Backend
}

// NewWindowsDesktopService creates a new WindowsDesktopsService.
func NewWindowsDesktopService(backend backend.Backend) *WindowsDesktopService {
	return &WindowsDesktopService{Backend: backend}
}

// GetWindowsDesktops returns all Windows desktops matching filter.
func (s *WindowsDesktopService) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	startKey := backend.ExactKey(windowsDesktopsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var desktops []types.WindowsDesktop
	for _, item := range result.Items {
		desktop, err := services.UnmarshalWindowsDesktop(item.Value,
			services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !filter.Match(desktop) {
			continue
		}
		desktops = append(desktops, desktop)
	}
	// If both HostID and Name are set in the filter only one desktop should be expected
	if filter.HostID != "" && filter.Name != "" && len(desktops) == 0 {
		return nil, trace.NotFound("windows desktop \"%s/%s\" doesn't exist", filter.HostID, filter.Name)
	}

	return desktops, nil
}

// CreateWindowsDesktop creates a windows desktop resource.
func (s *WindowsDesktopService) CreateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalWindowsDesktop(desktop)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
		Value:   value,
		Expires: desktop.Expiry(),
	}
	_, err = s.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("windows desktop %q %q doesn't exist", desktop.GetHostID(), desktop.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateWindowsDesktop updates a windows desktop resource.
func (s *WindowsDesktopService) UpdateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return trace.Wrap(err)
	}
	rev := desktop.GetRevision()
	value, err := services.MarshalWindowsDesktop(desktop)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
		Value:    value,
		Expires:  desktop.Expiry(),
		Revision: rev,
	}
	_, err = s.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("windows desktop %q %q  doesn't exist", desktop.GetHostID(), desktop.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertWindowsDesktop updates a windows desktop resource, creating it if it doesn't exist.
func (s *WindowsDesktopService) UpsertWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := services.CheckAndSetDefaults(desktop); err != nil {
		return trace.Wrap(err)
	}
	rev := desktop.GetRevision()
	value, err := services.MarshalWindowsDesktop(desktop)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.Key(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
		Value:    value,
		Expires:  desktop.Expiry(),
		Revision: rev,
	}
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteWindowsDesktop removes the specified windows desktop resource.
func (s *WindowsDesktopService) DeleteWindowsDesktop(ctx context.Context, hostID, name string) error {
	if name == "" {
		return trace.Errorf("name must not be empty")
	}

	key := backend.Key(windowsDesktopsPrefix, hostID, name)

	err := s.Delete(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("windows desktop \"%s/%s\" doesn't exist", hostID, name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllWindowsDesktops removes all windows desktop resources.
func (s *WindowsDesktopService) DeleteAllWindowsDesktops(ctx context.Context) error {
	startKey := backend.ExactKey(windowsDesktopsPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListWindowsDesktops returns all Windows desktops matching filter.
func (s *WindowsDesktopService) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	reqLimit := req.Limit
	if reqLimit <= 0 {
		return nil, trace.BadParameter("nonpositive parameter limit")
	}

	rangeStart := backend.Key(windowsDesktopsPrefix, req.StartKey)
	rangeEnd := backend.RangeEnd(backend.ExactKey(windowsDesktopsPrefix))
	filter := services.MatchResourceFilter{
		ResourceKind:   types.KindWindowsDesktop,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	// Get most limit+1 results to determine if there will be a next key.
	maxLimit := reqLimit + 1
	var desktops []types.WindowsDesktop
	if err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, maxLimit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(desktops) == maxLimit {
				break
			}

			desktop, err := services.UnmarshalWindowsDesktop(item.Value,
				services.WithExpires(item.Expires), services.WithRevision(item.Revision))
			if err != nil {
				return false, trace.Wrap(err)
			}

			if !req.WindowsDesktopFilter.Match(desktop) {
				continue
			}

			switch match, err := services.MatchResourceByFilters(desktop, filter, nil /* ignore dup matches */); {
			case err != nil:
				return false, trace.Wrap(err)
			case match:
				desktops = append(desktops, desktop)
			}
		}

		return len(desktops) == maxLimit, nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// If both HostID and Name are set in the filter only one desktop should be expected
	if req.HostID != "" && req.Name != "" && len(desktops) == 0 {
		return nil, trace.NotFound("windows desktop \"%s/%s\" doesn't exist", req.HostID, req.Name)
	}

	var nextKey string
	if len(desktops) > reqLimit {
		nextKey = backend.GetPaginationKey(desktops[len(desktops)-1])
		// Truncate the last item that was used to determine next row existence.
		desktops = desktops[:reqLimit]
	}

	return &types.ListWindowsDesktopsResponse{
		Desktops: desktops,
		NextKey:  nextKey,
	}, nil
}

func (s *WindowsDesktopService) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	reqLimit := req.Limit
	if reqLimit <= 0 {
		return nil, trace.BadParameter("nonpositive parameter limit")
	}

	rangeStart := backend.Key(windowsDesktopServicesPrefix, req.StartKey)
	rangeEnd := backend.RangeEnd(backend.ExactKey(windowsDesktopServicesPrefix))
	filter := services.MatchResourceFilter{
		ResourceKind:   types.KindWindowsDesktopService,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	// Get most limit+1 results to determine if there will be a next key.
	maxLimit := reqLimit + 1
	var desktopServices []types.WindowsDesktopService
	if err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, maxLimit, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(desktopServices) == maxLimit {
				break
			}

			desktop, err := services.UnmarshalWindowsDesktopService(item.Value,
				services.WithExpires(item.Expires), services.WithRevision(item.Revision))
			if err != nil {
				return false, trace.Wrap(err)
			}

			switch match, err := services.MatchResourceByFilters(desktop, filter, nil /* ignore dup matches */); {
			case err != nil:
				return false, trace.Wrap(err)
			case match:
				desktopServices = append(desktopServices, desktop)
			}
		}

		return len(desktopServices) == maxLimit, nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	var nextKey string
	if len(desktopServices) > reqLimit {
		nextKey = backend.GetPaginationKey(desktopServices[len(desktopServices)-1])
		// Truncate the last item that was used to determine next row existence.
		desktopServices = desktopServices[:reqLimit]
	}

	return &types.ListWindowsDesktopServicesResponse{
		DesktopServices: desktopServices,
		NextKey:         nextKey,
	}, nil
}

const (
	windowsDesktopsPrefix = "windowsDesktop"
)
