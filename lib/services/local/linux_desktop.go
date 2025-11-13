package local

import (
	"context"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
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

func (s *LinuxDesktopService) CreateLinuxDesktop(ctx context.Context, crownJewel *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.CreateResource(ctx, crownJewel)
	return r, trace.Wrap(err)
}

func (s *LinuxDesktopService) UpdateLinuxDesktop(ctx context.Context, crownJewel *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, crownJewel)
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
