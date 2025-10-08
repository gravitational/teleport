package plugin

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func OktaParseTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return 0, nil
	}
	raw := syncSettings.TimeBetweenImports
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_imports is not valid: %s", err)
	}
	if parsed < 0 {
		return 0, trace.BadParameter("time_between_imports %q cannot be a negative value", raw)
	}
	return parsed, nil
}
