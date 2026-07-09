// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package local

import (
	"context"

	"github.com/gravitational/trace"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type LinuxDesktopService struct {
	service *generic.ServiceWrapper[*linuxdesktopv1.LinuxDesktop]
}

const linuxDesktopKey = "linux_desktop"

// NewLinuxDesktopService creates a new LinuxDesktopService.
func NewLinuxDesktopService(b backend.Backend) (*LinuxDesktopService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*linuxdesktopv1.LinuxDesktop]{
			Backend:       b,
			ResourceKind:  types.KindLinuxDesktop,
			BackendPrefix: backend.NewKey(linuxDesktopKey),
			MarshalFunc:   services.MarshalLinuxDesktop,
			UnmarshalFunc: services.UnmarshalLinuxDesktop,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &LinuxDesktopService{service: service}, nil
}

func (s *LinuxDesktopService) ListLinuxDesktops(ctx context.Context, pagesize int, lastKey string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, pagesize, lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *LinuxDesktopService) GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

func (s *LinuxDesktopService) CreateLinuxDesktop(ctx context.Context, linuxDesktop *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.CreateResource(ctx, linuxDesktop)
	return r, trace.Wrap(err)
}

func (s *LinuxDesktopService) UpdateLinuxDesktop(ctx context.Context, linuxDesktop *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, linuxDesktop)
	return r, trace.Wrap(err)
}

func (s *LinuxDesktopService) UpsertLinuxDesktop(ctx context.Context, linuxDesktop *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.UpsertResource(ctx, linuxDesktop)
	return r, trace.Wrap(err)
}

func (s *LinuxDesktopService) DeleteLinuxDesktop(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *LinuxDesktopService) DeleteAllLinuxDesktops(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}
