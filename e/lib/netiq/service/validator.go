package service

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// validateConfig validates the configuration for the NetIQ service.
func validateConfig(c Config) error {
	if c.ClientConfig.OAuthClientID == "" {
		return trace.BadParameter("OAuthClientID is required")
	}
	if c.ClientConfig.OAuthClientSecret == "" {
		return trace.BadParameter("OAuthClientSecret is required")
	}
	if c.ClientConfig.OSPURL == "" {
		return trace.BadParameter("OSPURL is required")
	}
	if c.ClientConfig.APIURL == "" {
		return trace.BadParameter("APIURL is required")
	}
	if c.ClientConfig.IdentityVaultUser == "" {
		return trace.BadParameter("IdentityVaultUser is required")
	}
	if c.ClientConfig.IdentityVaultPassword == "" {
		return trace.BadParameter("IdentityVaultPassword is required")
	}
	if c.Clock == nil {
		return trace.BadParameter("Clock is required")
	}

	if c.Logger == nil {
		return trace.BadParameter("Logger is required")
	}
	if c.HostID == "" {
		return trace.BadParameter("HostID is required")
	}
	if c.SemaphoreSvc == nil {
		return trace.BadParameter("SemaphoreSvc is required")
	}

	if (c.AccessGraphConfig == servicecfg.AccessGraphConfig{}) {
		return trace.BadParameter("AccessGraphConfig is required")
	}

	if c.GetCreds == nil {
		return trace.BadParameter("GetCreds is required")
	}

	if c.ClusterFeatures == nil {
		return trace.BadParameter("ClusterFeatures is required")
	}

	return nil
}
