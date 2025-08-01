package service

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/intune"
)

// Config contains parameters needed by [Service].
type Config struct {
	APIConfig  intune.APIConfig
	Logger     *slog.Logger
	HTTPClient *http.Client
}

// New creates a new [Service].
// ctx is used to perform initial validations against the Intune API.
func New(ctx context.Context, config Config) (*Service, error) {
	// TODO(ravicious): Register metrics like the Jamf service does.

	if config.Logger == nil {
		return nil, trace.BadParameter("parameter Logger required")
	}

	// TODO(ravicious): Create a scheduler like the Jamf service does.

	// Connect to the Intune API and verify credentials.
	_, err := intune.NewClient(ctx, intune.ClientConfig{
		APIConfig:  config.APIConfig,
		Logger:     config.Logger,
		HTTPClient: config.HTTPClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{}, nil
}

type Service struct{}

func (s *Service) Run(ctx context.Context) error {
	return trace.NotImplemented("service.Run not implemented")
}
