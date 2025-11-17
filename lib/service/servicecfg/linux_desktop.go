package servicecfg

import (
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

// LinuxDesktopConfig specifies the configuration for the Linux Desktop
// Access service.
type LinuxDesktopConfig struct {
	Enabled bool
	// ListenAddr is the address to listed on for incoming desktop connections.
	ListenAddr utils.NetAddr
	// PublicAddrs is a list of advertised public addresses of the service.
	PublicAddrs []utils.NetAddr

	// ConnLimiter limits the connection and request rates.
	ConnLimiter limiter.Config

	Labels map[string]string
}
