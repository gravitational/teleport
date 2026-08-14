package leader

import (
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
)

const (
	semaphoreExpiration        = 10 * time.Minute
	semaphoreRenewal           = 2 * time.Minute
	semaphoreRenewalMaxRetries = 5
)

// Config is the configuration for the Okta leader service.
type Config struct {
	// SemaphoreName is the name of the semaphore to acquire.
	SemaphoreName string
	// SemaphoreKind is the kind of semaphore to acquire.
	SemaphoreKind string
	// HostIDHolder is the holder of the semaphore.
	HostIDHolder string
	// Clock is the clock used by the leader service.
	Clock clockwork.Clock
	// Semaphores is the semaphore service used by the leader service.
	Semaphores types.Semaphores
}

// CheckAndSetDefaults will check the configuration and set defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.SemaphoreName == "" {
		return trace.BadParameter("missing SemaphoreName")
	}
	if c.HostIDHolder == "" {
		return trace.BadParameter("missing HostIDHolder")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.SemaphoreKind == "" {
		return trace.BadParameter("missing SemaphoreKind")
	}
	if c.Semaphores == nil {
		return trace.BadParameter("missing Semaphores")
	}
	return nil

}
