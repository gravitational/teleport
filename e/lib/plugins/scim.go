package plugins

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

func newSCIMInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	status := &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING}
	if err := deps.statusSink.Emit(ctx, status); err != nil {
		deps.logger.WarnContext(ctx, "Failed to emit plugin status", "error", err)
	}
	return func() error {
		<-deps.lifetime.Done()
		return nil
	}, nil
}
