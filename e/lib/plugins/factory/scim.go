package factory

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

func SCIM(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	status := &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING}
	if err := deps.StatusSink.Emit(ctx, status); err != nil {
		deps.Logger.WarnContext(ctx, "Failed to emit plugin status", "error", err)
	}
	return func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, nil
}
