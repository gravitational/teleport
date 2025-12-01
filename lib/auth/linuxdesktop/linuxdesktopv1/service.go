package linuxdesktopv1

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the LinuxDesktop gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing LinuxDesktop.
	Backend services.LinuxDesktops

	// Reader is the cache for storing LinuxDesktop.
	Reader Reader

	// Emitter is the event emitter.
	Emitter events.Emitter
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Reader == nil {
		return trace.BadParameter("cache is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}

	return nil
}

type Reader interface {
	ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error)
	GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error)
}

// Service implements the teleport.LinuxDesktop.v1.LinuxDesktopService RPC service.
type Service struct {
	linuxdesktopv1.UnimplementedLinuxDesktopServiceServer

	authorizer authz.Authorizer
	backend    services.LinuxDesktops
	reader     Reader
	emitter    events.Emitter
}

// NewService returns a new LinuxDesktop gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		reader:     cfg.Reader,
		emitter:    cfg.Emitter,
	}, nil
}

// CreateLinuxDesktop creates Linux desktop resource.
func (s *Service) CreateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.CreateLinuxDesktopRequest) (rec *linuxdesktopv1.LinuxDesktop, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateLinuxDesktop(ctx, req.Desktop)

	return rsp, trace.Wrap(err)
}

// ListLinuxDesktops returns a list of Linux desktops.
func (s *Service) ListLinuxDesktops(ctx context.Context, req *linuxdesktopv1.ListLinuxDesktopsRequest) (*linuxdesktopv1.ListLinuxDesktopsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.reader.ListLinuxDesktops(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &linuxdesktopv1.ListLinuxDesktopsResponse{
		LinuxDesktops: rsp,
		NextPageToken: nextToken,
	}, nil
}

// GetLinuxDesktop returns Linux desktop resource.
func (s *Service) GetLinuxDesktop(ctx context.Context, req *linuxdesktopv1.GetLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.reader.GetLinuxDesktop(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// UpdateLinuxDesktop updates Linux desktop resource.
func (s *Service) UpdateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.UpdateLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateLinuxDesktop(ctx, req.LinuxDesktop)

	return rsp, trace.Wrap(err)
}

// UpsertLinuxDesktop upserts Linux desktop resource.
func (s *Service) UpsertLinuxDesktop(ctx context.Context, req *linuxdesktopv1.UpsertLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpsertLinuxDesktop(ctx, req.LinuxDesktop)

	return rsp, trace.Wrap(err)
}

// DeleteLinuxDesktop deletes Linux desktop resource.
func (s *Service) DeleteLinuxDesktop(ctx context.Context, req *linuxdesktopv1.DeleteLinuxDesktopRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = s.backend.DeleteLinuxDesktop(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func eventStatus(err error) events.Status {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return events.Status{
		Success:     err == nil,
		Error:       msg,
		UserMessage: msg,
	}
}

func getExpires(cj *timestamppb.Timestamp) time.Time {
	if cj == nil {
		return time.Time{}
	}
	return cj.AsTime()
}
