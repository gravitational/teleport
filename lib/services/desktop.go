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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// WindowsDesktops defines an interface for managing Windows desktop hosts.
type WindowsDesktops interface {
	WindowsDesktopGetter
	CreateWindowsDesktop(context.Context, types.WindowsDesktop) error
	UpdateWindowsDesktop(context.Context, types.WindowsDesktop) error
	UpsertWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error
	DeleteWindowsDesktop(ctx context.Context, hostID, name string) error
	DeleteAllWindowsDesktops(context.Context) error
	ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error)
	ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error)
}

type WindowsDesktopGetter interface {
	GetWindowsDesktops(context.Context, types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)
}

// MarshalWindowsDesktop marshals the WindowsDesktop resource to JSON.
func MarshalWindowsDesktop(s types.WindowsDesktop, opts ...MarshalOption) ([]byte, error) {
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch s := s.(type) {
	case *types.WindowsDesktopV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *s
			copy.SetResourceID(0)
			s = &copy
		}
		return utils.FastMarshal(s)
	default:
		return nil, trace.BadParameter("unrecognized windows desktop version %T", s)
	}
}

// UnmarshalWindowsDesktop unmarshals the WindowsDesktop resource from JSON.
func UnmarshalWindowsDesktop(data []byte, opts ...MarshalOption) (types.WindowsDesktop, error) {
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
	case types.V3:
		var s types.WindowsDesktopV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("windows desktop resource version %q is not supported", h.Version)
}

// MarshalWindowsDesktopService marshals the WindowsDesktopService resource to JSON.
func MarshalWindowsDesktopService(s types.WindowsDesktopService, opts ...MarshalOption) ([]byte, error) {
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch s := s.(type) {
	case *types.WindowsDesktopServiceV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *s
			copy.SetResourceID(0)
			s = &copy
		}
		return utils.FastMarshal(s)
	default:
		return nil, trace.BadParameter("unrecognized windows desktop service version %T", s)
	}
}

// UnmarshalWindowsDesktopService unmarshals the WindowsDesktopService resource from JSON.
func UnmarshalWindowsDesktopService(data []byte, opts ...MarshalOption) (types.WindowsDesktopService, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing windows desktop service data")
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
	case types.V3:
		var s types.WindowsDesktopServiceV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("windows desktop service resource version %q is not supported", h.Version)
}
