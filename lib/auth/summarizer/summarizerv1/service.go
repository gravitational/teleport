package summarizerv1

import (
	"log/slog"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type SummarizerServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.Summarizer
	Logger     *slog.Logger
}

type SummarizerService struct {
	pb.SummarizerServiceServer
	authorizer authz.Authorizer
	backend    services.Summarizer
	logger     *slog.Logger
}

func NewSummarizerService(cfg SummarizerServiceConfig) (*SummarizerService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "summarizer")
	}

	return &SummarizerService{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		logger:     cfg.Logger,
	}, nil
}
