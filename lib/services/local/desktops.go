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

	"github.com/gravitational/teleport/api/defaults"
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

func (s *WindowsDesktopService) getWindowsDesktop(ctx context.Context, name, hostID string) (types.WindowsDesktop, error) {
	key := backend.NewKey(windowsDesktopsPrefix, hostID, name)
	item, err := s.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	desktop, err := s.itemToWindowsDesktop(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return desktop, nil
}

func (*WindowsDesktopService) itemToWindowsDesktop(item *backend.Item) (types.WindowsDesktop, error) {
	desktop, err := services.UnmarshalWindowsDesktop(
		item.Value,
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	return desktop, trace.Wrap(err)
}

func (s *WindowsDesktopService) getWindowsDesktopsForHostID(ctx context.Context, hostID string) ([]types.WindowsDesktop, error) {
	startKey := backend.ExactKey(windowsDesktopsPrefix, hostID)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var desktops []types.WindowsDesktop
	for _, item := range result.Items {
		desktop, err := s.itemToWindowsDesktop(&item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktops = append(desktops, desktop)
	}

	return desktops, nil
}

// GetWindowsDesktops returns all Windows desktops matching the specified filter.
// Most callers should prefer ListWindowsDesktops instead, as it supports pagination.
func (s *WindowsDesktopService) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	// TODO(zmb3,espadolini): implement this via ListWindowsDesktops

	// do a point-read instead of a range-read if a filter is provided
	if filter.HostID != "" && filter.Name != "" {
		desktop, err := s.getWindowsDesktop(ctx, filter.Name, filter.HostID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []types.WindowsDesktop{desktop}, nil
	}
	if filter.HostID != "" {
		return s.getWindowsDesktopsForHostID(ctx, filter.HostID)
	}

	startKey := backend.ExactKey(windowsDesktopsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var desktops []types.WindowsDesktop
	for _, item := range result.Items {
		desktop, err := s.itemToWindowsDesktop(&item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !filter.Match(desktop) {
			continue
		}
		desktops = append(desktops, desktop)
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
		Key:     backend.NewKey(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
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
		Key:      backend.NewKey(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
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
		Key:      backend.NewKey(windowsDesktopsPrefix, desktop.GetHostID(), desktop.GetName()),
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

	key := backend.NewKey(windowsDesktopsPrefix, hostID, name)

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
	if reqLimit <= 0 || reqLimit > defaults.DefaultChunkSize {
		reqLimit = defaults.DefaultChunkSize
	}

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

	var rangeStart, rangeEnd backend.Key
	switch {
	case req.Name != "" && req.HostID != "":
		rangeStart = backend.NewKey(windowsDesktopsPrefix, req.HostID, req.Name)
		rangeEnd = rangeStart
	case req.HostID != "":
		rangeStart = backend.ExactKey(windowsDesktopsPrefix, req.HostID)
		rangeEnd = backend.RangeEnd(rangeStart)
	default:
		rangeStart = backend.ExactKey(windowsDesktopsPrefix)
		rangeEnd = backend.RangeEnd(rangeStart)
	}

	if req.StartKey != "" {
		k := backend.NewKey(windowsDesktopsPrefix, req.StartKey)
		if k.Compare(rangeStart) > 0 {
			rangeStart = k
		}
	}

	var resp types.ListWindowsDesktopsResponse
	for item, err := range s.Backend.Items(ctx, backend.ItemsParams{
		StartKey: rangeStart,
		EndKey:   rangeEnd,
	}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		desktop, err := s.itemToWindowsDesktop(&item)
		if err != nil {
			continue
		}

		if !req.WindowsDesktopFilter.Match(desktop) {
			continue
		}

		switch match, err := services.MatchResourceByFilters(desktop, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			if len(resp.Desktops) >= reqLimit {
				resp.NextKey = backend.GetPaginationKey(desktop)
				return &resp, nil
			}

			resp.Desktops = append(resp.Desktops, desktop)
		}
	}

	return &resp, nil
}

func (s *WindowsDesktopService) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	reqLimit := req.Limit
	if reqLimit <= 0 || reqLimit > defaults.DefaultChunkSize {
		reqLimit = defaults.DefaultChunkSize
	}

	rangeStart := backend.NewKey(windowsDesktopServicesPrefix, req.StartKey)
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

	var resp types.ListWindowsDesktopServicesResponse
	for item, err := range s.Backend.Items(ctx, backend.ItemsParams{
		StartKey: rangeStart,
		EndKey:   rangeEnd,
	}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		desktop, err := services.UnmarshalWindowsDesktopService(
			item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision),
		)
		if err != nil {
			continue
		}

		switch match, err := services.MatchResourceByFilters(desktop, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			if len(resp.DesktopServices) >= reqLimit {
				resp.NextKey = backend.GetPaginationKey(desktop)
				return &resp, nil
			}

			resp.DesktopServices = append(resp.DesktopServices, desktop)
		}
	}

	return &resp, nil
}

const (
	windowsDesktopsPrefix = "windowsDesktop"
)
