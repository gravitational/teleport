package mfa

import (
	"context"
	"log/slog"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// CheckAndSetDefaults checks the config and sets default values where appropriate.
func (c *Config) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("mfa service config missing required parameter AccessPoint")
	}

	return nil
}

// NodeGetter is a service that gets a node.
type NodeGetter interface {
	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
}

// ClusterNetworkingConfigGetter is a service that gets the cluster networking configuration.
type ClusterNetworkingConfigGetter interface {
	// GetClusterNetworkingConfig returns the cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
}

// AccessPoint represents the upstream data source required by the mfa service.
type AccessPoint interface {
	services.ClusterNameGetter
	services.RoleGetter
	NodeGetter
	services.AuthPreferenceGetter
	services.AuthorityGetter
	ClusterNetworkingConfigGetter
	services.UserGetter
}

// Config configures the core mfa service impl.
type Config struct {
	// AccessPoint is the upstream data source required by the mfa service.
	AccessPoint AccessPoint
}

// Service is the core mfa service implementation.
type Service struct {
	cfg Config
}

var _ mfav1.MFAServiceServer = &Service{}

// NewService creates a new mfa service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

// CreateChallengeForAction creates an MFA challenge that is tied to a specific user action. The action_id is required
// and the created challenge will be correlated to that action.
func (s *Service) CreateChallengeForAction(
	ctx context.Context,
	req *mfav1.CreateChallengeForActionRequest,
) (*mfav1.CreateChallengeForActionResponse, error) {
	switch {
	case req.GetActionId() == "":
		return nil, trace.BadParameter("action_id is required")

	case req.GetUser() == "":
		return nil, trace.BadParameter("user is required")
	}

	slog.Debug("Creating MFA challenge for action", slog.String("action_id", req.GetActionId()))

	return nil, trace.NotImplemented("not implemented")
}

// ValidateChallengeForAction validates the MFA challenge response provided by the user for a specific user action.
// The action_id is required and must match the action the challenge was created for.
func (s *Service) ValidateChallengeForAction(
	ctx context.Context,
	req *mfav1.ValidateChallengeForActionRequest,
) (*mfav1.ValidateChallengeForActionResponse, error) {
	switch {
	case req.GetActionId() == "":
		return nil, trace.BadParameter("action_id is required")

	case req.GetMfaResponse() == nil:
		return nil, trace.BadParameter("response is required")

	case req.GetUser() == "":
		return nil, trace.BadParameter("user is required")
	}

	slog.Debug("Validating MFA challenge for action", slog.String("action_id", req.GetActionId()))

	return nil, trace.NotImplemented("not implemented")
}
