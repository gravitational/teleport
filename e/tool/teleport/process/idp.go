package process

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// initSAMLIdP initializes the SAML IdP.
//
//nolint:revive // Because we want this to be IdP.
func initSAMLIdP(ctx context.Context, cfg *servicecfg.Config, plugin *web.Plugin) error {
	log := cfg.Logger.With(teleport.ComponentKey, eteleport.ComponentSAMLIdP)
	authClient := plugin.GetProxyClient()
	accessPoint := plugin.GetAccessPoint()

	// Create the authorizer.
	clusterName, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: eteleport.ComponentSAMLIdP,
			Logger:    log,
			Client:    authClient,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName.GetClusterName(),
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Get the public web address.
	publicAddr, err := cfg.Proxy.WebPublicAddr()
	if err != nil {
		return trace.Wrap(err)
	}

	// Initialize the SAML IdP.
	samlIdP, err := saml.New(ctx, saml.Config{
		Logger:      log,
		Clock:       cfg.Clock,
		Client:      authClient,
		AccessPoint: accessPoint,
		Authorizer:  authorizer,
		BaseURL:     publicAddr,
		Emitter:     authClient,
		HighLimiter: plugin.GetHighLimiter(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	plugin.RegisterSAMLIdP(samlIdP)

	cfg.Logger.InfoContext(ctx, "SAML identity provider has started successfully.")

	return nil
}
