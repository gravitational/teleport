package oracle

import (
	"context"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// getURIs is a simple helper that returns the endpoint to dial.
// It exists to intentionally couple the engine dialing logic with the endpoint
// resolver logic.
func getURIs(db types.Database) []string {
	return strings.Split(db.GetURI(), ",")
}

// NewHealthChecker returns an endpoint health checker.
func NewHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	uris := getURIs(cfg.Database)
	return healthcheck.NewTargetDialer(func(context.Context) ([]string, error) {
		if len(uris) == 0 {
			return nil, trace.BadParameter("no URIs found")
		}
		return uris, nil
	}), nil
}
