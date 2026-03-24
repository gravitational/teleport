package oktaplugin

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	oktaplugin "github.com/gravitational/teleport/lib/okta/plugin"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// DefaultTimeBetweenImports is the default amount of time between Okta to Teleport
	// periodic syncs. This setting can be configured with the
	// okta.sync_settings.time_between_imports Okta plugin value.
	DefaultTimeBetweenImports = 30 * time.Minute

	// DefaultTimeBetweenAssignmentProcessLoops is the default amount of time that has to pass
	// between running the assignments process loop. It also determines how often the cached
	// Okta assignments client is invalidated. This setting can be configured with the
	// okta.sync_settings.time_between_assignment_process_loops Okta plugin value.
	// TODO(kopiczko) Increase to 10m.
	DefaultTimeBetweenAssignmentProcessLoops = 5 * time.Minute
)

// Get fetches the Okta plugin if it exists and does proper type assertions.
func Get(ctx context.Context, plugins services.Plugins, withSecrets bool) (*types.PluginV1, error) {
	return oktaplugin.Get(ctx, plugins, withSecrets)
}

// GetTimeBetweenImports parses syncSettings.TimeBetweenImports to duration. If it is not set
// it returns a default value of 30m.
func GetTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return DefaultTimeBetweenImports, nil
	}
	raw := syncSettings.TimeBetweenImports
	if raw == "" {
		return DefaultTimeBetweenImports, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_imports is not valid: %s", err)
	}
	if d < 0 {
		return 0, trace.BadParameter("time_between_imports %q cannot be a negative value", raw)
	}
	if d == 0 {
		return DefaultTimeBetweenImports, nil
	}
	return d, nil
}

// GetTimeBetweenAssignmentProcessLoops parses syncSettings.TimeBetweenAssignmentProcessLoops
// to duration. If it is not set it returns a default value of 10m.
func GetTimeBetweenAssignmentProcessLoops(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return DefaultTimeBetweenAssignmentProcessLoops, nil
	}
	raw := syncSettings.TimeBetweenAssignmentProcessLoops
	if raw == "" {
		return DefaultTimeBetweenAssignmentProcessLoops, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_assignment_process_loops is not valid: %s", err)
	}
	if d < 0 {
		return 0, trace.BadParameter("time_between_assignment_process_loops %q cannot be a negative value", raw)
	}
	if d == 0 {
		return DefaultTimeBetweenAssignmentProcessLoops, nil
	}
	return d, nil
}
