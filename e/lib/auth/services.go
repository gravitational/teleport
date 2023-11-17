package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/okta"
)

type CleanupFuncs []func() error

// StartServices will start additional services on the auth server that are
// not gRPC related. It will return a cleanup function that will clean up
// any of the services started here. Even if the function errors, cleanup should
// still be run.
func StartServices(ctx context.Context, plugin *Plugin) (func(), error) {
	cleanupFuncs := make(CleanupFuncs, 0)

	cleanup := func() {
		for _, cleanupFunc := range cleanupFuncs {
			// Skip nil functions.
			if cleanupFunc == nil {
				continue
			}

			if err := cleanupFunc(); err != nil {
				plugin.authServer.Logger.Errorf("Error creating Okta access request reconciler: %v", err)
			}
		}
	}

	var err error
	cleanupFuncs, err = startOktaReconciler(ctx, plugin, cleanupFuncs)
	if err != nil {
		return cleanup, trace.Wrap(err)
	}

	return cleanup, nil
}

func startOktaReconciler(ctx context.Context, plugin *Plugin, cleanupFuncs CleanupFuncs) (CleanupFuncs, error) {
	clusterName, err := plugin.authServer.AuthServer.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaAccessRequestReconciler, err := okta.NewAccessRequestReconciler(ctx, &okta.AccessRequestReconcilerConfig{
		AccessPoint: plugin.authServer.AuthServer,
		OktaClient:  plugin.authServer.AuthServer.OktaClient(),
		ClusterName: clusterName.GetClusterName(),
		Plugins:     plugin.plugins,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error creating Okta access requests reconciler")
	}

	cleanupFuncs = append(cleanupFuncs, func() error {
		oktaAccessRequestReconciler.Stop()
		return nil
	})

	if err := oktaAccessRequestReconciler.Start(ctx); err != nil {
		return cleanupFuncs, trace.Wrap(err, "error starting Okta access requests reconciler")
	}

	return cleanupFuncs, nil
}
