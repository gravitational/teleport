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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// DynamicWindowsDesktops defines an interface for managing dynamic Windows desktops.
type DynamicWindowsDesktops interface {
	GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error)
	CreateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	UpdateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	UpsertDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	DeleteDynamicWindowsDesktop(ctx context.Context, name string) error
	ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error)
}

// MarshalDynamicWindowsDesktop marshals the DynamicWindowsDesktop resource to JSON.
func MarshalDynamicWindowsDesktop(s types.DynamicWindowsDesktop, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch s := s.(type) {
	case *types.DynamicWindowsDesktopV1:
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, s))
	default:
		return nil, trace.BadParameter("unrecognized windows desktop version %T", s)
	}
}

// UnmarshalDynamicWindowsDesktop unmarshals the DynamicWindowsDesktop resource from JSON.
func UnmarshalDynamicWindowsDesktop(data []byte, opts ...MarshalOption) (types.DynamicWindowsDesktop, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing windows desktop data")
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
		var s types.DynamicWindowsDesktopV1
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
	return nil, trace.BadParameter("windows desktop resource version %q is not supported", h.Version)
}
