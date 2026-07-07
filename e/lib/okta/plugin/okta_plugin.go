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

	// DefaultTargetProcessingBackoffStep is the step value for linear backoff when processing Okta assignment targets.
	// 15m chosen as starting point, since we want initial retry to be fairly soon after failure to retry
	// transient failures, but also for backoff to grow fairly quickly to avoid repeatedly reprocessing
	// persistent failures, which we know are quite common.
	DefaultTargetProcessingBackoffStep time.Duration = 15 * time.Minute
	// DefaultTargetProcessingBackoffMax is the max duration for linear backoff when processing Okta assignment targets.
	// 12hrs chosen as a starting point, since we have customer environments with persistent failures that we don't
	// want to be constantly retrying.
	DefaultTargetProcessingBackoffMax time.Duration = 12 * time.Hour
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

// GetTargetProcessingBackoffStep returns the backoff step value used for reprocessing failed Okta assignment targets.
func GetTargetProcessingBackoffStep(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return DefaultTargetProcessingBackoffStep, nil
	}
	raw := syncSettings.TargetProcessingBackoffStep
	if raw == "" {
		return DefaultTargetProcessingBackoffStep, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("target_processing_backoff_step is not valid: %s", err)
	}
	if d < 0 {
		return 0, trace.BadParameter("target_processing_backoff_step %q cannot be a negative value", raw)
	}
	if d == 0 {
		return DefaultTargetProcessingBackoffStep, nil
	}
	return d, nil
}

// GetTargetProcessingBackoffMax returns the max backoff value used for reprocessing failed Okta assignment targets.
func GetTargetProcessingBackoffMax(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return DefaultTargetProcessingBackoffMax, nil
	}
	raw := syncSettings.TargetProcessingBackoffMax
	if raw == "" {
		return DefaultTargetProcessingBackoffMax, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("target_processing_backoff_max is not valid: %s", err)
	}
	if d < 0 {
		return 0, trace.BadParameter("target_processing_backoff_max %q cannot be a negative value", raw)
	}
	if d == 0 {
		return DefaultTargetProcessingBackoffMax, nil
	}
	return d, nil
}
