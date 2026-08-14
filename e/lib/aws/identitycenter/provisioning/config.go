package provisioning

import (
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
)

const (
	defaultProvisioningTimeout   = 2 * time.Minute
	defaultMaxConcurrentRequests = 7
)

// ProvisionerConfig provisions the principal assignment in AWS Identity Center.
type ProvisionerConfig struct {
	// Assignment is the principal assignment service.
	Assignment services.IdentityCenterPrincipalAssignments
	// Log is the logger.
	Log *slog.Logger
	// ProvisioningTimeout is the provisioning timeout.
	ProvisioningTimeout time.Duration
	// SDKClient is the AWS Identity Center client.
	SDKClient icsdk.Client
	// MaxConcurrentRequests is the maximum number of concurrent requests.
	MaxConcurrentRequests int
}

func (cfg *ProvisionerConfig) CheckAndSetDefaults() error {
	if cfg.Assignment == nil {
		return trace.BadParameter("missing assignment service")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default().With(teleport.ComponentKey, "AWS-IC-AssignmentProvisioner")
	}
	if cfg.ProvisioningTimeout == 0 {
		cfg.ProvisioningTimeout = defaultProvisioningTimeout
	}
	if cfg.MaxConcurrentRequests == 0 {
		cfg.MaxConcurrentRequests = defaultMaxConcurrentRequests
	}
	return nil
}
